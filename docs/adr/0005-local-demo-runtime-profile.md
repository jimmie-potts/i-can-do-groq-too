# ADR 0005: Use a bounded loopback-only demo as FastGate's first runnable profile

- **Status:** Accepted
- **Date:** 2026-08-06
- **Accepted:** 2026-08-06
- **Scope:** ICGT-011 runtime provider, listener trust boundary, request concurrency, and overload behavior

## Context

ICGT-010 implements FastGate's strict model-turn HTTP handler but deliberately leaves it unmounted.
The default command still serves only the operational `/healthz` surface (`GET`, with implicit
`HEAD`). Making the inference route client-reachable requires four choices that must agree:

1. which provider-neutral invoker answers unscripted runtime requests;
2. which network listeners are allowed without caller authentication or TLS;
3. how many admitted model turns may retain bodies and run concurrently; and
4. what a caller observes when that capacity is full.

The strict deterministic fake from ICGT-008 is not a runtime provider. It owns one finite ordered
script, has one caller, and intentionally panics when traffic differs from the test. A normal HTTP
server may invoke the same handler concurrently and for an unknown number of requests. Putting the
fake behind a mutex would remove one data race without fixing script exhaustion, mismatch panics, or
unbounded waiting.

One admitted request may retain an encoded body of up to 8 MiB before semantic validation. A provider-
only limit would therefore leave concurrent body admission unbounded. Conversely, putting a permit in
front of every transport check would make overload replace the exact `404`, `405`, and `415` behavior
reviewed in ICGT-010.

ICGT-011 is the final M1 unit. It needs a real local happy path without introducing a live-provider
SDK, credentials, streaming, or a public deployment profile.

## Runtime-provider options

### A. Mount the strict deterministic fake

This would reuse the existing test double directly.

Advantages:

- no new provider package; and
- exact scripted outcomes already exist for tests.

Costs:

- ordinary extra or different requests become assertion panics;
- the fake is finite and single-owner rather than concurrency-safe; and
- serialized callers could still wait without a bound and exhaust the script.

### B. Return an unavailable failure for every admitted request

This would make the route safe to call but deliberately prevent completion.

Advantages:

- stateless and small; and
- clearly avoids pretending to run a model.

Costs:

- M1 would still lack a runnable completed path; and
- learners could not trace a successful request through the whole service.

### C. Use a stateless fixed-output local demo

This provider returns one fixed, valid result whenever its initial context observation is active.

Advantages:

- concurrency-safe without a mutex, script, credential, or network;
- completes the local walking skeleton; and
- cannot echo prompt content or fabricate token usage.

Costs:

- it is a demonstration, not model inference; and
- a second provider implementation exists alongside the stricter test oracle.

### D. Add a live provider now

This would make the first runnable endpoint call OpenAI, Groq, or another external service.

Advantages:

- produces genuine model output.

Costs:

- introduces credentials, SDK or wire behavior, network failures, billing, and retry policy before
  the local runtime boundary is proven; and
- violates ADR 0002's fake-first, OpenAI-first-live sequence.

## Listener and authority options

### A. Trust only the configured listen string

This checks what the user requested before `net.Listen` but not what was actually bound.

The configuration and the listener are different evidence. Name resolution, injected listeners, and
future startup refactors can make the actual socket differ from the original string.

### B. Verify a concrete TCP listener bound to loopback

`Service.Serve` checks the supplied listener before starting `http.Server.Serve`. The listener
itself must be a `*net.TCPListener`, and its concrete `*net.TCPAddr` must contain an IPv4 or IPv6
loopback IP. A wrapper that merely reports a loopback-looking address is not evidence that the
underlying listener is TCP. The existing context check still runs first: a nil context returns its
fixed context-required error and leaves listener ownership with the caller. After a nonnil context is
validated, a nil listener interface or any nil dynamic listener value remains the existing
missing-listener input error. That includes a typed-nil `*net.TCPListener` or custom pointer wrapper;
neither `Addr` nor `Close` may be called because no usable resource exists. Wildcard, non-loopback,
Unix, wrapped, and unknown dynamically nonnil listener forms are rejected, and each rejected owned
listener is closed.

This proves where the process is listening. It does not identify or authenticate the caller.

### C. Treat `Host` as an authorization rule

The service could allow a list of Host authorities in addition to checking the socket.

That can be useful in a later browser, reverse-proxy, or non-loopback threat model. For this profile,
however, `Host` is caller-controlled and would not authenticate another local process. A correct rule
would also need reviewed hostname, port, IPv4, IPv6, proxy, and deployment semantics. Adding a small
allowlist now could create a misleading security claim.

Leaving `Host` unrestricted also leaves browser-origin and DNS-rebinding risk unreviewed. Requiring
JSON and omitting CORS permission may block ordinary browser clients, but neither is an authentication
boundary or a durable defense against a hostile local or browser-origin caller. That threat model must
be reviewed before the runnable command can mount a live or billable provider, even if the listener
remains loopback-only.

## Concurrency options

### A. Leave handler concurrency unbounded

This preserves the simplest path but allows an unknown number of valid requests to retain bodies,
invoke the provider, and wait on response writes.

### B. Limit only provider calls

This protects a future provider but allows every concurrent request to read and retain up to 8 MiB
before reaching the limit.

### C. Wait in a bounded or unbounded queue

A queue can smooth short bursts, but it introduces wait time, queue capacity, cancellation, fairness,
and timeout policy. M1 has no measurements that justify those policies.

### D. Fail fast after transport preflight

A process-wide nonblocking permit gate runs after ICGT-010's target, method, and representation checks
but before the body is read. A permit remains owned through provider execution and response writing.
When no permit is available, the request receives one fixed `503` without waiting or dispatching.

This bounds the expensive model-turn path while preserving deterministic `404`, `405`, and `415`
responses. It does not bound accepted TCP connections, header parsing, `net/http` request goroutines,
the cheap transport-rejection paths, or a saturated request while it writes the fixed `503` without a
permit.

## Decision

Select runtime option C, listener option B without option C, and concurrency option D.

ICGT-011 will implement this exact first runnable profile:

- the default command constructs a stateless local demo invoker, the existing executor, the model-turn
  HTTP handler, and the service explicitly;
- the demo returns output text `FastGate local demo response.` and omits usage;
- it does not inspect or echo prompt content, make network calls, read credentials, retry, stream, or
  claim to run a model;
- `Invoke` observes `ctx.Err()` once immediately before selecting its outcome; an observed
  `context.Canceled` or `context.DeadlineExceeded` returns a zero result and that exact sentinel,
  while a nil observation selects the immutable demo result and is not replaced by later
  cancellation;
- `Service.Serve` accepts only a dynamic `*net.TCPListener` whose `*net.TCPAddr` contains a loopback
  IP; a listener wrapper is rejected even when its reported address looks like loopback TCP;
- validation preserves the existing order: a nil context returns exactly
  `serve FastGate: context is required`, makes no listener close attempt, and leaves any supplied
  listener with the caller;
- after a nonnil context is validated, a nil listener interface or any nil dynamic listener value,
  including a typed-nil `*net.TCPListener` or custom pointer wrapper, returns exactly
  `serve FastGate: listener is required` without calling `Addr` or `Close` because no usable resource
  exists;
- only then does listener ownership transfer for every dynamically nonnil listener, so an admission
  rejection is closed exactly once before an error is returned;
- listener rejection returns exactly `serve FastGate: listener must be a loopback TCP listener`; if
  that close also fails, the returned joined diagnostic is exactly that line followed by
  `close rejected FastGate listener`, and the arbitrary raw close error is deliberately not wrapped or
  exposed;
- no Host-header allowlist is added, and loopback is documented as reachability rather than caller
  authentication;
- a custom dispatcher keeps health separate without putting a path-cleaning `http.ServeMux` in front
  of the inference handler;
- transport target, method, encoding, and media-type rejections retain ICGT-010 behavior and consume
  no inference permit;
- `-max-concurrent-model-turns` selects a positive limit from 1 through 16, with default 4;
- malformed startup arguments produce exactly `parse startup configuration: invalid arguments`;
  the command neither emits nor returns the raw token or the standard `flag` package diagnostic, and
  an invalid numeric value remains invalid even if a later duplicate flag is valid;
- one nonblocking permit is acquired after successful transport preflight and before any body read;
- the permit remains held through the response write and is released with `defer`, including when
  `http.ErrAbortHandler` unwinds the handler;
- health traffic bypasses inference permits;
- saturation returns `503 Service Unavailable`, body `service unavailable\n`, media type
  `text/plain; charset=utf-8`, `Cache-Control: no-store`, and
  `X-Content-Type-Options: nosniff`;
- saturation reads no request body, invokes no provider, waits in no queue, starts no retry, and sends
  no `Retry-After`; and
- ICGT-005's graceful-first bounded shutdown remains the lifecycle policy.

The strict fake remains the assertion-oriented test oracle. ICGT-011 may use controlled test handlers
or invokers to prove concurrency, but the runnable command never mounts a finite script.

## User impact

After ICGT-011 is implemented, starting FastGate with its defaults will expose both health and one
local deterministic model-turn route. A successful response has the normal bounded model-turn JSON
shape and the fixed demo output. It does not become larger because of the permit check, and the check
adds only one in-process nonblocking acquisition; this ADR makes no benchmark or latency promise.

The fifth simultaneous transport-valid request at the default limit may receive a small immediate
`503` rather than wait. A user may select 1 through 16 active requests at startup. Invalid target,
method, encoding, or media type still receives its specific transport response even while model-turn
capacity is full, and health remains available.

A malformed concurrency argument now produces one short generic startup error instead of echoing the
supplied value. This is intentionally less detailed because command arguments can accidentally
contain sensitive or extremely long content.

Any process able to reach the loopback socket may call the unauthenticated demo. Host integrations
around WSL or container networking may also make a loopback-bound service reachable from the local
host. A hostile web page and DNS rebinding are not covered by this profile; strict JSON and absent
CORS permission must not be treated as security guarantees. This is an explicitly local learning
profile, not a safe remote deployment profile.

## Consequences and sequencing

- ICGT-011 can remain one story because provider choice, listener admission, and bounded work are all
  prerequisites for one observable safe local endpoint. None lands as a partially reachable runtime.
- The implementation stays standard-library-only and adds no `go.sum`.
- The first runtime limit is a reviewed safety bound, not a throughput recommendation. Later measured
  work may change it through a new decision.
- Queues, fairness, per-tenant limits, rate policy, connection limits, readiness, telemetry, and
  backpressure remain unimplemented.
- Authentication, TLS, CORS, trusted-proxy behavior, Host policy, and non-loopback listeners require a
  separately reviewed profile before remote deployment. A reviewed Host, Origin, CORS, DNS-rebinding,
  and caller-authentication threat model is also a prerequisite before any live or billable provider
  is mounted in any runnable profile, even on loopback and even alongside the demo. A future opt-in adapter may
  be implemented and tested only through non-runnable seams until that prerequisite is satisfied.
- Requiring a concrete `*net.TCPListener` intentionally rejects listener wrappers such as TLS
  listeners. Any later TLS or wrapped-listener runtime profile must revise this admission decision
  rather than weakening it silently.
- The demo's single initial context observation is only basic provider-port behavior. ICGT-015 owns
  cancellation propagation and race evidence, ICGT-016 owns upstream deadlines and cleanup grace,
  and ICGT-017 owns slow-client backpressure and memory bounds after ICGT-012 introduces SSE framing.
- OpenAI remains the first live provider in M3 after deterministic streaming and operational evidence.
- Code Assist Harness remains a separate client. ICGT-011 changes no harness workflow, tool, approval,
  transcript, retry, or correctness behavior.
- If ICGT-011 exceeds roughly 400 changed production lines, needs more than two new production
  abstractions, or begins owning any deferred policy above, implementation stops for split review.
