# ICGT-011 - Bind the safe local demo runtime

- **Status:** Planned
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-010
- **Lesson:** [Safe local runtime composition](../docs/lessons/icgt-011-safe-local-runtime.md)
- **Review priority:** High

## User story

> As a gateway learner, I want the implemented model-turn handler mounted behind one deterministic,
> loopback-only, concurrency-bounded runtime so that I can exercise the complete non-streaming
> FastGate path locally without turning the strict test fake into a server dependency or implying that
> the endpoint is authenticated, remotely deployable, or backed by a live model.

## Primary concept and invariant

This story owns the final M1 runtime-composition boundary:

> FastGate starts inference only after it has selected a stateless runtime invoker and verified the
> actual listener as a concrete loopback `*net.TCPListener`. A transport-valid request either
> acquires one bounded permit before its body is read, holds it through response writing, and runs
> once; or it acquires nothing and receives an immediate fixed overload response. Health and cheap
> transport rejection remain available without an inference permit, and every acquired permit is
> returned even when the HTTP handler aborts.

The provider, listener, route, and permit choices form one observable safe local endpoint. Splitting
them would either add unused production code or make the route reachable before all required safety
conditions exist. [ADR 0005](../docs/adr/0005-local-demo-runtime-profile.md) records the accepted
alternatives and exact first profile.

This remains one small story only while it introduces at most two new production abstractions—the
stateless demo provider and the private permit gate—and stays near the repository's roughly 400-line
production review heuristic. The implementation must stop for split review if it needs a provider
selector, queue, worker pool, generic routing framework, Host-policy framework, or any deferred
operational feature.

## Existing dependency evidence

- ICGT-008's [`fake.Provider`](../gateway/internal/provider/fake/fake.go) is a finite, single-owner
  assertion oracle and explicitly forbids concurrent calls.
- ICGT-009's [`modelturn.Executor`](../gateway/internal/modelturn/executor.go) admits at most 8 MiB,
  maps one provider-neutral request, and invokes its injected provider at most once.
- ICGT-010's [`modelturnhttp` handler](../gateway/internal/modelturnhttp/handler.go) performs exact
  target, method, encoding, and media preflight before the first body read and presents every closed
  non-streaming outcome.
- ICGT-005's [`service.Service`](../gateway/internal/service/service.go) already owns bounded
  graceful-first shutdown, force-close, and server-result joining.

No dependency currently mounts inference in the command, selects an unscripted runtime provider,
checks the actual listener, or bounds concurrent admitted model turns.

## Scope

- Add one stateless fixed-output provider under `gateway/internal/provider/demo`.
- Make the model-turn HTTP handler own a validated nonblocking permit gate.
- Acquire permits only after ICGT-010 transport preflight and before body admission.
- Preserve ICGT-010 transport responses without consuming inference capacity.
- Add a small explicit service dispatcher for health and the injected inference handler.
- Verify the actual listener is a concrete loopback `*net.TCPListener` before starting
  `http.Server.Serve`.
- Assemble demo provider, executor, bounded handler, service, and listener explicitly in `main`.
- Add `-max-concurrent-model-turns` with default 4 and accepted range 1 through 16.
- Prove the complete default profile with deterministic direct and real-loopback tests.
- Keep the implementation standard-library-only, offline, credential-free, and independent of Code
  Assist Harness.

## Locked behavior

### Stateless runtime demo

- Add planned package path `gateway/internal/provider/demo` with a small `Provider` implementing
  `provider.Invoker`; `demo.New() (*Provider, error)` constructs its immutable result.
- Its constructor creates one valid immutable `provider.Result` whose output is exactly
  `FastGate local demo response.` and whose usage is absent.
- Every non-terminated invocation returns that result. A context already canceled or expired before
  the call returns the exact matching context sentinel allowed by the provider port; this is basic
  port compliance, not cancellation-conformance evidence.
- The provider has no mutable request state and is safe for concurrent calls. It does not inspect,
  retain, copy, log, or echo conversation or instruction content.
- It has no script, model ID, credential, endpoint, network client, goroutine, timer, retry, usage
  estimator, or provider SDK.
- The runnable command uses this provider by default. It never mounts the strict deterministic fake,
  and no provider-selection flag is introduced.

### Post-preflight concurrency gate

- Change the constructor to
  `modelturnhttp.NewHandler(executor *modelturn.Executor, maxConcurrentModelTurns int)` so every
  constructed handler has a maximum number of concurrent model turns. There is no unbounded runtime
  constructor.
- Reject a limit below 1 or above 16 before allocating capacity. The command default is 4.
- Preserve preflight order: target, method, encoding, and media type are checked before a permit is
  attempted. Their existing `404`, `405`, and `415` results therefore remain deterministic even while
  all model-turn permits are occupied.
- After successful preflight, attempt one nonblocking permit acquisition before the first body read.
  Do not wait, enqueue, start a goroutine, schedule a retry, or create a timer.
- Hold the permit across complete body admission, executor/provider work, JSON preparation, status and
  header commitment, and the body write.
- Release it with `defer`. Do not recover panics; an exact `http.ErrAbortHandler` must unwind through
  the gate, release the permit, and reach `net/http` unchanged.
- The gate bounds only transport-valid model-turn requests that acquire a permit, from before their
  first body read through their application response write. It does not claim to bound accepted TCP
  connections, header parsing, transport-rejected handlers, request goroutines already created by
  `net/http`, or a saturated request while that unpermitted handler writes its fixed `503`.

### Exact overload response

When a transport-valid request cannot acquire a permit, return exactly:

- status `503 Service Unavailable`;
- body `service unavailable\n`;
- `Content-Type: text/plain; charset=utf-8`;
- `Cache-Control: no-store`;
- `X-Content-Type-Options: nosniff`; and
- no `Retry-After` header.

The application makes zero body-read calls and zero provider calls for that request. The response is
an uncorrelated runtime-capacity result, not a normalized provider failure, retry command, rate-limit
policy, or proof that a later attempt is safe or free.

### Health and inference dispatch

- Change the service constructor to `service.New(config Config, inferenceHandler http.Handler)`; it
  accepts the already constructed inference handler and rejects nil.
- Replace the health-only top-level handler with a small explicit dispatcher. Requests belonging to
  the existing health path retain current health method/status/header behavior and bypass inference
  permits. The dispatcher selects health only when the request and `request.URL` are nonnil and its
  parsed `Path` is exactly `/healthz`, then passes the request unchanged to the existing health
  handler; every other request reaches the injected model-turn handler unchanged.
- The dispatcher inspects only that parsed `URL.Path`; it does not inspect or rewrite `RawPath`,
  `RawQuery`, `RequestURI`, query values, or `Host`. Consequently, the existing health handler still
  owns GET, HEAD, POST, headers, body, and query behavior for a parsed `/healthz` path. An encoded
  spelling that Go parses to `/healthz` follows that existing handler. A trailing slash, repeated
  slash, dot-segment alternative, or nil URL does not select health and reaches the model-turn
  handler unchanged, where ICGT-010 returns its direct fixed `404` without a service redirect. A nil
  request follows the same unchanged model-handler path.
- Pin the existing observable health matrix before changing composition: GET authors `200`, `ok\n`,
  and its two current headers; HEAD selects the same handler and actual `net/http` serving suppresses
  its wire body; POST remains the existing `405` with `Allow: GET, HEAD`; query and encoded spellings
  whose parsed path is `/healthz` retain those semantics. The story changes dispatch ownership, not
  health protocol behavior.
- Do not put a new `http.ServeMux` in front of the inference handler. `ServeMux` may clean repeated or
  dot-segment paths and issue redirects before ICGT-010 can apply its raw-target rules.
- Preserve exact model-turn target, query, encoded-path, absolute-form, method, media, trailer, HEAD,
  response-abort, and write behavior from ICGT-010.
- Health remains an operational signal only. It does not report provider readiness or free permit
  capacity.

### Actual-listener enforcement

- Configuration remains a requested listen address, not security evidence. The actual listener
  passed to `Service.Serve` is authoritative.
- Preserve the existing validation order. A nil context returns exactly
  `serve FastGate: context is required`; no listener close is attempted and the caller retains any
  supplied listener on that path.
- After a nonnil context is validated, treat a nil listener interface or typed-nil
  `*net.TCPListener` as a missing required input: return exactly
  `serve FastGate: listener is required` and make no close attempt because no usable resource exists.
- Only after those checks and before starting the server goroutine, require the listener itself to be
  a `*net.TCPListener` and its `Addr()` to be a `*net.TCPAddr` with an IPv4 or IPv6 loopback IP. An
  interface wrapper that merely reports a loopback-looking TCP address is rejected because its
  reported address does not prove the underlying listener type.
- Reject wildcard, unspecified, LAN/global, Unix, wrapped, and unknown nonnil listener or address
  types. Do not format the rejected address into logs or errors.
- Listener ownership transfers to `Serve` only after the nonnil context and nonnil listener checks.
  When later admission fails, close the rejected listener exactly once before returning exactly
  `serve FastGate: listener must be a loopback TCP listener`.
- If rejection cleanup also fails, retain that fact only as the fixed sentinel/category
  `close rejected FastGate listener`. Join those two fixed errors so the exact diagnostic is the base
  line followed by the cleanup line. Deliberately discard rather than wrap the arbitrary raw close
  error, because `main` may log the returned chain and listener implementations are outside this
  boundary's trust.
- A configured loopback string paired with an injected non-loopback actual listener must fail. Tests
  must not pass merely because `Config.ListenAddress` contains `127.0.0.1`.
- Keep the concrete-listener assertion visible in `Serve`. A small private address-classification
  helper may make LAN/global IP cases deterministic, but production must call it only after the
  dynamic listener has been proven to be `*net.TCPListener`; a synthetic `Addr()` value cannot bypass
  that assertion.
- No Host-header allowlist is added. `Host` is caller-controlled and is not treated as authentication
  or proof of the bound socket.

### Command composition and lifecycle

- `gateway/cmd/fastgate/main.go` remains the composition root. It constructs, in order, the local demo
  provider, executor, bounded HTTP handler, service, and TCP listener.
- Parse `-max-concurrent-model-turns` beside `-listen`. Invalid capacity fails before listener
  creation. Keep operation errors bounded and content-free.
- The default command mounts both health and model-turn handling once actual-listener validation
  succeeds. No opt-in demo flag is required.
- Keep ICGT-005 graceful-first shutdown. Cancellation stops server admission, bounded shutdown lets
  active handlers finish, failure forces closure, and `Serve` joins the server goroutine before
  returning.
- Do not add `Server.BaseContext` cancellation that would immediately terminate accepted work and
  defeat graceful draining.

### Security and user-facing meaning

- Loopback limits network reachability; it does not authenticate callers. Any local process able to
  reach the listener may call the demo, and Windows/WSL or container host integrations may expose
  loopback across the local-host boundary.
- The profile is not approved for wildcard, LAN, proxy, container-cluster, or public deployment.
- A caller-supplied non-loopback-looking `Host` does not change actual-listener admission and is not
  rejected by this story. A focused test makes that lack of Host authorization intentional.
- Browser-origin and DNS-rebinding risk remain unreviewed. Strict JSON and the absence of CORS
  permission are not authentication or durable security guarantees.
- Before any live or billable provider is mounted in any runnable profile—even on loopback and even
  alongside rather than instead of the demo—a separate review must select and test the Host, Origin,
  CORS, DNS-rebinding, and caller-authentication threat model. A future opt-in provider adapter may
  exist behind non-runnable test seams until that prerequisite is satisfied.
- Do not add CORS, authentication, TLS, trusted-proxy, or authorization-header behavior.
- Successful response shape and size remain the ICGT-010 contract plus the fixed short demo output.
  Permit acquisition is one local nonblocking operation, but this story makes no latency or throughput
  promise.

## Human-sized implementation checkpoints

1. **Demo provider:** add the immutable fixed result, direct context behavior, concurrent tests, and
   proof that request content is neither inspected nor echoed.
2. **Bounded model-turn handler:** validate 1 through 16 permits, preserve preflight precedence, prove
   exact no-read/no-dispatch overload, hold the permit through writes, and prove panic-safe reuse.
3. **Service boundary:** inject the inference handler, dispatch health without path cleaning, validate
   the actual listener, and prove rejected-listener cleanup before any server or provider work.
4. **Runnable composition:** add the CLI setting and explicit command wiring, then prove health,
   completed demo inference, unusual Host non-authorization, saturation, graceful shutdown, and
   response cleanup through a proxy-disabled real loopback client.

Each checkpoint passes its focused tests before the next begins. Stop for review rather than starting
M2 behavior inside one of these checkpoints.

## Acceptance criteria

1. The demo provider returns exactly `FastGate local demo response.` with usage absent for concurrent
   valid calls, echoes no request content, makes no external call, and passes race-enabled tests.
2. The command selects that demo by default and never constructs the strict scripted fake or a live
   provider.
3. Handler construction accepts limits 1 through 16, rejects zero, negative, and 17 before capacity
   allocation, and the CLI default is exactly 4.
4. Target, method, encoding, and media rejection retains ICGT-010's exact behavior while capacity is
   full and consumes no permit, body read, or provider call.
5. Exactly the configured number of transport-valid controlled requests may enter body/provider work.
   The next receives the exact fixed `503` immediately, reads no body, invokes no provider, waits in no
   queue, and receives no `Retry-After`.
6. A permit is unavailable while its request is blocked in response writing, then becomes reusable
   after normal return or write failure.
7. `http.ErrAbortHandler` propagates unchanged, writes no fabricated response, releases its permit,
   and allows a later request to enter.
8. Health succeeds and preserves its current parsed-path, method, query, header, and body behavior
   while all inference permits are held. The dispatch matrix covers GET, HEAD, and POST for parsed
   `/healthz`, a query, an encoded spelling parsed to `/healthz`, trailing/repeated slashes, a dot
   segment, a nil URL, and a nil request.
9. The explicit dispatcher inspects only parsed `URL.Path`, passes every request unchanged, and does
   not redirect, clean, or accept model-turn target spellings that the ICGT-010 handler rejects.
10. `Service.Serve` first rejects a nil context with its exact existing error, makes no close attempt,
    and leaves any supplied listener with the caller. With a nonnil context, a nil interface or
    typed-nil `*net.TCPListener` returns the exact required-listener error without a close. Actual IPv4
    and IPv6 loopback `*net.TCPListener` values are accepted; wildcard, unspecified, non-loopback,
    Unix, wrapped, and unknown nonnil listeners or addresses are rejected before server work begins.
11. Every nonnil rejected actual listener is closed exactly once. The base diagnostic is exactly
    `serve FastGate: listener must be a loopback TCP listener`; a cleanup failure adds only the fixed
    `close rejected FastGate listener` line and never exposes or wraps raw listener error text.
12. A real proxy-disabled loopback exchange crosses command-equivalent demo, executor, bounded
    handler, service, and listener composition and returns the exact completed model-turn document.
13. A non-loopback-looking Host sent over the accepted loopback connection does not become an
    authentication decision; documentation still warns that any reachable local caller is accepted.
14. Existing graceful shutdown, force-close, context-abort, strict-fake, executor, contract, and
    health tests remain green with no timing sleeps or leaked goroutines/listeners/bodies.
15. No external dependency, `go.sum`, credential, provider SDK, prompt logging, CAH change, or visual
    lesson is added. Focused checks and `./scripts/check` pass.

## Deterministic validation

Minimum focused evidence includes:

- demo construction, fixed result, absent usage, pre-terminated context, sentinel-content
  non-reflection, concurrent calls, and race execution;
- handler limits -1, 0, 1, 4, 16, and 17 without allocating invalid capacity;
- channel-controlled exact-capacity admission, immediate one-over rejection, zero body reads, zero
  extra invocations, and successful permit reuse without sleeps;
- target, method, encoding, and media rejections while capacity is held, proving preflight precedence;
- a blocking response writer proving the permit remains owned through writing;
- exact abort panic and ordinary writer-failure cases proving release without recovery or redispatch;
- health GET, direct-versus-wire HEAD, POST, query, and encoded-spelling behavior while inference is
  saturated, plus trailing-slash, repeated-slash, dot-segment, nil-URL, and nil-request fallthrough
  to the unchanged model handler;
- no service redirects or path cleaning for trailing-slash, repeated-slash, dot-segment, encoded,
  query, absolute-form, and authority-bearing model-turn targets;
- real IPv4 and available IPv6 loopback `*net.TCPListener` acceptance plus real wildcard rejection;
  a pure TCP-address classifier covers unspecified and LAN/global IPs without requiring an external
  interface; Unix, wrapped-loopback-reporting, and synthetic listeners are rejected at the
  concrete-listener boundary, while nil interface and typed-nil TCP cases separately prove the
  required-listener error and zero close attempts;
- nil-context precedence with both nil and nonnil listeners, proving the exact context error, zero
  close attempts, and caller-retained listener ownership;
- configured-loopback/actual-non-loopback mismatch, zero server starts, rejected-listener close, and
  exact fixed base/cleanup diagnostics whose error chain omits the raw close cause;
- `-max-concurrent-model-turns` default, exact endpoints, below/above rejection, positional-argument
  rejection, and construction before listener creation;
- one real proxy-disabled loopback completed exchange with bounded response read/close, an unusual
  Host value, health availability, service cancellation, graceful drain, and joined Serve result;
- focused repeated tests, `go vet ./...`, `go test -race ./...`, `git diff --check`, the applicable
  PR-review checklist, an independent concurrency/cleanup/security review, and `./scripts/check`.

## Human review checkpoint

- **Production path:** Trace the planned `gateway/internal/provider/demo/demo.go` fixed result into
  [`executor.go`](../gateway/internal/modelturn/executor.go), then review permit placement in
  [`handler.go`](../gateway/internal/modelturnhttp/handler.go), health/inference dispatch and actual
  listener admission in [`service.go`](../gateway/internal/service/service.go), and final composition
  in [`main.go`](../gateway/cmd/fastgate/main.go).
- **Failure/test path:** Review the planned demo tests and permit-focused handler tests, then trace
  [`service_test.go`](../gateway/internal/service/service_test.go) for rejected-listener closure and
  the planned command integration test for saturation, health bypass, abort release, and graceful
  shutdown.
- **Invariant:** The reviewer can explain why only transport-valid model turns consume one of at most
  the configured permits before reading a body, why the permit remains held through the write and is
  always returned, and why actual loopback reachability is not caller authentication.
- **Deferred:** Streaming/SSE, cancellation conformance and races, upstream deadlines and cleanup
  grace, slow-client backpressure, telemetry, queues/fairness, connection limits, retries, live
  providers, credentials, authentication, TLS, CORS, Host policy, trusted proxies, non-loopback
  deployment, Code Assist Harness integration, and visual lessons.

## Documentation impact

- Keep this story and its single Markdown lesson Planned until exact implementation evidence exists.
- Record the accepted profile and rejected alternatives in ADR 0005.
- Promote ICGT-011 to the reviewed M1 sequence while keeping every status document explicit that the
  command remains health-only until implementation.
- After implementation, update the root and FastGate setup with one exact local request/response
  example, add an implementation note, replace planned lesson paths/pseudocode with exact excerpts,
  and mark M1 complete.
- Add a concise PR-review checklist entry for the reusable permit-placement and response-precedence
  defect class found during readiness.
- Keep Code Assist Harness unchanged; a future harness-owned adapter begins only after the versioned
  ICGT-021/022 handoff.

## Out of scope

- Making the strict scripted fake concurrency-safe or using it for runtime traffic.
- Multiple demo modes, prompt echoing, generated text, usage estimation, model aliases beyond the
  existing admitted contract, provider selection, OpenAI, Groq, or any live/provider-SDK behavior.
- Queues, worker pools, fairness, rate limits, per-tenant policy, retries, `Retry-After`, readiness,
  telemetry, benchmarks, or autoscaling.
- SSE, partial output, cancellation acknowledgement or races, upstream deadlines, cleanup grace,
  reaping, and slow-client backpressure.
- Host authorization, Origin policy, caller authentication, TLS, CORS, DNS-rebinding mitigation,
  trusted proxies, redirects, wrapped/TLS listeners, wildcard or non-loopback listeners, and a
  remotely deployable security profile.
- Changing Code Assist Harness workflow state, tools, approvals, transcripts, adapters, retry policy,
  or correctness evaluation.
- Creating a visual lesson or other visual companion.
