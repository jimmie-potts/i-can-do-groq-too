# ICGT-010 lesson: Presenting model-turn v1 over HTTP

- **Unit:** ICGT-010
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; no HTTP presentation package or inference route exists yet
- **Story:** [ICGT-010](../../user-stories/icgt-010-present-model-turn-over-http.md)
- **Review priority:** High
- **Visual companion:** Not required; the Markdown lesson is the planned learning artifact
- **Related architecture:** [ADR 0002](../adr/0002-fake-first-openai-first-live.md),
  [ADR 0003](../adr/0003-fastgate-api-surface.md),
  [model-turn v1](../../gateway/contracts/model-turn/v1/README.md), and
  [ICGT-009 bounded admission](../../user-stories/icgt-009-admit-and-execute-model-turn.md)

> This lesson describes approved planned behavior, not observed runtime behavior. The proposed
> `gateway/internal/modelturnhttp` package does not exist yet. The default FastGate command still
> serves only `GET /healthz`; ICGT-010 does not register an inference route with that command.

## Quick summary

ICGT-009 already accepts bounded request bytes, enforces the strict model-turn v1 contract, rejects
unsupported requests before provider work, and returns one closed `modelturn.Outcome`. It deliberately
does not decide HTTP paths, methods, media types, statuses, headers, or response serialization.

ICGT-010 plans the next layer: one small HTTP handler that presents those existing outcomes without
changing their meaning. The handler will recognize exactly `POST /v1/model-turns`, reject transport
mistakes before it calls `Request.Body.Read`, pass the request context and body directly to the
existing executor, and map each valid non-terminated closed outcome to one bounded HTTP presentation.
A matching caller-context termination instead aborts the response without inventing protocol meaning.

This is a handler and presentation story only. It does not attach the handler to the default
executable, select a runtime provider, enforce a process listener, or define concurrent use of the
single-owner deterministic fake. ICGT-011 owns that runtime assembly and concurrency policy.

The central invariant is: **the HTTP layer may present an existing FastGate outcome, but it must not
weaken admission, invent provider meaning, retry work, or expose the handler as a runtime service
before its owner and concurrency policy exist.**

## Learning objectives

After completing this unit, you should be able to:

- distinguish HTTP transport rejection from model-turn protocol failure;
- explain why exact target, method, encoding, and media-type checks happen before body admission;
- trace one `modelturn.Outcome` into a status, media type, headers, and bounded body;
- preserve provider failure code, retryability, and optional usage without reinterpretation;
- explain why request termination and response-write failure cannot authorize another provider call;
- identify which layer owns server request-body closure; and
- distinguish a tested HTTP handler from a route exposed by the runnable FastGate process.

## Why this unit matters

The current implementation already contains the two boundaries needed on either side of an HTTP
presentation layer:

- [`Executor.Execute`](../../gateway/internal/modelturn/executor.go) turns one `context.Context` and
  bounded `io.Reader` into a closed outcome.
- [`Outcome`](../../gateway/internal/modelturn/outcome.go) exposes enough information for presentation
  without exposing mutable or mixed internal state.

Without one reviewed owner for the translation between those values and an HTTP response, several
mistakes are easy:

- invalid media types could reach provider admission;
- the handler could duplicate or drift from the existing 8 MiB request limit;
- provider failure usage could be silently dropped;
- upstream authentication failure could be mislabeled as caller authentication failure;
- cancellation could be converted into an invented provider error;
- a failed response write could accidentally cause another response or provider retry; or
- a package-level handler could be described as a runnable inference service even while the command
  still serves health only.

ICGT-010 isolates those decisions before runtime listener and concurrency behavior are added.

## Junior engineer foundation

### An HTTP response has independent parts

An HTTP response includes:

- a numeric status such as `200`, `400`, or `503`;
- headers such as `Content-Type` and `Cache-Control`; and
- a body containing JSON, plain text, or no application-written data.

The status gives a broad HTTP classification. The model-turn failure body carries precise FastGate
meaning. A common misconception is that the status replaces the protocol failure. It does not. For
example, `invalid_request` and `unsupported_capability` preserve different model-turn meanings even
when a client treats both as rejected work.

### Transport validation is not JSON validation

A **provider port** is the small FastGate-owned interface the executor calls for upstream work. A
**provider adapter** is a concrete implementation that translates that interface to one provider;
the current strict fake is a test implementation of the port, not a live adapter. Later **Server-Sent
Events (SSE)** can send response fragments incrementally over HTTP. **Backpressure** is the rule that
keeps a slower client from causing unbounded buffering or producer work. ICGT-010 uses neither SSE nor
backpressure because it presents one already complete, bounded response.

These problems belong to different layers:

| Request problem | Owner | Handler/executor body read? | Provider work? |
| --- | --- | ---: | ---: |
| Wrong target or query | Planned HTTP handler | No | No |
| Wrong method | Planned HTTP handler | No | No |
| Any content encoding | Planned HTTP handler | No | No |
| Missing or unsupported media type | Planned HTTP handler | No | No |
| Malformed or schema-invalid JSON | Existing model-turn executor | Yes, within ICGT-009's bound | No |
| Supported request | Existing executor and provider port | Yes | Exactly once |

A `415 Unsupported Media Type` does not mean the JSON was malformed. It means the HTTP layer refused
the representation before asking model-turn admission to interpret it.

The `No` entries describe calls made by the handler and executor. Go's HTTP server may already have
buffered bytes or may drain unread bytes while closing a request. A recording body in a direct
handler test proves application ownership; it cannot prove what happened on the network.

### A handler is not automatically a runnable endpoint

A Go `http.Handler` can be exercised directly or through `httptest.NewServer` without being
registered in a production command. ICGT-010 plans exactly that: an independently testable handler.

The current [`service.New`](../../gateway/internal/service/service.go) still installs only its health
handler, and the current [`main`](../../gateway/cmd/fastgate/main.go) constructs that service. ICGT-010
does not change either path. Therefore `go run ./gateway/cmd/fastgate` remains a health-only process
during and after this unit.

### The server owns request-body closure

The existing executor consumes but deliberately does not close its `io.Reader`. For server requests,
Go's `http.Server` owns closing `Request.Body` after the handler returns. The planned handler passes
`request.Body` directly to the executor and does not add a second close owner.

This distinction matters in tests. A direct `handler.ServeHTTP` call does not reproduce every
resource action performed by `http.Server`. A direct handler test can prove that a transport
rejection does not read the body; it must not claim to prove server-owned closure.

## Key concepts

### One constructor creates the boundary

The planned package exports one constructor:

```go
NewHandler(*modelturn.Executor) (http.Handler, error)
```

This signature keeps the transport attached to the concrete, already reviewed executor instead of
inventing a second execution interface. A nil executor is a caller-programming error and returns the
exact safe error `model-turn HTTP executor is required`. The constructor does not mount a route,
create a server, or select a provider.

### Exact target matching comes first

The accepted target has one spelling:

```text
/v1/model-turns
```

It also has no query marker or query value. These are different targets and will be rejected:

```text
/v1/model-turns/
/v1/model-turns?debug=true
/v1/model-turns?
/v1/%6dodel-turns
http://proxy.example/v1/model-turns
```

A decoded path that looks equivalent is not accepted as another spelling. ICGT-010 adds no aliases,
redirects, trailing-slash normalization, query options, or compatibility paths. Target validation
runs first, so a different target receives the fixed `404` even when its method or media type is also
wrong. The handler requires the raw `RequestURI` plus its parsed URL fields to describe only the exact
origin-form target. Rejecting an absolute-form target does not select a Host-header policy; ICGT-011
still owns runtime Host and listener decisions.

### Method precedes representation validation

After the exact target is accepted, the method must be `POST`. Any other method receives:

- status `405 Method Not Allowed`;
- `Allow: POST`; and
- exact plain text `method not allowed\n`.

The handler does not read the body or call the executor on this path.

`HEAD` has one extra HTTP rule: a server does not send response body bytes for a HEAD request. The
handler still selects `405`, sets `Allow: POST`, and supplies `method not allowed\n` to `net/http`.
Direct handler evidence observes that one write attempt; a real HTTP client observes the status and
headers with an empty body because the server suppresses it.

### Encoded request bodies are unsupported

Any `Content-Encoding` field is rejected with `415 Unsupported Media Type`, including an empty field
or `Content-Encoding: identity`. This first handler does not decompress gzip, Brotli, or another
representation.

Rejecting encoding before the handler reads prevents compressed bytes from reaching the executor and
bypassing assumptions about which representation the executor bounds. A future compression feature
would need its own decompression limits and review.

### The accepted media type is deliberately small

There must be exactly one `Content-Type` field. It must parse as `application/json` with either:

- no parameters; or
- exactly one `charset=utf-8` parameter.

Media-type and parameter names and the UTF-8 charset value are compared case-insensitively. Missing,
multiple, malformed, or other media types and parameters receive `415` before body admission.
Examples include `text/json`, `application/json; profile=v1`, and repeated `Content-Type` fields.

`mime.ParseMediaType` normalizes more spellings than this endpoint accepts: it can collapse repeated
identical parameters and decode RFC 2231 names such as `charset*`. A small lexical precheck therefore
requires either no semicolon or exactly one semicolon with raw parameter name `charset`, without a
star, continuation, or second parameter; optional whitespace around the raw name is trimmed first.
`ParseMediaType` then validates the syntax and normalized UTF-8 value. Focused tests lock
duplicate-identical and RFC 2231 rejection so this check does not quietly broaden.

The optional charset is a transport spelling accepted for ordinary client interoperability. It does
not change the normative model-turn JSON parse profile.

### Request bounds remain owned by ICGT-009

The handler passes `request.Body` directly to
[`Executor.Execute`](../../gateway/internal/modelturn/executor.go). It does not first call
`io.ReadAll`, allocate another request copy, or add a second independently maintained byte limit.

The current executor already:

- retains at most 8 MiB plus one overflow byte;
- rejects malformed and ambiguous JSON;
- recovers a request ID only after the complete document is safe;
- rejects unsupported capabilities and aliases before dispatch; and
- invokes its injected provider at most once.

ICGT-010 must not duplicate those checks merely because HTTP-specific helpers exist.

### Closed outcomes make presentation exhaustive

The existing [`Outcome`](../../gateway/internal/modelturn/outcome.go) exposes:

```go
RequestID() (string, bool)
FailureBody() ([]byte, bool)
FailureCode() (string, bool)
ProviderOutcome() (provider.Result, error, bool)
```

Outside callers cannot construct a valid mixed state. A valid outcome represents one of:

1. an uncorrelated admission failure;
2. a correlated admission failure;
3. a correlated internal failure; or
4. an admitted provider result, normalized failure, or matching caller-context termination.

The planned handler classifies these alternatives through the accessors. It does not parse an
existing failure body to rediscover its code, and it does not reinterpret a provider error after the
provider contract has validated it.

### Every written response has shared safety headers

Every handler-authored response includes:

```text
Cache-Control: no-store
X-Content-Type-Options: nosniff
```

A `405` also includes `Allow: POST`. The handler never fabricates `Retry-After`. Provider
`retryable` is an observation in a model-turn failure body, not a delay or permission to retry.

### Transport failures use fixed plain text

| Condition | Status | Body | Media type |
| --- | ---: | --- | --- |
| Wrong path, encoded alias, trailing slash, or query | `404` | `not found\n` | `text/plain; charset=utf-8` |
| Exact target with wrong method | `405` | `method not allowed\n` | `text/plain; charset=utf-8` |
| Any content encoding or invalid media profile | `415` | `unsupported media type\n` | `text/plain; charset=utf-8` |
| Ordinary executor error or invalid returned outcome/accessor state | `500` | `internal server error\n` | `text/plain; charset=utf-8` |

These bodies do not pretend to be `model_turn.failed` documents because the handler has not admitted
a safe request ID.

Every JSON response is compact and has no appended line feed. The plain-text bodies above retain
their named final line feed. Tests must check that byte-level framing in addition to decoding JSON.

### Admission outcomes retain their existing bodies

ICGT-009 already owns these bodies and messages:

| Outcome | Status | Media type | Body owner |
| --- | ---: | --- | --- |
| Uncorrelated `invalid request\n` | `400` | `text/plain; charset=utf-8` | ICGT-009 |
| Correlated `invalid_request` | `400` | `application/json` | ICGT-009 |
| Correlated `unsupported_capability` | `422` | `application/json` | ICGT-009 |
| Correlated `internal_error` | `500` | `application/json` | ICGT-009 |

The handler writes the copied failure bytes returned by `Outcome.FailureBody`. It does not rebuild
those envelopes or replace their fixed messages.

### Completed results become model-turn JSON

A valid provider result becomes a strict `model_turn.completed` document containing the version,
kind, exact admitted request ID, complete validated output text, and optional usage.

Usage absence stays absent. Observed zero becomes a present object with both counters equal to zero.
Those states must not collapse. The handler uses `encoding/json`, not string concatenation, because
output may contain quotes, backslashes, controls, or multibyte Unicode that require escaping.

### Provider failures preserve evidence and gain fixed messages

A direct validated `*provider.Failure` becomes a strict `model_turn.failed` document. The handler
copies the provider-owned code, `retryable`, and optional usage unchanged, then adds only this fixed
message and status:

| Provider code | Status | Fixed message |
| --- | ---: | --- |
| `authentication_failed` | `502` | `FastGate could not authenticate to the upstream.` |
| `rate_limited` | `429` | `The upstream rate limit was reached.` |
| `request_rejected` | `422` | `The upstream rejected the request.` |
| `unavailable` | `503` | `The upstream is unavailable.` |
| `invalid_response` | `502` | `The upstream returned an invalid response.` |
| `unsupported_upstream_output` | `502` | `The upstream returned output that model-turn v1 does not support.` |
| `internal_error` | `500` | `The request could not be processed.` |

`authentication_failed` describes FastGate's upstream credential failure. It does not mean caller
authentication failed. The handler neither retries nor adds a `Retry-After` header.
It recognizes a normalized failure by direct `*provider.Failure` assertion; it does not use
`errors.As`, unwrap, or format the already validated alternative.

### Caller termination does not become a failure envelope

The provider contract permits exact `context.Canceled` or `context.DeadlineExceeded` only when the
same value is already reported by the supplied caller context. Simply returning would make
`net/http` synthesize an empty `200`, which would be a false response. For that outcome, the planned
handler directly compares the sentinel and panics with the standard `http.ErrAbortHandler` before any
application response write. Go's server treats that exact sentinel as an intentional response abort
and suppresses a panic stack trace.

That does not claim a client is guaranteed to observe no transport bytes. Go's HTTP server and the
connection own transport behavior after the request context ends. A direct test recovers the exact
sentinel with zero writer calls. A separate real-server regression supplies a deliberately canceled
server-side child context and uses a counting termination invoker to prove the executor/provider path
was reached once before the client observes no HTTP response; this prevents a request canceled before
dispatch from passing the test vacuously. ICGT-010 proves only that the handler does not invent a
completion, model-turn failure, status, or retry. Cancellation acknowledgement, races, cleanup, and
remote termination evidence remain outside this unit.

### Marshal before committing the status

A handler cannot reliably replace an HTTP status after writing headers or body bytes. The planned
implementation builds the complete bounded response body before `WriteHeader`.

If serialization unexpectedly fails, the handler can still choose fixed plain-text `500` because no
response is committed. If `ResponseWriter.Write` fails after commitment, the handler returns. It does
not write a second terminal body, invoke the provider again, or retry the response.

## Architecture and invariants

ICGT-010 preserves these ownership rules:

- `modelturnhttp` owns HTTP target, method, representation, headers, statuses, and serialization.
- `modelturn` continues to own body bounds, strict JSON, v1 admission, correlation, capability and
  alias policy, mapping, and one invocation.
- `provider` continues to own provider-neutral result and normalized failure meaning.
- The request context and body pass directly to `Executor.Execute`.
- `http.Server`, not the handler or executor, owns server request-body closure.
- Every response is bounded and prepared before status commitment.
- A write failure cannot authorize another response or provider invocation.
- Matching caller-context termination panics only with `http.ErrAbortHandler`, causing no application
  response write or implicit server `200`.
- Future runtime middleware must let that sentinel reach `net/http` rather than recover it into an
  ordinary response.
- The strict fake remains single-owner, unchanged, fresh per test, and serially used.
- The default service and command register no inference route.
- ICGT-011 owns actual listener enforcement, runtime provider assembly, and concurrency policy.

This story does not change Code Assist Harness. FastGate owns server-side protocol presentation. A
future CAH-owned `FastGateProvider` remains responsible for trusted endpoint selection and client-side
mapping.

## Practical walkthrough

Implementation should proceed through four small review pauses.

### Checkpoint 1: Exact transport preflight

Implement the exact target, empty-query, method, content-encoding, and media-type checks with fixed
headers, statuses, and bodies. Before continuing, tests should prove precedence, `Allow: POST`, the
two accepted JSON media forms, and zero handler/executor body reads and provider calls for every
rejection.

### Checkpoint 2: Existing admission outcomes

Pass the accepted request context and body directly to the executor. Map uncorrelated, correlated
admission, and correlated internal failures through their existing body and code accessors. Prove the
canonical `tool_calls` request remains a zero-call `422` at the HTTP boundary.

### Checkpoint 3: Provider result and failure presentation

Marshal completed results and all seven provider failure codes. Preserve request ID, output,
retryability, and usage absence versus observed zero. Compare representative values semantically with
the committed completed and unsupported-upstream-output fixtures.

### Checkpoint 4: Terminal transport behavior

Prove matching cancellation/deadline outcomes produce only `http.ErrAbortHandler`, make zero writes,
and do not become an implicit `200` through a real server. Also prove serialization finishes before
status commitment, a failing writer receives no second write, and one proxy-disabled real-loopback
request uses a timeout, bounded response read, explicit response-body closure, idle-connection
cleanup, and one strict-fake exchange. Keep the command and service unchanged.

## Personal code review map

| Review path | Current or planned | Why it matters | Question to answer |
| --- | --- | --- | --- |
| [`modelturn.Executor`](../../gateway/internal/modelturn/executor.go) | Current | Owns body/semantic admission and the only provider call | Where may provider work first begin? |
| [`modelturn.Outcome`](../../gateway/internal/modelturn/outcome.go) | Current | Supplies the closed alternatives to map | How can presentation distinguish failures without parsing JSON? |
| [`provider.Result` and `provider.Failure`](../../gateway/internal/provider/provider.go) | Current | Own output, code, retryability, and usage presence | Which observations must cross unchanged? |
| [`provider/fake`](../../gateway/internal/provider/fake/fake.go) | Current | Proves exact serial interactions | Why does a fresh fake per test not prove runtime concurrency safety? |
| [`service.New`](../../gateway/internal/service/service.go) and [`main`](../../gateway/cmd/fastgate/main.go) | Current | Show the command remains health-only | What does ICGT-010 deliberately leave unwired? |
| [Completed schema](../../gateway/contracts/model-turn/v1/schema/success.schema.json) and [failure schema](../../gateway/contracts/model-turn/v1/schema/failure.schema.json) | Current contract | Define response shape | How is optional usage represented? |
| Planned `modelturnhttp.NewHandler` | Planned; no file exists | Will reject a nil executor and own HTTP preflight/presentation | Does one accepted request lead to one prepared response? |
| Planned `modelturnhttp` tests | Planned; no file exists | Must prove zero handler-read, zero-dispatch, exhaustive mapping, and write behavior | Which test proves precedence and usage absence versus zero? |

## Implementation code samples

The following examples are **PSEUDOCODE ONLY**. They are not current Go APIs and must be replaced by
focused exact excerpts after implementation.

### Transport checks before body admission

```text
PSEUDOCODE ONLY

set no-store and nosniff headers

if raw and parsed target are not exactly origin-form /v1/model-turns:
    write fixed 404 plain text
    return

if method is not POST:
    set Allow to POST
    write fixed 405 plain text
    return

if Content-Encoding exists or Content-Type is not one approved JSON form:
    write fixed 415 plain text
    return

outcome, ordinary_error = executor.Execute(request.Context(), request.Body)
present(outcome, ordinary_error)
```

The body is untouched until transport checks pass. The executor remains the only request-byte and
JSON-admission owner.

### Classify the closed outcome

```text
PSEUDOCODE ONLY

if ordinary_error exists:
    prepare fixed plain-text 500
else if failure body exists:
    choose status from optional admitted failure code
    use the existing copied failure body
else if provider outcome exists:
    if completed result:
        marshal completed envelope
    else if direct provider failure:
        marshal failed envelope with fixed message
    else if matching caller-context termination:
        panic with exact http.ErrAbortHandler before any write
    else:
        prepare fixed plain-text 500
else:
    prepare fixed plain-text 500
```

The implementation must remain explicit. It should not use reflection, body parsing, or a generic
response framework.

### Commit one prepared response

```text
PSEUDOCODE ONLY

body, marshal_error = build_complete_bounded_body()
if marshal_error:
    body = fixed internal-server-error text
    status = 500
    media_type = text/plain

set Content-Type
write status once
write body once

if body write fails:
    return
```

There is no alternate terminal write, provider reinvocation, response retry, or error logging.

## Failure scenarios to study

| Failure | Responsible boundary | Planned safe result | Deterministic evidence |
| --- | --- | --- | --- |
| Encoded path, trailing slash, or query | HTTP target admission | Fixed `404`; no read or call | Recording body plus empty fake |
| Wrong method on exact target | HTTP method admission | Fixed `405` and `Allow: POST`; no read or call | Header and zero-read assertions |
| Missing, repeated, malformed, or unsupported media type | HTTP representation admission | Fixed `415`; no read or call | Table-driven media matrix |
| Any `Content-Encoding` | HTTP representation admission | Fixed `415`; no decompression or read | Encoding table and empty fake |
| Malformed body without safe ID | Existing executor | Existing `invalid request\n` with `400` | Exact body and status |
| Valid ID with invalid request shape | Existing executor | Existing `invalid_request` with `400` | Exact body and zero calls |
| `tool_calls` plus unknown alias | Existing executor | `unsupported_capability` with `422`; capability wins | Empty fake verification |
| Completed result with absent usage | HTTP presentation | `200` JSON omits usage | Exact field inventory |
| Completed result with observed-zero usage | HTTP presentation | `200` JSON includes zero counters | Absence-versus-zero pair |
| Provider failure with usage | HTTP presentation | Fixed status/message; exact code, retryability, and usage | Every-code table |
| Matching canceled or expired context | Request control flow | Exact response abort; no application write or implicit `200` | Counting writer plus reached-once real-server path |
| Ordinary executor error or invalid returned outcome/accessor state | HTTP safety fallback | Fixed plain-text `500` | Safe fallback test |
| Body write failure after commitment | HTTP writer | Return after one write; no retry | Failing writer and call counts |
| Concurrent strict-fake use | Outside ICGT-010 | Not attempted | Explicit deferral and race gate |

## Planned test and validation evidence

The implementation should add focused evidence for:

- the exact `NewHandler` signature and nil-executor error;
- exact target and trailing, encoded, query, forced-query, absolute-form, authority, and opaque variants;
- method/target/media precedence;
- all accepted and rejected content-type forms, including duplicate-identical and RFC 2231
  parameters, and every content-encoding field;
- zero handler/executor body reads and zero fake calls for transport rejection without claiming zero
  server buffering or draining;
- representative uncorrelated, invalid, unsupported-capability, alias, and admitted requests;
- completed result usage absent, observed zero, and nonzero;
- output requiring JSON escaping and exact request-ID echo;
- all provider failure codes, retryability values, and usage-presence states;
- semantic parity with committed v1 fixtures plus compact JSON with no trailing line feed;
- matching cancellation and deadline with exact `http.ErrAbortHandler` and zero application writes,
  plus a real-server case whose server-side canceled child context and counting termination invoker
  prove one dispatch before the client receives no response instead of an implicit `200`;
- direct and real-server HEAD behavior;
- a writer failure with no second write or redispatch; and
- one serial `httptest.NewServer` request using a proxy-disabled client and request timeout, a bounded
  response read, explicit response-body closure, idle-connection cleanup, and then `VerifyComplete`.

After implementation, run focused and repeated tests, applicable `go vet` and race checks, the
[PR review regression checklist](../pr-review-checklist.md), an independent adversarial review, and
`./scripts/check`. These are planned commands, not current validation evidence.

## Speed and size implications

ICGT-010 adds no provider call, retry, timer, queue, or background goroutine beyond the one invocation
already owned by ICGT-009.

The handler will marshal one complete non-streaming provider outcome before committing a status. That
creates one response-sized byte slice proportional to an already bounded result. It also means the
first response byte cannot be written until the complete result is available and serialized. This is
expected for the current non-streaming contract; later streaming work owns incremental time to first
byte.

The handler creates no second whole-request buffer. The existing 8 MiB-plus-one admission bound
remains unchanged. This planned unit makes no measured latency, throughput, allocation, or exact
serialized-size claim before implementation evidence exists.

## What changed during implementation

No runtime implementation exists yet, so there are no observed implementation changes to report.

Implementation review must revisit whether raw-path checks reject every encoded alias, repeated
headers remain unambiguous, all provider failure and usage-presence states are covered, every JSON
body is prepared before status commitment, matching termination causes zero writes, and no test uses
the single-owner fake concurrently.

After code exists, replace this section with actual failed assumptions, review discoveries, design
changes, and validation. Do not mark this lesson verified while this section remains speculative.

## Production expansion

### Example production scenario

A reusable local or deployed FastGate process needs more than an independently tested handler. It
needs an explicit runtime provider, listener admission, concurrency bounds, authentication policy for
non-loopback use, cancellation and cleanup semantics, operational telemetry, and a release profile
clients can trust.

ICGT-011 owns the next runtime assembly: actual loopback-listener enforcement, provider wiring, and
concurrency policy. It must not infer that a handler tested once with a strict fake is safe for
concurrent serving.

### Representative capabilities and tools

- Go [`net/http`](https://pkg.go.dev/net/http) supplies handlers, requests, response writers, and the
  server request-body lifecycle.
- Go [`http.ErrAbortHandler`](https://pkg.go.dev/net/http#ErrAbortHandler) is the exact sentinel used
  to abort a terminated response without ordinary panic-stack logging.
- Go [`mime.ParseMediaType`](https://pkg.go.dev/mime#ParseMediaType) parses media types and parameters
  without ad hoc splitting.
- Go [`encoding/json`](https://pkg.go.dev/encoding/json) escapes completed text and constructs
  bounded response documents.
- Go [`net/http/httptest`](https://pkg.go.dev/net/http/httptest) supports direct handler tests and one
  local loopback integration exchange.
- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110) defines HTTP semantics; the local endpoint still
  intentionally selects a narrower profile.

These are standard-library or specification references, not new framework dependencies.

### Local versus production

| Dimension | ICGT-010 | Production expansion |
| --- | --- | --- |
| Exposure | Independently tested handler only | Explicitly registered and operated route |
| Provider | Fresh serial strict fake in tests | Runtime-selected concurrency-safe adapter |
| Network boundary | One `httptest` loopback exchange | Enforced listener, TLS, and authentication policy |
| Concurrency | Deliberately unclaimed | Bounded active requests and owned rejection policy |
| Response mode | Fully buffered non-streaming JSON | Versioned SSE with cancellation and backpressure |
| Failures | Fixed safe presentation | Operational correlation, metrics, and incident evidence |
| Retries | None | Later narrowly reviewed provider/network policy |
| Client integration | No CAH adapter | Separately owned and pinned CAH `FastGateProvider` |

### Trade-offs and graduation signals

The planned handler stays small because it trusts already validated internal values and does not own
runtime concurrency. This makes HTTP mapping personally reviewable and prevents the strict fake from
quietly becoming a production server component.

A runtime endpoint becomes justified only when the next story can name the provider instance used by
the process, how simultaneous requests are bounded or rejected, which listener addresses are
accepted, who owns shutdown and in-flight work, and how tests prove those rules without live
credentials. Authentication and TLS become mandatory before any reviewed non-loopback profile.

SSE would improve time to first visible output by sending validated fragments before the whole model
turn completes. Its cost is a versioned event grammar plus explicit cancellation, cleanup, and
slow-client backpressure. FastGate should graduate to it only after the ICGT-012 through ICGT-017
sequence can prove those behaviors deterministically and a workload benefits from incremental output.

## Practical exercises

- Predict the response for `GET /v1/model-turns?debug=true` and explain which check wins.
- Compare a missing `Content-Type` with malformed JSON: identify which layer rejects each and whether
  the body is read.
- Write the expected completed JSON for absent usage, then for present zero usage.
- Trace `authentication_failed` from `provider.Failure` to its fixed message and explain why the
  result is not a caller `401`.
- Simulate a writer that fails after `WriteHeader` and explain why another response or provider call
  would be unsafe.
- Compare a handler tested through `httptest.NewServer` with one registered by the FastGate command.

## Key takeaways

- HTTP target and representation checks happen before model-turn body admission.
- ICGT-009 remains the only request-byte, strict-JSON, semantic-admission, and dispatch owner.
- A closed outcome lets HTTP presentation remain exhaustive without parsing failure bodies.
- Provider code, retryability, and usage cross unchanged; only fixed messages and statuses are added.
- Usage absence and observed zero are different public observations.
- Caller-context termination uses `http.ErrAbortHandler` so a no-write return cannot become an
  invented empty `200`.
- Marshal-before-commit preserves one-terminal-response behavior.
- A failed response write never authorizes provider or response retry.
- Go's HTTP server owns server request-body closure.
- An independently tested handler is not yet a runnable inference endpoint.
- The Code Assist Harness adapter remains outside this repository.

## Glossary

- **HTTP target:** The path and query components used to identify the requested resource.
- **Raw path spelling:** The encoded path form retained by a parsed HTTP request.
- **Media type:** The declared format of a body, such as `application/json`.
- **Content encoding:** A transformation such as gzip applied to representation bytes.
- **Transport rejection:** A failure decided before model-turn body admission.
- **Protocol failure:** A model-turn outcome carrying FastGate's versioned failure meaning.
- **Presentation mapping:** Translation from a validated outcome to status, headers, and body.
- **Response commitment:** The point after which the status cannot reliably be replaced.
- **Usage absence:** Evidence that the provider did not report usage.
- **Observed zero:** A present usage observation whose counters are zero.
- **Application response write:** A call by this handler to write status or body, distinct from
  transport cleanup performed by `net/http`.
- **Response abort:** The exact `http.ErrAbortHandler` sentinel panic that asks Go's server to stop a
  response without logging an ordinary panic stack.
- **Runtime assembly:** Construction of the listener, handler, provider, and concurrency policy used
  by an executable process.
- **Provider port:** The small provider-neutral invocation interface FastGate owns.
- **Provider adapter:** A concrete translator from the provider port to one upstream provider.
- **Server-Sent Events (SSE):** An HTTP response format that carries a sequence of server-to-client
  events incrementally.
- **Backpressure:** A bound or flow-control rule that prevents a slow consumer from causing unlimited
  buffered data or producer work.

See the shared [glossary](../glossary.md) for repository-wide terms.

## Teach-back questions

1. Why must ICGT-010 reject the wrong target, encoding, or media type before passing the body to `Executor.Execute`?
2. How does the handler preserve provider failure meaning and usage absence versus observed zero without parsing or reinterpreting the existing outcome?
3. Why does a successful `httptest` loopback exchange not mean the default FastGate command is ready for concurrent inference traffic?

## Further reading

- [ICGT-010 delivery contract](../../user-stories/icgt-010-present-model-turn-over-http.md)
- [ICGT-009 delivery contract](../../user-stories/icgt-009-admit-and-execute-model-turn.md)
- [ICGT-009 bounded-admission lesson](icgt-009-bounded-model-turn-admission.md)
- [ADR 0002: Fake first, OpenAI first live](../adr/0002-fake-first-openai-first-live.md)
- [ADR 0003: FastGate-owned model-turn protocol](../adr/0003-fastgate-api-surface.md)
- [FastGate model-turn v1 contract](../../gateway/contracts/model-turn/v1/README.md)
- [Provider contracts](../../gateway/internal/provider/provider.go)
- [Deterministic fake](../../gateway/internal/provider/fake/fake.go)
- [Go `net/http`](https://pkg.go.dev/net/http)
- [Go `http.ErrAbortHandler`](https://pkg.go.dev/net/http#ErrAbortHandler)
- [Go `mime`](https://pkg.go.dev/mime)
- [Go `encoding/json`](https://pkg.go.dev/encoding/json)
- [Go `httptest`](https://pkg.go.dev/net/http/httptest)
- [RFC 9110: HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110)
- [RFC 8259: JSON](https://www.rfc-editor.org/rfc/rfc8259)
