# ICGT-005 lesson: Go service lifecycle

- **Unit:** ICGT-005
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; the root Go module, offline gate, FastGate health behavior, and
  bounded lifecycle are implemented and validated
- **Story:** [ICGT-005](../../user-stories/icgt-005-bootstrap-fastgate-service.md)
- **Review priority:** High
- **Visual companion:** [FastGate service lifecycle](assets/icgt-005-go-service-lifecycle.pptx)
- **Related architecture:** [ADR 0004](../adr/0004-go-toolchain-and-module-strategy.md),
  [Local setup](../setup.md), [FastGate README](../../gateway/README.md), and
  [FastGate agent guidelines](../../gateway/AGENTS.md)

## Quick summary

ICGT-005 turns the reviewed Go decision into a real, checked module and the smallest useful
FastGate process. The process validates bounded HTTP settings, opens one listener, returns `ok` from
`GET /healthz`, and owns shutdown through the final server join. It has no inference request,
provider, tenant, streaming, or routing behavior.

The central lesson is ownership: code that starts concurrent work must also define how that work
stops and how the owner learns that it has stopped.

## Learning objectives

After this lesson, you should be able to:

- explain why `main` is the composition root while `Service` owns the HTTP lifecycle;
- trace cancellation from an operating-system signal to graceful shutdown and final join;
- distinguish operational health from provider readiness;
- explain why every server timeout and header bound is explicit;
- describe how channel-controlled tests avoid fixed ports and timing sleeps; and
- show how the offline gate prevents module, toolchain, source-inventory, or dependency drift.

## Junior engineer foundation

A process receives signals such as Ctrl+C (`os.Interrupt`) or `SIGTERM`. `signal.NotifyContext`
turns those signals into a normal Go `context.Context` cancellation. The rest of the program does
not need to know which signal occurred; it observes `<-ctx.Done()`.

Graceful shutdown has three distinct jobs:

1. stop accepting new connections;
2. allow active handlers a bounded opportunity to finish; and
3. wait until the serving goroutine has actually returned.

Returning from `main` is not itself graceful cleanup. It can terminate goroutines and active
requests immediately. Conversely, calling `Shutdown` without joining `Serve` leaves the owner
without proof that serving work ended.

## Architecture and invariants

```text
signal context
    -> validate config
    -> open listener
    -> start http.Server.Serve in one owned goroutine
        -> GET /healthz returns bounded local health
    -> cancellation or Serve completion
        -> graceful Shutdown with five-second deadline in either case
        -> if drain fails: Close active connections
        -> receive the Serve result if it was not already received (join)
        -> return nil or a safe wrapped error
```

The important invariants are:

- the exact root `go.mod` is the only module boundary;
- construction validates configuration before `net.Listen` or serving;
- the health response means only “this local process is serving”;
- one owner starts, stops, and joins the HTTP server;
- shutdown has a deadline and failure forces closure before the owner returns; and
- no code in this unit contacts a model provider or starts an unowned goroutine.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [`go.mod`](../../go.mod) and [`scripts/check`](../../scripts/check) | Turn ADR 0004 into an enforced module and offline build boundary | Can a workspace, nested module, mismatched tool, or unformatted source escape the gate? |
| [`gateway/cmd/fastgate/main.go`](../../gateway/cmd/fastgate/main.go) | Converts signals and flags into validated construction and one listener | What is created here, and what lifecycle details remain delegated? |
| [`gateway/internal/service/service.go`](../../gateway/internal/service/service.go) | Owns bounded server configuration, health, shutdown, forced close, and join | Which receive operation proves that the serving goroutine ended? |
| [`gateway/internal/service/service_test.go`](../../gateway/internal/service/service_test.go) | Proves real loopback health, early-failure cleanup, and deterministic shutdown failure | How do the fakes prove cleanup, deadline, close, and join without sleeping? |

Personally read the two production files and the three lifecycle tests. The surrounding policy tests
are useful, but those paths contain the core learning value.

## Implementation walkthrough

### 1. The composition root validates before serving

[`run`](../../gateway/cmd/fastgate/main.go) parses one bounded input, asks the service constructor to
validate every setting, opens the listener only after validation succeeds, and then transfers
lifecycle control to `Serve`:

```go
func run(ctx context.Context, arguments []string) error {
	config := service.DefaultConfig()
	flags := flag.NewFlagSet("fastgate", flag.ContinueOnError)
	flags.StringVar(
		&config.ListenAddress,
		"listen",
		config.ListenAddress,
		"TCP address for the operational health server",
	)
	if err := flags.Parse(arguments); err != nil {
		return fmt.Errorf("parse startup configuration: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("parse startup configuration: unexpected positional arguments")
	}

	application, err := service.New(config)
	if err != nil {
		return fmt.Errorf("validate startup configuration: %w", err)
	}

	listener, err := net.Listen("tcp", config.ListenAddress)
	if err != nil {
		return fmt.Errorf("listen for FastGate health traffic: %w", err)
	}

	return application.Serve(ctx, listener)
}
```

The default address is loopback-only. The constructor also requires positive read-header, read,
write, idle, and shutdown timeouts; it caps configured header bytes at one MiB and uses a smaller
16 KiB default. The first process therefore cannot silently inherit `http.Server` zero values for
important network bounds.

### 2. Health is deliberately smaller than readiness

The handler uses Go's method-aware `ServeMux` pattern:

```go
func newHealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+healthPath, func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte(healthBody))
	})
	return mux
}
```

`GET /healthz` returns exactly `ok\n`. Go also provides normal `HEAD` behavior for a registered GET
pattern. A `POST` receives `405 Method Not Allowed`; an unknown path receives `404 Not Found`.
There is no provider probe, model name, tenant check, remote call, or unbounded response.

### 3. A terminal event still requires cleanup and a join

The service starts exactly one serving goroutine and gives it a buffered result channel. It then
waits for either an early server result or owner cancellation:

```go
serveResult := make(chan error, 1)
go func() {
	serveResult <- service.server.Serve(listener)
}()

var serveErr error
serveFinished := false
select {
case serveErr = <-serveResult:
	serveFinished = true
case <-ctx.Done():
}

shutdownErr := service.stop(ctx)
if !serveFinished {
	serveErr = <-serveResult
}
return errors.Join(normalizeServeError(serveErr), shutdownErr)
```

Cancellation is not the only reason serving can end. A listener or server failure can return while
active handlers still exist, so both select branches call the same cleanup helper. `serveFinished`
is separate from `serveErr` because a server is allowed to return `nil`; a `nil` error cannot tell us
whether the channel was already received. Receiving the result is the visible goroutine join.

The cleanup helper creates a fresh bounded context and forces closure only when graceful shutdown
fails:

```go
func (service *Service) stop(ctx context.Context) error {
	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.WithoutCancel(ctx),
		service.shutdownTimeout,
	)
	defer cancelShutdown()

	shutdownErr := service.server.Shutdown(shutdownCtx)
	if shutdownErr == nil {
		return nil
	}

	closeErr := service.server.Close()
	return errors.Join(
		fmt.Errorf("graceful FastGate shutdown: %w", shutdownErr),
		wrapError("force-close FastGate server", closeErr),
	)
}
```

When cancellation triggered cleanup, `context.WithoutCancel(ctx)` matters because the owner context
is already canceled. Reusing it directly would make graceful shutdown fail immediately. On an early
serving failure, the same helper still supplies a fresh bounded cleanup window. `WithTimeout`
preserves context values in both cases.

If graceful shutdown fails, `Close` stops active HTTP work before the join. `errors.Join` preserves
the shutdown cause and any close or serve failure without logging request bodies or provider data.

### 4. The failure tests control events, not wall-clock timing

The early-failure test releases the controlled server immediately, then proves the original error is
preserved and `Shutdown` still received a deadline:

```go
serveFailure := errors.New("accept failed")
fake := newControlledServer(serveFailure)
shutdownObserved := make(chan bool, 1)
fake.shutdown = func(ctx context.Context) error {
	_, bounded := ctx.Deadline()
	shutdownObserved <- bounded
	return nil
}
fake.close = func() error { return nil }
fake.release()
application := &Service{server: fake, shutdownTimeout: time.Second}

err := application.Serve(context.Background(), inertListener{})

if !errors.Is(err, serveFailure) {
	t.Fatalf("Serve() error = %v, want original serve failure", err)
}
```

The forced-close test scripts `Shutdown` to observe a deadline and return
`context.DeadlineExceeded`. `Close` releases the blocked fake server:

```go
fake.shutdown = func(ctx context.Context) error {
	_, bounded := ctx.Deadline()
	shutdownObserved <- bounded
	return context.DeadlineExceeded
}
fake.close = func() error {
	fake.release()
	return nil
}

cancelServe()
err := receiveResult(t, serveResult)

if !errors.Is(err, context.DeadlineExceeded) {
	t.Fatalf("Serve() error = %v, want context deadline exceeded", err)
}
```

There is no sleep that hopes the server reaches the right state. The test waits on `fake.started`,
cancels the owner, and lets explicit channels control when serving can return. Because `Serve`
cannot return until `Close` releases the fake, receiving the final result also proves the join.

## Test strategy and evidence

The focused test suite proves four reviewable behaviors:

- table-driven handler tests prove success headers/body, method rejection, and path rejection
  without sockets;
- an injected ephemeral loopback listener proves a real HTTP request and graceful cancellation; and
- one controlled-server test proves early serving failure still triggers bounded cleanup while
  preserving the original cause; and
- another controlled-server test proves the forced-close failure path deterministically.

The real-listener test disables ambient proxy use and bounds its response read. Its two-second
contexts are deadlock guards, not synchronization sleeps. The race detector checks the same tests.

The repository gate separately proves:

- exact `go.mod` bytes and no `go.work`, nested `go.mod`, `go.sum`, or `go.work.sum`;
- `go` and `gofmt` come from the same local distribution;
- Go resolves the repository-root module with toolchain and module downloads disabled;
- exported secrets are omitted from child environments, `HOME` and Go configuration are temporary,
  and telemetry is disabled; this does not sandbox arbitrary filesystem or network access;
- every tracked or nonignored `.go` file is discovered with NUL-safe Git output;
- formatting, vet, ordinary tests, race prerequisites, and race tests pass; and
- CI uses a SHA-pinned `setup-go`, reads the version from `go.mod`, and verifies `GOVERSION`.

## What changed during implementation

- Go was absent at the start. The official Go 1.26.5 Linux archive was downloaded, matched against
  its published SHA-256, installed at `/usr/local/go`, and verified with the race preflight.
- The shared `/tmp` tmpfs was full even though the WSL root filesystem had ample capacity. Build
  temporary and cache paths were redirected into task-owned `.git` paths for local validation. The
  repository gate itself did not hide or delete unrelated temporary data.
- The first sandboxed loopback test failed with `socket: operation not permitted`. The identical
  focused and full tests passed when granted loopback access; no external network was used.
- The previous conditional Go hook was replaced. Once `go.mod` exists, missing or incompatible Go,
  `gofmt`, cgo, C compiler, manifest, or race support is now a hard failure.
- A real-time “sleep until shutdown expires” test was unnecessary. The controlled server returns a
  scripted deadline failure after verifying that production supplied a bounded context.
- Independent review found that an unexpected `Serve` failure returned before lifecycle cleanup and
  that the gate scrubbed only selected provider variables. The final design funnels both terminal
  events through cleanup, re-executes with an allowlisted environment, isolates Go configuration,
  disables telemetry, and rejects duplicate, mutable, conditional, or shell-text-only `setup-go`
  representations. The same structural check requires the policy, version-verification, and final
  gate commands to be unconditional executable steps in the same job's `steps` block rather than
  comments, echoed text, or unrelated-job commands.
- The first allowlist implementation exported its internal re-execution marker. A policy test run by
  the outer gate inherited that marker and correctly exposed that a nested gate could skip
  sanitization. The final script unsets the marker immediately after re-execution, so every later
  invocation performs its own environment scrub.

## Validation

The following passed with Go 1.26.5 on Linux/amd64:

```bash
go test ./gateway/internal/service
python3 -m unittest discover -s tests -p 'test_*.py'
./scripts/check
git diff --check
```

The canonical gate ran `go vet ./...`, `go test ./...`, the race prerequisite compile, and
`go test -race ./...` under `GOTOOLCHAIN=local`, `GOPROXY=off`, `GONOPROXY=none`, `GOSUMDB=off`,
`GOWORK=off`, `GOFLAGS=-mod=readonly`, `GOENV=off`, and an isolated temporary Go telemetry setting
of `off`.

## Production expansion

| Local implementation | Possible production expansion | Cost | Graduation signal |
| --- | --- | --- | --- |
| One operational `/healthz` route | Separate liveness and dependency-aware readiness | More states and orchestration policy to test | A deployer must distinguish “restart me” from “temporarily remove me from traffic” |
| Fixed reviewed timeouts | Deployment-specific bounded configuration | More validation and operational combinations | Measured workloads show the defaults are incorrect |
| Standard-library error return | Structured, low-cardinality logs and traces | Logging policy, correlation, and redaction maintenance | Operators cannot diagnose lifecycle failures from process status alone |
| Signal-driven single process | Service manager or Kubernetes termination integration | Deployment manifests and drain coordination | A deployment story owns rollout and termination behavior |
| Plain loopback HTTP | TLS and authenticated operational access | Certificates, rotation, authorization, and failure handling | The route must leave a trusted local boundary |

None of those capabilities is required merely to make this learning scaffold look more
“production-like.” Each should enter with a story that can test its new failure modes.

## Practical exercises

- Draw the owner of the signal context, listener, HTTP server, shutdown context, and serving
  goroutine.
- Change a copied test case from `GET` to `POST` and predict the status and response before running
  it.
- Temporarily add a `toolchain` directive to a copied `go.mod` and explain which gate stage should
  reject it before CI provisioning.
- Write down why receiving from `serveResult` is stronger evidence than merely calling `Shutdown`.

## Key takeaways

- Process lifecycle is a real API: startup, cancellation, cleanup, and terminal errors need explicit
  semantics.
- Configuration validation precedes listener creation, and every important server bound is visible.
- Health reports only evidence the process owns; it does not imply provider readiness.
- A graceful shutdown call is incomplete until the serving goroutine is joined.
- Deterministic tests synchronize on events and channels rather than guessing with sleeps.
- The offline gate turns the Go module decision into enforceable repository behavior.

## Glossary

- **Composition root:** the program location that selects concrete configuration and dependencies.
- **Context:** a Go value that carries cancellation, deadlines, and request-scoped values.
- **Graceful shutdown:** stopping admission while allowing active work a bounded chance to finish.
- **Join:** waiting for started concurrent work to return before its owner completes.
- **Listener:** the resource that accepts new network connections.
- **Race detector:** Go instrumentation that reports conflicting unsynchronized memory access.
- **Readiness:** whether a process should receive traffic; it may depend on more than liveness.

## Teach-back questions

1. Trace ownership from `main` creating the signal context through `Service.Serve` receiving the final server result.
2. How does `TestServiceShutdownFailureForcesCloseAndJoins` prove a bounded cleanup failure without using a synchronization sleep?
3. What concrete operational need would justify splitting `/healthz` into separate readiness and liveness endpoints?

## Further reading

- [ICGT-005 delivery contract](../../user-stories/icgt-005-bootstrap-fastgate-service.md)
- [ICGT-005 implementation note](../../user-stories/notes/2026-08-02-icgt-005-fastgate-lifecycle.md)
- [Go `net/http` package](https://pkg.go.dev/net/http)
- [Go `context` package](https://pkg.go.dev/context)
- [Go race detector](https://go.dev/doc/articles/race_detector)
- [Go toolchain selection](https://go.dev/doc/toolchain)
