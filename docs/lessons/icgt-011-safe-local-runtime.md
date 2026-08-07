# ICGT-011 lesson: Binding a safe local demo runtime

- **Unit:** ICGT-011
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Approved design; implementation has not started
- **Story:** [ICGT-011](../../user-stories/icgt-011-bind-local-demo-runtime.md)
- **Review priority:** High
- **Visual companion:** Not required; the Markdown lesson is the planned learning artifact
- **Related architecture:** [ADR 0005](../adr/0005-local-demo-runtime-profile.md) (governing runtime
  profile), [ADR 0002](../adr/0002-fake-first-openai-first-live.md),
  [ADR 0003](../adr/0003-fastgate-api-surface.md),
  [ICGT-005 lifecycle](../../user-stories/icgt-005-bootstrap-fastgate-service.md), and
  [ICGT-010 HTTP presentation](../../user-stories/icgt-010-present-model-turn-over-http.md)

> This lesson describes the accepted ICGT-011 design, not observed implementation. Existing links
> identify the boundaries already delivered by earlier stories. Every new path and code example is
> labeled **PLANNED** or **PSEUDOCODE ONLY** until implementation and validation replace it with
> exact source evidence.

## Quick summary

ICGT-010 built and tested a model-turn HTTP handler, but the runnable FastGate command still mounts
only the operational `/healthz` surface (`GET`, with implicit `HEAD`). ICGT-011 plans the final M1
assembly that makes the existing non-streaming model-turn path reachable as a deliberately local
demo.

The planned inbound request flow is:

```text
verified concrete loopback *net.TCPListener
    -> HTTP server
    -> explicit dispatcher
         |-> health handler (no inference permit)
         `-> model-turn transport preflight
                |-> fixed transport rejection (no inference permit)
                `-> nonblocking permit
                       -> body admission
                       -> model-turn executor
                       -> fixed demo provider
                       -> response write
                       -> deferred permit release
```

The demo provider will return the fixed text `FastGate local demo response.` with usage absent. It
will not call a model, use credentials, access a network, echo the prompt, or pretend that token usage
was measured. Up to four transport-valid model turns may be active at once. A fifth receives an
immediate fixed `503 Service Unavailable`; it does not wait in a queue, have its body read by
application code, or reach the provider.

The **planned invariant** is: **FastGate will serve model turns only through an actually verified TCP
loopback listener; accepted inference work will hold one of the configured permits—four by default—
from before body reading through response writing; excess work will neither wait nor dispatch; and
every acquired permit will be returned even when the HTTP handler aborts.**

## Learning objectives

After completing this unit, you should be able to:

- explain what a composition root does and why `main` selects concrete implementations;
- distinguish the configured listen address from the address of the actual bound listener;
- explain why loopback limits network reachability but does not authenticate a caller;
- distinguish a strict test fake from a stateless runtime demo;
- trace transport preflight, permit acquisition, body admission, provider execution, and response
  writing in ownership order;
- explain immediate load shedding and why this unit adds no waiting queue;
- explain why `defer` releases a permit during `http.ErrAbortHandler` panic unwinding; and
- describe how the existing bounded shutdown interacts with admitted requests.

## Why this unit matters

The required pieces already exist separately:

- [`main.go`](../../gateway/cmd/fastgate/main.go) parses one listen setting, creates the service,
  opens the listener, and transfers it to `Service.Serve`.
- [`service.go`](../../gateway/internal/service/service.go) owns the HTTP server, health handler, and
  bounded graceful-first shutdown.
- [`handler.go`](../../gateway/internal/modelturnhttp/handler.go) owns exact model-turn transport
  preflight and presentation but is not mounted in the runnable command.
- [`executor.go`](../../gateway/internal/modelturn/executor.go) bounds and validates the body before
  making at most one provider-neutral invocation.
- [`fake.go`](../../gateway/internal/provider/fake/fake.go) supplies a strict ordered test oracle with
  an explicit single-owner rule.

Simply connecting those pieces would be unsafe. A normal Go HTTP server may execute handlers
concurrently, while the strict fake mutates one finite script and forbids concurrent use. A configured
address can also differ from the concrete address returned by a listener. Finally, a reachable
handler without admission control could start an unbounded number of body reads, provider calls, and
slow response writes.

ICGT-011 will close those gaps together because each is required before the first runtime inference route
is honestly usable. It does not expand the model-turn protocol, add a live provider, or introduce the
streaming reliability work owned by M2.

## Junior engineer foundation

### A composition root chooses the concrete pieces

A **composition root** is the small startup location where interfaces are connected to concrete
implementations. In this service, that location is the runnable command. The provider-neutral
`Invoker` interface belongs in the domain boundary, but the choice to run a local demo belongs in
`main`.

A tiny example is:

```text
interface: provider.Invoker
implementation selected at startup: demo provider
consumer: model-turn executor
```

The executor should not secretly construct a demo provider. If it did, tests and future live-provider
assembly could not replace the implementation without changing domain code.

### Configuration is intent; a listener is evidence

The text `127.0.0.1:8080` in configuration expresses where FastGate intends to listen. The concrete
`*net.TCPListener` returned dynamically by the command's TCP `net.Listen` call represents what the
operating system actually bound, and its concrete `*net.TCPAddr` supplies the address evidence
immediately before serving. Checking
only an interface's reported `Addr()` is insufficient because a wrapper can report a
loopback-looking address while hiding another transport.

This matters because code can inject a listener, an address may resolve differently than expected,
or a future startup path may bind a wildcard address even though earlier validation inspected a
different string. ICGT-011 therefore plans to inspect the actual listener inside `Service.Serve`
before the server goroutine starts.

### Loopback is a network boundary, not an identity

IPv4 `127.0.0.0/8` and IPv6 `::1` are loopback address space. Traffic to them stays within the
relevant host/network environment rather than listening on a LAN or public interface.

A common misconception is that “loopback-only” means “only the trusted user.” It does not. Other
local processes can connect. Depending on WSL, container, virtualization, or host-forwarding setup,
the practical local boundary can also include a host integration. ICGT-011 adds no authentication,
authorization, TLS, or caller identity.

The approved first profile also adds no Host-header allowlist. `Host` is supplied by the caller; it
does not prove which socket accepted the connection. Checking it would create hostname, IPv4, IPv6,
port, and forwarding policy that has not been designed as an authentication mechanism. A later
security profile may constrain authority together with real authentication and TLS requirements.

This leaves browser-origin and DNS-rebinding risk deliberately unreviewed. Requiring JSON and not
granting CORS permission may prevent ordinary browser calls, but those details are not authentication
or durable security boundaries. Before a live or billable provider can be mounted in any runnable
profile—even on loopback—the Host, Origin, CORS, DNS-rebinding, and caller-authentication threat model
needs a separate reviewed decision and deterministic evidence. This includes a provider exposed
alongside rather than instead of the demo.

### A permit is temporary ownership of capacity

A **permit** is one token from a fixed-capacity collection. A request may enter the bounded region
only after acquiring a token, and it must return the token on every exit path.

The approved default is four active model turns, configurable with
`-max-concurrent-model-turns` from 1 through 16. This limit is deliberately small enough to inspect
and test. It is not a measured production throughput recommendation.

No queue is added. When all permits are held, a new valid inference request fails immediately rather
than waiting for an unknown amount of time. This behavior is called **load shedding**.

## Key concepts

### The strict fake and the demo provider have different jobs

The existing deterministic fake is an assertion tool. Its ordered script proves that a test sent an
exact request and consumed every expected exchange. Its documentation explicitly says that
`Invoke` and `VerifyComplete` must not run concurrently, and an extra or mismatched call panics with
a bounded content-safe diagnostic.

The planned demo provider has a different contract:

| Property | Existing strict fake | **PLANNED** fixed demo |
| --- | --- | --- |
| Purpose | Verify exact test interactions | Make the local walking skeleton runnable |
| State | Finite mutable script and cursor | No request-dependent mutable state |
| Concurrent runtime use | Explicitly unsupported | Safe for concurrent calls |
| Output | Test-selected scripted result/failure | Exact fixed output text |
| Prompt handling | Compares exact expected content in tests | Does not echo or retain content |
| Usage | Test-selected observation | Always absent |
| Unexpected traffic | Panics as a test-programming failure | Returns the same valid demo result |
| Network/credentials | None | None |

The planned source paths are `gateway/internal/provider/demo/demo.go` and
`gateway/internal/provider/demo/demo_test.go`. These files do not exist yet. The demo is not a
general provider simulator, and it is not evidence that real inference occurred. The one terminal
context rule is basic port compliance: if the supplied context is already canceled or expired, the
demo will return that exact context sentinel instead of the fixed result. This is not evidence for
the cancellation-conformance work deferred beyond M1.

### **PLANNED** explicit dispatch preserves exact target behavior

The current model-turn handler deliberately examines both raw and parsed request-target fields. It
rejects alternative spellings, encoded aliases, queries, and redirects rather than treating them as
equivalent to `/v1/model-turns`.

Putting a path-cleaning router in front of it could change the request before those checks run. Go's
`http.ServeMux`, for example, has useful routing and canonicalization behavior, but canonicalization
is not desirable at this exact protocol boundary.

ICGT-011 therefore plans a small explicit dispatcher in the existing service package:

- only a nonnil request whose URL is nonnil and whose parsed `URL.Path` equals `/healthz` selects the
  existing health behavior;
- every non-health request reaches the model-turn handler unchanged; and
- the dispatcher neither inspects nor rewrites `RawPath`, `RawQuery`, `RequestURI`, query values, or
  `Host`, and it never cleans, redirects, aliases, or decodes the target.

That produces a reviewable matrix. GET, HEAD, POST, queries, and an encoded spelling whose parsed
path is `/healthz` retain the existing health handler's behavior. A trailing slash, repeated slash,
dot-segment alternative, nil URL, or nil request falls through unchanged to the model-turn handler
and receives its fixed direct `404`; the service does not canonicalize or redirect it.

| Input selected by dispatcher | Existing handler behavior to preserve |
| --- | --- |
| `GET /healthz` | `200`, `ok\n`, `Cache-Control: no-store`, and `Content-Type: text/plain; charset=utf-8` |
| `HEAD /healthz` | Same handler and status/headers; the real HTTP server suppresses wire body bytes |
| `POST /healthz` | Existing `405` and `Allow: GET, HEAD` |
| `GET /healthz?probe=1` | Same health success; query remains unchanged |
| Encoded spelling parsed to `/healthz` | Existing health handler decides; dispatcher does not decode or rewrite |
| Trailing/repeated slash, dot segment, nil URL, or nil request | Unchanged model-turn handler returns its fixed `404`; no service redirect |

The planned changes belong in the existing `gateway/internal/service/service.go` and
`gateway/internal/service/service_test.go`. This is direct composition, not a new routing framework.

### **PLANNED** permit acquisition follows existing transport preflight

ICGT-010 already checks the exact target, method, encoding, declared representation trailers, and
media type before it reads the body. Those checks remain first.

Only a request that passes transport preflight attempts to acquire a permit. Consequently:

- wrong targets retain the reviewed `404` response during saturation;
- wrong methods retain the reviewed `405` response;
- unsupported representations retain the reviewed `415` response;
- none of those rejected requests consumes scarce inference capacity; and
- a transport-valid request receives either a permit or the new fixed overload response.

The planned gate therefore belongs beside the existing preflight in
`gateway/internal/modelturnhttp/handler.go`, with focused tests in the existing handler test files or
the planned `gateway/internal/modelturnhttp/concurrency_test.go`. A service-level wrapper could only
place a permit before the private preflight or duplicate transport rules; either choice would weaken
one reviewed owner.

### **PLANNED** permit ownership covers the complete expensive region

Once acquired, one permit stays held through:

1. the existing bounded request-body read;
2. strict JSON and semantic admission;
3. at most one provider invocation;
4. response preparation; and
5. the application response write.

Holding the permit through the write matters because a slow client can keep a handler and response
resources active even after the provider returned. Releasing earlier would make the advertised
“active model turns” limit narrower than the work users actually cause.

The permit does **not** bound:

- listening or accepted TCP connections;
- the initial `net/http` goroutine created for a request;
- health requests;
- requests rejected by transport preflight; or
- a saturated transport-valid request while its unpermitted handler writes the fixed `503`; or
- operating-system and HTTP-server buffers outside application ownership.

Those limits require different evidence and remain later production work.

### **PLANNED** saturation returns one small fixed response

When all four default permits are held, the planned response is:

```text
status: 503 Service Unavailable
content-type: text/plain; charset=utf-8
cache-control: no-store
x-content-type-options: nosniff
body: service unavailable\n
```

There is no `Retry-After` header. FastGate has no queue position, refill schedule, or delay estimate,
so inventing one would be misleading. The request body is not read by application code and the
provider is not invoked. The service does not retry the operation.

`503` describes temporarily unavailable local service capacity. `429 Too Many Requests` would imply
a caller rate or quota policy, which ICGT-011 does not implement.

### **PLANNED** `defer` returns capacity during abort panics

The current presentation layer uses the exact `http.ErrAbortHandler` panic when an invoked provider
returns cancellation or deadline termination matching the caller context. Go's HTTP server treats
that sentinel as “abort this response without logging another stack trace.” An outer layer must not
recover it, rewrite it, or emit a second response.

Go runs deferred calls while unwinding a panic. The planned gate therefore releases its permit with
`defer` immediately after acquisition, then lets any panic continue unchanged.

**PSEUDOCODE ONLY — planned permit lifetime:**

```text
if transport preflight rejects:
    write the existing rejection
    return

if no permit is immediately available:
    write fixed 503
    return

defer release permit
execute admitted body and provider path
prepare and write exactly one response
```

The defer also covers ordinary returns and unexpected panics. It is cleanup, not panic recovery.

### **PLANNED** actual-listener admission happens before serving

After the context and listener required-input checks pass, `Service.Serve` receives ownership of the
nonnil listener. Before starting the HTTP server, the planned code will require the listener itself to be a
`*net.TCPListener` and its address to be a concrete `*net.TCPAddr` containing a loopback IP.

Accepted examples include:

```text
127.0.0.1:8080
[::1]:8080
```

Rejected examples include:

```text
0.0.0.0:8080       wildcard IPv4
[::]:8080          wildcard IPv6
192.168.1.20:8080  LAN address
203.0.113.10:8080  non-loopback address
/tmp/fastgate.sock  non-TCP listener
wrapped listener    may only report a loopback-looking address
```

The existing nil-context check remains first. It returns exactly
`serve FastGate: context is required`, makes no listener close attempt, and leaves any supplied
listener with the caller. After a nonnil context is validated, a nil listener interface or typed-nil
`*net.TCPListener` is a missing input, not an owned rejection. It returns the existing exact error
`serve FastGate: listener is required` and makes no close attempt because no usable resource exists.

For every nonnil admission rejection, the base error remains exactly
`serve FastGate: listener must be a loopback TCP listener`. It does not copy an arbitrary listener
address into a diagnostic. Because ownership has transferred to `Serve`, that rejected listener is
closed exactly once before the method returns. Ownership transfers only after the nonnil context and
nonnil listener checks. If the close also fails, the returned error joins only the fixed cleanup
category `close rejected FastGate listener`; it deliberately does not wrap or reveal the arbitrary
raw close error. The server goroutine and provider path never start.

Checking only `Config.ListenAddress` would prove the requested text, not the bound socket. Checking
only the actual listener is the decisive runtime safety gate; configuration remains useful startup
intent and validation input.

Tests need not bind an arbitrary public or LAN address to prove the IP rule. Production keeps the
`*net.TCPListener` type assertion visible, then may call one private `*net.TCPAddr` classifier. Real
loopback and wildcard TCP listeners prove the dynamic boundary, while pure classifier cases cover
unspecified, LAN, and global addresses. A custom wrapper that reports a loopback address still proves
rejection and cleanup—it cannot use the classifier to bypass the concrete-listener check.

### Existing bounded shutdown remains the lifecycle owner

[`Service.Serve`](../../gateway/internal/service/service.go) already starts one server goroutine,
waits for serving to end or the process context to be canceled, calls `Shutdown` with an independent
bounded context, force-closes if graceful shutdown fails, and joins the server result before
returning.

ICGT-011 plans to preserve that lifecycle:

- no new worker pool or queue needs a second shutdown protocol;
- stopped acceptance prevents new network work;
- an already admitted request may finish during the existing shutdown grace;
- its permit remains held until its handler exits;
- if graceful shutdown times out, server closure ends local HTTP resources; and
- deferred cleanup returns the permit as the handler exits.

The story will not attach immediate request cancellation through `http.Server.BaseContext`, because
that would terminate already accepted work instead of allowing the reviewed graceful drain.

Local shutdown does not prove that a future remote provider stopped billable work. ICGT-011's demo
has no remote work, while later cancellation and provider-cleanup stories must define that evidence.

## Architecture and invariants

| Boundary | Planned owner and invariant |
| --- | --- |
| Provider choice | The command explicitly selects the fixed demo; the strict fake remains test-only. |
| Transport | Existing preflight retains `404`, `405`, and `415` behavior before capacity admission. |
| Capacity | At most the configured 1-16 valid model turns enter body admission; excess work waits nowhere and dispatches zero times. |
| Health | The service dispatcher selects only parsed `/healthz`, passes requests unchanged, and bypasses inference permits. |
| Listener | `Service.Serve` verifies a concrete loopback `*net.TCPListener`, closes a rejection, and starts no server first. |
| Cleanup | Deferred permit release preserves abort panics; existing bounded shutdown force-closes if draining fails and joins. |
| Repository boundary | FastGate owns transport; Code Assist Harness continues to own workflow, tools, approvals, and correctness. |

Deferred work includes live providers, credentials, streaming, backpressure, queues, fairness,
retries, cancellation conformance, telemetry, connection limits, Host/Origin/CORS policy,
DNS-rebinding mitigation, authentication, TLS, wrapped or non-loopback serving, and the future Code
Assist Harness adapter. No visual lesson is planned.

## Practical walkthrough

Implementation is planned in four human-sized checkpoints.

### Checkpoint 1: Stateless demo provider

Add the planned `gateway/internal/provider/demo/demo.go` and its focused test. Construct one valid
immutable provider result containing `FastGate local demo response.` and absent usage. Prove repeated
and concurrent invocations return the same valid value without echoing request content or sharing
mutable request state.

Review pause: explain why this runtime implementation satisfies the provider port while the strict
fake remains the better assertion tool in tests.

### Checkpoint 2: Post-preflight permits

Extend the existing model-turn HTTP handler so successful preflight attempts a nonblocking permit
before calling the executor. Add exact overload response evidence, application zero-read evidence,
zero provider calls, capacity reuse, and panic-safe release. Keep the permit through the response
write.

Review pause: trace one accepted request, one saturated request, one malformed transport request,
and one `http.ErrAbortHandler` path.

### Checkpoint 3: Dispatcher and actual-listener safety

Extend the service with explicit health/model-turn dispatch and actual-listener validation. Prove
IPv4 and IPv6 loopback `*net.TCPListener` acceptance, fixed rejection of wildcard, non-loopback,
non-TCP, and wrapped listeners, exact rejection cleanup, and no server start on rejection. Prove the
parsed-path health matrix stays responsive while all inference permits are held.

Review pause: identify which address is configuration and which object supplies runtime evidence.

### Checkpoint 4: Command composition and end-to-end evidence

Extend `gateway/cmd/fastgate/main.go` and add the planned
`gateway/cmd/fastgate/main_test.go`. Thread the `-max-concurrent-model-turns` value through startup,
validate the inclusive 1-16 range, then assemble demo provider, executor, handler, service, and
listener explicitly.

Run one real proxy-disabled loopback exchange through the complete process-equivalent path. Bound and
close every response body. Exercise shutdown without synchronization sleeps. Then update this lesson
from planned design to exact implementation excerpts and validation results.

## Personal code review map

### Existing evidence to understand before implementation

| Current review path | Why it matters | Question to answer |
| --- | --- | --- |
| [`main.go`](../../gateway/cmd/fastgate/main.go) | Current composition root and listener creation | Where will concrete provider selection belong? |
| [`service.go`](../../gateway/internal/service/service.go) | Current server ownership and bounded shutdown | Where can the actual listener be rejected before serving starts? |
| [`service_test.go`](../../gateway/internal/service/service_test.go) | Real loopback and controlled shutdown patterns | How are resources joined without sleeps? |
| [`handler.go`](../../gateway/internal/modelturnhttp/handler.go) | Exact transport preflight before body admission | Why must permit acquisition follow this preflight? |
| [`server_test.go`](../../gateway/internal/modelturnhttp/server_test.go) | Existing real-server abort and serial-loopback evidence | Which semantics require a real HTTP server rather than only direct handler calls? |
| [`executor.go`](../../gateway/internal/modelturn/executor.go) | Bounded request admission and one provider invocation | What work begins after a permit is acquired? |
| [`fake.go`](../../gateway/internal/provider/fake/fake.go) | Finite, strict, single-owner fake contract | Why is this not the command's runtime provider? |

### **PLANNED** implementation review paths

These paths describe the accepted file plan. New paths are intentionally not linked because they do
not exist yet.

| Planned path | Review focus |
| --- | --- |
| `gateway/internal/provider/demo/demo.go` | Stateless fixed result, absent usage, no content echo |
| `gateway/internal/provider/demo/demo_test.go` | Repeatable and concurrent behavior under the race detector |
| `gateway/internal/modelturnhttp/handler.go` | Preflight-before-permit order, full permit lifetime, fixed overload response |
| `gateway/internal/modelturnhttp/concurrency_test.go` if needed | Channel-controlled saturation, zero read/dispatch, reuse, abort release |
| `gateway/internal/service/service.go` | Explicit dispatch, actual-listener validation, rejection cleanup, unchanged shutdown |
| `gateway/internal/service/service_test.go` | Loopback matrix, no-start rejection, health bypass, bounded lifecycle |
| `gateway/cmd/fastgate/main.go` | Explicit composition and bounded CLI setting |
| `gateway/cmd/fastgate/main_test.go` | Startup validation and command-equivalent local exchange |

The most important production path for personal review will be transport preflight through permit
acquisition and deferred release in `handler.go`. The most important failure paths will be the
channel-controlled saturation test and actual-listener rejection/closure test. The reviewer should
be able to explain the central invariant at the top of this lesson and identify M2 behavior that is
still absent.

## Planned code samples

No ICGT-011 implementation excerpt exists yet. The following examples express control-flow intent,
not final Go APIs or syntax.

### **PSEUDOCODE ONLY — happy path and owned cleanup**

```text
config = parse and validate startup flags
demo = construct fixed-output demo provider
executor = construct model-turn executor(demo)
handler = construct bounded model-turn HTTP handler(executor, max concurrency)
service = construct HTTP service(config, handler)
listener = bind configured TCP address
serve(ctx, listener):
    return context-required error and retain caller ownership when ctx is nil
    return required-listener error without close when listener is nil or typed-nil TCP
    otherwise reject and close unless it is *net.TCPListener with loopback *net.TCPAddr
    start existing bounded server lifecycle

on model-turn request:
    preserve existing preflight rejection, if any
    fail with fixed 503 unless a permit is immediately available
    defer permit release
    execute bounded body and provider path
    prepare and write one response
```

Each constructor remains explicit. Listener inspection happens before the server goroutine, and the
permit has only two acquisition states: succeed now or fail now. It never waits.

### **PSEUDOCODE ONLY — failure-path test**

```text
hold four admitted requests with channels
assert a fifth valid request gets exact 503 with zero body reads and dispatches
assert invalid transport and health responses remain available
release one held request and assert later valid work enters
panic admitted work with exact ErrAbortHandler
assert the panic propagates and its returned permit admits later work
release and join every goroutine
```

Channels provide observable checkpoints without scheduler-dependent sleeps.

## Failure scenarios to study

| Scenario | Responsible boundary | Safe planned outcome | Deterministic evidence |
| --- | --- | --- | --- |
| Context is nil with any listener | Service input validation | Exact context-required error; caller retains listener; no server/provider start | Nil and nonnil listener cases |
| Listener is nil or typed-nil TCP | Service input validation | Exact required-listener error; no close or server/provider start | Direct missing-input cases |
| Actual listener is wildcard, LAN, wrapped, or non-TCP | Service listener admission | Close once; exact fixed error; no server/provider start | Real TCP listeners plus rejection/cleanup seam |
| Rejected-listener close also fails | Service listener cleanup | Add only fixed cleanup category; omit raw cause | Exact joined-error and `errors.Is` test |
| Concurrency setting is 0 or above 16 | Startup/config validation | Reject before listener creation | Table-driven command/config test |
| Four valid requests are active | Model-turn permit gate | Fifth gets exact 503; zero body reads/provider calls | Channel-controlled saturation test |
| Invalid media request arrives while saturated | Existing transport preflight | Preserve exact 415; consume no permit | Saturated preflight precedence test |
| Health or an alternate health spelling arrives while saturated | Explicit dispatcher | Preserve parsed-path health behavior or unchanged model-handler 404 | Held permits plus dispatch matrix |
| Admitted handler panics with `http.ErrAbortHandler` | Deferred permit cleanup | Release permit and propagate exact sentinel | Recover only in test; then acquire again |
| Process context ends during active work | Existing service lifecycle | Stop accepting, bounded drain, force close if required, join | Controlled handler channels and deadline |
| Demo receives prompt content | Demo provider | Return fixed output without reflecting content | Distinct sensitive-marker inputs yield same result |

The tests must not log request bodies or embed real secrets. A clearly fake marker can prove that
output does not derive from input.

Minimum validation also covers limits 1 and 16 plus rejection at 0 and 17, exact overload headers,
demo concurrency under the race detector, the complete health dispatch matrix, real IPv4 and
available IPv6 loopback listeners, real wildcard rejection, pure LAN/global address classification,
wrapped-listener rejection, and one real proxy-disabled loopback exchange through the full assembly.
Focused repeated tests, race tests, `go vet`, the PR review checklist, an
independent concurrency/cleanup/security review, and the offline credential-free `./scripts/check`
remain required before Done.

## User-visible speed and response-size impact

After implementation, acquiring and releasing a permit for an admitted local demo request will be
constant work. The demo adds no external network or model-execution step, but that design fact is not
benchmark evidence about latency or FastGate production throughput.

The concurrency limit changes behavior only when requests overlap:

- the first four valid model turns may progress together by default;
- an overlapping fifth fails immediately instead of waiting;
- immediate failure reduces queue time and retained memory at the cost of requiring the caller to
  decide whether and when to try again; and
- changing the limit controls simultaneous work, not the speed of one accepted request.

The demo output text is always 29 ASCII bytes, and usage is omitted. The completed JSON response also
contains the normal protocol framing and caller request ID, so total response size varies with that
bounded identifier. The limit does not make each successful response larger. The overload body is a
fixed 20 bytes: `service unavailable` plus one line feed.

Request-body and output bounds from earlier stories do not change. Streaming in M2 will change when
bytes are delivered and add event framing; it is deliberately not inferred from this fixed
non-streaming demo.

## What changed during implementation

Implementation evidence does not exist yet. The approved readiness discussion selected:

- a default-enabled stateless fixed demo instead of the finite strict fake, prompt echo, or an
  always-unavailable placeholder;
- concrete `*net.TCPListener` loopback enforcement instead of trusting configuration text or an
  arbitrary wrapper's reported address;
- no Host allowlist because Host does not authenticate the connection;
- an explicit prerequisite to review browser-origin, DNS-rebinding, CORS, and authentication policy
  before a live or billable provider can be mounted in any runnable profile, alongside or instead of
  the demo;
- one configurable nonblocking limit, default 4 and valid from 1 through 16;
- permit acquisition after transport preflight and before body reading;
- permit ownership through the response write;
- immediate fixed `503` with no queue or `Retry-After`; and
- reuse of the existing bounded shutdown rather than a worker-pool lifecycle.

After code exists, replace this section with observed constraints, failed assumptions, review-driven
changes, exact source excerpts, and validation results.

## Production expansion

### Example production scenario

Suppose a future authenticated FastGate fronts a live provider for many tenants. A fixed process-wide
limit of four would not express per-tenant fairness, provider quotas, connection pressure, different
model costs, or live saturation signals. Immediate load shedding might protect the process but reject
too much useful work during brief bursts.

A production expansion could introduce measured global and per-route limits, a small bounded queue
with explicit deadlines, fairness, connection controls, telemetry, and authenticated non-loopback
transport. Each additional mechanism needs an owner and overload contract. It should not silently
turn this unit's immediate failure into unbounded waiting.

### Representative capabilities and tools

- Go's [`net/http`](https://pkg.go.dev/net/http) and
  [`net`](https://pkg.go.dev/net) packages provide the server and listener primitives used here.
- [`golang.org/x/sync/semaphore`](https://pkg.go.dev/golang.org/x/sync/semaphore) supports weighted
  permits and waiting acquisition; those capabilities are unnecessary for this fixed nonblocking
  gate and would add a dependency and queue semantics.
- [Envoy circuit breaking](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/circuit_breaking)
  can bound upstream connections and pending requests, with additional proxy configuration and
  operational ownership.
- [OpenTelemetry metrics](https://opentelemetry.io/docs/specs/otel/metrics/) can expose active work,
  overload counts, and latency, with schema, cardinality, collection, and retention costs.
- [Kubernetes readiness probes](https://kubernetes.io/docs/concepts/configuration/liveness-readiness-startup-probes/)
  can stop routing new traffic to an unready instance, but readiness is not authentication or
  concurrency control.

These are comparisons, not ICGT-011 dependencies.

### Local versus production

| Dimension | ICGT-011 local profile | Possible production expansion |
| --- | --- | --- |
| Provider | Stateless fixed demo | Live, capability-aware adapters |
| Network | Verified concrete TCP loopback; browser-origin risk unreviewed | Authenticated TLS and reviewed Host/Origin/CORS/proxy policy |
| Capacity | One process-wide active limit | Measured route/tenant/provider limits |
| Overload | Immediate fixed 503 | Explicit bounded queue, deadlines, fairness, or adaptive shedding |
| Connections | Existing `net/http` behavior | Reviewed connection and listener limits |
| Observability | Deterministic tests | Bounded metrics, tracing, alerts, and capacity dashboards |
| Shutdown | Existing bounded local drain | Deployment-aware draining and remote-cleanup evidence |

### Trade-offs and graduation signals

Immediate rejection is simple, bounds waiting memory, and makes overload deterministic. It can waste
brief spare capacity when bursts would fit safely in a tiny queue. A queue adds latency, cancellation,
fairness, memory, and shutdown obligations.

Graduate only after measurements show a need. Useful signals include sustained 503 rate, permit
utilization, request/service-time distributions, slow response writes, provider quota behavior,
tenant fairness requirements, or a concrete non-loopback client requirement. A tool name or a desire
for “more scale” is not sufficient evidence.

## Practical exercises

1. Before running the planned saturation test, predict the outcomes of four held inference requests,
   a fifth valid request, one invalid-media request, and a simultaneous health request.
2. Trace ownership of the configured address, actual listener, accepted permit, request context,
   HTTP server, and final serve result. Mark the exact point where each resource is released.
3. Compare full-handler limiting, provider-only limiting, and a bounded waiting queue on paper. List
   what each option bounds and the new cleanup behavior it would require; do not implement the
   alternatives.

## Key takeaways

- The command is the composition root; it explicitly chooses the stateless demo without changing the
  provider-neutral executor.
- The concrete `*net.TCPListener` and its `*net.TCPAddr`, not configuration text, a wrapper's report,
  or the Host header, prove that the process bound TCP loopback. Loopback still does not authenticate
  callers or address browser-origin risk.
- Transport preflight precedes the permit gate, so detailed transport failures remain stable and do
  not consume inference capacity.
- A permit covers body admission through response writing, and a deferred release runs during normal
  return or panic unwinding without recovering `http.ErrAbortHandler`.
- Immediate fixed 503 shedding bounds active inference work without creating a queue, retry promise,
  or quota policy.
- This local demo completes M1; streaming reliability begins in M2 and live providers begin in M3.

## Glossary

- **Actual listener:** For this profile, the concrete `*net.TCPListener` and `*net.TCPAddr` returned
  after the operating system binds a socket; runtime evidence distinct from configuration intent or a
  wrapper's reported address.
- **Composition root:** The startup location that selects concrete implementations and connects them
  to interfaces.
- **Concurrency:** Multiple requests making progress during overlapping time.
- **Dispatcher:** Small routing code that selects a handler without changing the request target.
- **Load shedding:** Rejecting excess work promptly to protect bounded capacity.
- **Loopback:** Host-local IP address space such as IPv4 `127.0.0.0/8` or IPv6 `::1`; not caller
  authentication.
- **Permit:** A token granting temporary ownership of one slot in a bounded concurrent region.
- **Panic unwinding:** Go's process of running deferred calls while a panic travels up the call stack.
- **Strict fake:** A test implementation that verifies exact scripted interactions and treats
  mismatches as test-programming errors.
- **Stateless demo provider:** The planned runtime `Invoker` that returns one fixed valid result
  without request-dependent mutable state, network calls, or credentials.

See also the shared [repository glossary](../glossary.md).

## Teach-back questions

1. Why must FastGate inspect the actual listener instead of trusting only the configured address, and
   why is a nonnil rejected listener closed while a missing listener is not?
2. What work does one model-turn permit bound, and why must its deferred release run when
   `http.ErrAbortHandler` panics?
3. Why is the stateless fixed demo suitable for concurrent local runtime use while the strict
   deterministic fake remains test-only?

## Further reading

- [ICGT-011 story](../../user-stories/icgt-011-bind-local-demo-runtime.md)
- [ICGT-010 implementation lesson](icgt-010-model-turn-http-presentation.md)
- [ICGT-005 lifecycle lesson](icgt-005-go-service-lifecycle.md)
- [ADR 0005: bounded loopback-only demo runtime](../adr/0005-local-demo-runtime-profile.md)
- [ADR 0002: fake first, OpenAI first live](../adr/0002-fake-first-openai-first-live.md)
- [ADR 0003: FastGate-owned model-turn protocol](../adr/0003-fastgate-api-surface.md)
- [FastGate architecture boundary](../architecture.md)
- [Go `net.Listener`](https://pkg.go.dev/net#Listener)
- [Go `net.IP.IsLoopback`](https://pkg.go.dev/net#IP.IsLoopback)
- [Go `http.ErrAbortHandler`](https://pkg.go.dev/net/http#ErrAbortHandler)
- [Go `defer`, panic, and recover](https://go.dev/blog/defer-panic-and-recover)
