# ICGT-010 - Present model-turn v1 over HTTP

- **Status:** Done
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-009
- **Lesson:** [Model-turn HTTP presentation](../docs/lessons/icgt-010-model-turn-http-presentation.md)
- **Review priority:** High

## User story

> As a gateway developer, I want one strict HTTP handler around the existing model-turn executor so
> that a local client can observe every admitted non-streaming outcome with deterministic transport
> semantics, while transport preflight failures start no provider work and terminated requests do
> not become false responses.

## Primary concept and invariant

This story owns HTTP presentation, not listener or runtime composition:

> The handler checks the exact HTTP target, method, encoding, and media type before it asks the body
> to produce bytes.
> It then calls the executor once and converts every non-terminated closed model-turn outcome into one
> bounded HTTP response; a matching caller-context termination causes no application write. Every
> transport or admission rejection starts zero provider work.

ICGT-008's strict fake is finite and single-owner, while a normal HTTP server may invoke a handler
concurrently. ICGT-010 therefore uses a fresh fake serially in tests and proves one real loopback HTTP
exchange without mounting that fake in the runnable command. ICGT-011 separately owns service
binding, runtime-provider selection, concrete loopback `*net.TCPListener` enforcement, and bounded
concurrency.

The implementation is one handler/presenter package with 321 production lines: one line above the
review heuristic after adversarial review added declared-trailer protection. Splitting that transport
check from its handler would make the boundary harder to review. Stop for split review if the implementation begins owning listener lifecycle,
runtime-provider construction, concurrency control, authentication, or cancellation policy, or if it
needs more than the one exported constructor locked below.

## Scope

- Add the standard-library-only package `gateway/internal/modelturnhttp`.
- Expose one constructor, `NewHandler(*modelturn.Executor) (http.Handler, error)`.
- Recognize exactly `POST /v1/model-turns` inside that handler.
- Apply the exact transport checks and precedence below before body admission.
- Pass `request.Context()` and `request.Body` directly to `Executor.Execute`.
- Present every valid `modelturn.Outcome`: uncorrelated admission failure, correlated admission or
  internal failure, completed provider result, normalized provider failure, or matching context
  termination.
- Prove one real loopback HTTP exchange with one exact deterministic-fake script.
- Keep the default FastGate service and executable health-only.

These scope and constructor bullets describe ICGT-010 as delivered. The accepted ICGT-011 design
later supersedes the one-argument constructor with a required bounded-concurrency argument and adds
the handler's private permit state. Until ICGT-011 is implemented, the source and verified excerpts
remain exactly as recorded here; the later story must preserve this unit's transport and presentation
semantics while evolving that construction boundary.

## Locked behavior

### Small Go boundary

- `NewHandler` rejects a nil executor with exactly `model-turn HTTP executor is required`.
- As delivered by ICGT-010, the returned handler owns no mutable request state, goroutine, timer,
  retry, queue, or logger. That does not claim the injected executor and invoker are safe for
  concurrent calls. ICGT-011 is accepted to add only bounded permit state at this boundary.
- The handler uses only the public `modelturn.Outcome` and provider value accessors. It does not parse
  an existing failure body, inspect private outcome state, or accept provider SDK types.
- Private typed response structures and `encoding/json` construct completed and provider-failure
  documents. No generic response framework or duplicated public-domain model is introduced.

### Exact request target and preflight precedence

Checks occur in this order:

1. **Target:** require `RequestURI` to be exactly `/v1/model-turns`. Also require a nonnil `URL` with
   `Path` exactly `/v1/model-turns` and empty `RawPath`, `RawQuery`, `Scheme`, `Host`, and `Opaque`,
   plus `ForceQuery: false`. A different path, case, trailing slash, encoded alias, query, or
   absolute/authority/opaque-form target receives status `404` and exact body `not found\n`. This
   rejects proxy-form request targets without selecting a Host-header policy.
2. **Method:** require `POST`. Any other method receives status `405`, `Allow: POST`, and exact body
   `method not allowed\n`.
3. **Content encoding:** reject any `Content-Encoding` field in the initial header section, including
   an empty value or `identity`, and reject a `Content-Encoding` name declared in the request trailer
   inventory. Both receive status `415` and exact body `unsupported media type\n`.
4. **Content type:** require exactly one `Content-Type` field in the initial header section and reject
   a `Content-Type` name declared in the request trailer inventory. Before parsing, apply one small lexical
   occurrence check: allow no semicolon, or exactly one semicolon followed by one parameter whose raw
   name, after optional whitespace is trimmed, is `charset` case-insensitively, without `*`,
   continuations, or another semicolon. Then parse with `mime.ParseMediaType` and accept only
   `application/json` with either no parameters or the one normalized value `charset=utf-8`.
   Media-type, parameter-name, and UTF-8 charset comparisons are case-insensitive. Missing, repeated,
   malformed, duplicate-identical, RFC 2231 extended or continued, unsupported, or additionally
   parameterized values receive the same fixed `415`.
5. **Execution:** only after all transport checks pass, call
   `executor.Execute(request.Context(), request.Body)` once.

For every target, method, encoding, or media-type rejection, the handler makes zero
`Request.Body.Read` calls, never calls the executor, and invokes the provider zero times. Target
mismatch wins over method or representation errors; method mismatch wins over representation errors;
representation rejection wins over JSON admission. `Accept` negotiation, redirects, CORS, and
alternate routes do not exist in this profile.

[RFC 9110 section 6.5.1](https://www.rfc-editor.org/rfc/rfc9110#section-6.5.1) does not permit
content-format fields to be generated as late trailers. Go exposes declared
trailer names before the body is read, so the handler rejects those names during preflight. An
undeclared forbidden trailer can appear to Go's HTTP/1 parser only after body EOF; it is invalid
sender metadata and is never merged into or interpreted as representation metadata. The handler does
not perform a late post-execution `415`, because provider work might already have happened and that
response would violate the zero-work transport-rejection invariant. Valid chunked requests without
forbidden trailer declarations remain accepted.

### Body ownership and bounds

- The handler consumes the body only through `Executor.Execute`.
- It does not close or drain the body, inspect `Content-Length`, call `io.ReadAll` or `ParseForm`,
  decompress content, or add another body-limit wrapper.
- Go's `http.Server` owns closing a server request body after `ServeHTTP` returns. Direct handler tests
  do not claim to reproduce that server lifecycle.
- `http.Server` may buffer or drain unread request bytes while completing its own lifecycle. The
  zero-read claim applies to handler/executor calls, not bytes the transport may already hold or later
  consume.
- ICGT-009 remains the sole owner of the exact 8 MiB-plus-one raw-body mechanism.
- All uncorrelated admission failures, including oversized, malformed, read-failed, and unsafe-ID
  bodies, share the existing `400` presentation. ICGT-010 does not invent a second `413` policy.

### Common response rules

Every handler-authored response sets these headers before status commitment:

```text
Cache-Control: no-store
X-Content-Type-Options: nosniff
```

JSON responses use exactly `application/json`. Plain-text responses use exactly
`text/plain; charset=utf-8`. A `405` additionally sets `Allow: POST`. The handler adds no
`Retry-After`, `WWW-Authenticate`, CORS, redirect, cookie, or content-negotiation header.

Transport text bodies include their named final line feed. Model-turn JSON bodies are compact and
have no appended line feed. The complete bounded body is prepared before `WriteHeader`, followed by
at most one body-write call. A response-write failure causes no retry, replacement error body,
second provider call, alternate terminal result, or unsafe logging.

For a `HEAD` request, the handler still selects `405`, sets `Allow: POST`, and supplies the fixed
method body to `net/http`; the server suppresses those body bytes on the wire as required by HEAD
semantics. Direct-handler tests prove the handler's one attempted body write, while a real-server test
proves a client observes the status and headers with an empty response body.

### Transport, admission, and internal outcomes

| Outcome | Status | Media type | Exact body owner |
| --- | ---: | --- | --- |
| Wrong target | `404` | `text/plain; charset=utf-8` | ICGT-010: `not found\n` |
| Wrong method | `405` | `text/plain; charset=utf-8` | ICGT-010: `method not allowed\n` |
| Unsupported encoding or media type | `415` | `text/plain; charset=utf-8` | ICGT-010: `unsupported media type\n` |
| Uncorrelated admission failure | `400` | `text/plain; charset=utf-8` | Exact `Outcome.FailureBody()` |
| Correlated `invalid_request` | `400` | `application/json` | Exact `Outcome.FailureBody()` |
| Correlated `unsupported_capability` | `422` | `application/json` | Exact `Outcome.FailureBody()` |
| Correlated `internal_error` | `500` | `application/json` | Exact `Outcome.FailureBody()` |
| Ordinary executor error or invalid returned outcome/accessor state | `500` | `text/plain; charset=utf-8` | ICGT-010: `internal server error\n` |

Status selection for a correlated admission/internal failure uses `Outcome.FailureCode()`. The
handler does not parse or byte-match `Outcome.FailureBody()`. An ordinary executor error, an invalid
zero outcome, or another impossible accessor combination fails closed with the fixed plain-text
`500` without formatting the error.

### Completed provider outcome

A valid provider result receives status `200` and a strict `model_turn.completed` document with:

- `version: "v1"`;
- `kind: "model_turn.completed"`;
- the exact admitted `request_id`;
- the complete validated `output_text`; and
- `usage` only when the provider reported it.

Usage absence stays absent. Observed zero stays present with both counters set to zero. Output text is
neither normalized nor truncated; `encoding/json` preserves its decoded value while escaping JSON
syntax safely.

### Normalized provider failures

A direct validated `*provider.Failure` becomes a strict `model_turn.failed` document. It preserves
the exact admitted request ID, provider-owned code, `retryable` observation, and optional usage. The
handler supplies only the fixed presentation message and status below:

| Provider code | Status | Exact message |
| --- | ---: | --- |
| `authentication_failed` | `502` | `FastGate could not authenticate to the upstream.` |
| `rate_limited` | `429` | `The upstream rate limit was reached.` |
| `request_rejected` | `422` | `The upstream rejected the request.` |
| `unavailable` | `503` | `The upstream is unavailable.` |
| `invalid_response` | `502` | `The upstream returned an invalid response.` |
| `unsupported_upstream_output` | `502` | `The upstream returned output that model-turn v1 does not support.` |
| `internal_error` | `500` | `The request could not be processed.` |

The handler adds no retry. `retryable` remains an observation, not permission to repeat billable
work. Upstream `authentication_failed` is not mapped to `401`, because caller authentication is not
implemented. Classification uses a direct `*provider.Failure` type assertion; it does not call
`errors.As`, unwrap, or format the error.

### Context termination

A direct `context.Canceled` or `context.DeadlineExceeded` retained by
`Outcome.ProviderOutcome()` is control flow, not a provider failure. ICGT-009 has already validated
that it exactly matches the supplied request context. After direct sentinel comparison, the handler
panics with the exact standard `http.ErrAbortHandler` sentinel before attempting `WriteHeader` or
`Write`. Go's server then aborts the response and suppresses a panic stack trace instead of inventing
an implicit empty `200`. The handler does not invent `499`, `504`, `unavailable`, or `internal_error`,
and it does not use `errors.Is` or unwrap the sentinel.

This is not cancellation-conformance evidence. Disconnect acknowledgement, races, cleanup, and
confirmed versus unconfirmed upstream termination remain deferred to their owning stories. A result
that wins a future cancellation race is also outside ICGT-010; this story does not add a second
post-invocation context check that could discard a valid provider result. ICGT-011's runtime
composition must allow `http.ErrAbortHandler` to reach `net/http`; middleware must not recover it into
an ordinary response.

### Privacy and safety

The handler never logs, returns, or formats request content, instructions, aliases, rejected header
values, parser or reader errors, response-writer errors, raw provider errors, provider headers,
credentials, endpoints, bodies, or fake mismatch content. Only admitted output text, admitted request
ID, bounded usage, normalized failure fields, and fixed transport text may cross the response
boundary.

## Human-sized implementation checkpoints

1. **Transport preflight:** implement exact target, method, encoding, and media checks plus shared
   headers; review precedence and zero-read/zero-dispatch evidence.
2. **Existing failures:** pass context/body directly to the executor and map uncorrelated,
   correlated admission, and correlated internal outcomes without parsing their bodies.
3. **Provider presentation:** marshal completed results and every provider failure; review exact
   request ID, output escaping, retryability, and usage absence versus observed zero.
4. **Terminal transport behavior:** prove context termination makes zero writes, marshal occurs before
   status commitment, a write failure causes no retry, and one real loopback request consumes one
   fresh strict-fake exchange.

Each checkpoint must pass focused tests before the next begins. No checkpoint mounts the handler in
the default service or uses the strict fake concurrently.

## Acceptance criteria

1. Exact target, method, content-encoding, and content-type precedence is deterministic, including
   declared representation-trailer names. Every
   transport rejection makes zero handler body-read calls, never calls the executor, and invokes the
   provider zero times; the test does not claim that `http.Server` never buffers or drains bytes.
2. Constructor nil handling and every fixed transport response have exact bounded text, status,
   media type, shared safety headers, and any required `Allow` header without exposing input.
3. Uncorrelated, correlated admission, and correlated internal outcomes receive their exact status,
   media type, headers, and existing copied body. Status is selected through `FailureCode()` rather
   than body parsing.
4. Completed results preserve the exact request ID and decoded output while distinguishing absent,
   observed-zero, and nonzero usage.
5. All seven provider failure codes receive the locked status and fixed message while preserving
   exact retryability and absent, observed-zero, or nonzero usage.
6. Exact context cancellation and deadline outcomes panic only with `http.ErrAbortHandler`, cause
   zero application response-write attempts, and fabricate no protocol outcome. A real-server case
   uses a deliberately canceled server-side child context and a counting termination invoker to
   prove that the executor/provider path was reached once before the client receives no HTTP response,
   rather than passing vacuously because a request never reached the handler or observing an implicit
   `200`.
7. Every JSON body is prepared before status commitment. A failing writer sees no second body write,
   provider call, retry, or unsafe diagnostic.
8. Malformed, oversized, correlated-invalid, unknown-alias, and `tool_calls` requests retain
   ICGT-009 behavior. The combined capability/alias case proves capability precedence and zero
   provider calls at the HTTP boundary.
9. One serial `httptest.NewServer` exchange crosses HTTP, executor admission, one exact fake
   invocation, outcome presentation, client response-body cleanup, and final fake verification.
10. Existing lifecycle and `GET /healthz` tests remain unchanged, and the command still exposes no
    inference route.
11. The implementation adds no listener policy, Host trust, runtime provider, concurrency gate,
    authentication, TLS, proxy, redirect, retry, stream, timer, goroutine, queue, telemetry,
    external-network service, or live-provider dependency.
12. The completed lesson contains exact production/test links and excerpts, observed review changes,
    exercises, glossary terms, and exactly three teach-back questions; focused checks and
    `./scripts/check` pass.

## Deterministic validation

Minimum focused evidence includes:

- exact target plus trailing-slash, encoded-path, nonempty-query, forced-query, absolute-form,
  authority-bearing, and opaque variants;
- wrong target plus wrong method/media, and wrong method plus wrong media, to prove precedence;
- missing, repeated fields, malformed, duplicate-identical parameter, RFC 2231 extended/continued,
  unsupported, extra-parameter, wrong-charset, accepted bare JSON, and accepted sole UTF-8-charset
  media types;
- initial and declared-trailer `Content-Encoding`, including empty and `identity`, plus a declared
  trailer `Content-Type` and an accepted chunked request without forbidden trailer declarations;
- recording bodies and empty fakes proving transport rejection reads and dispatches zero times;
- exact uncorrelated and correlated admission responses;
- capability-before-alias rejection with an empty fake;
- completed output requiring JSON escaping and usage absent, observed zero, and nonzero;
- every provider failure code, both retryability values across the table, and failure usage absent,
  observed zero, and nonzero;
- semantic parity with the committed minimal-completed and unsupported-upstream-output fixtures;
- an invalid provider alternative becoming correlated `500` after one dispatch;
- canceled and deadline-expired request contexts proving an exact `http.ErrAbortHandler` panic and
  zero direct writes, plus a real-server case whose server-side canceled child context and counting
  termination invoker prove one dispatch before the client receives no response instead of an
  implicit `200`;
- direct and real-server `HEAD` cases proving handler-authored versus wire-suppressed body behavior;
- a failing response writer proving no second write or redispatch; and
- one serial loopback integration exchange followed by `VerifyComplete`.

Focused tests, 20 repeated deterministic runs, `go vet`, race tests, the PR review regression
checklist, an independent adversarial review, and `./scripts/check` all passed. The completed lesson
records the commands, environment constraint, and observed results.

## Human review checkpoint

- **Production path:** Trace [`handler.go`](../gateway/internal/modelturnhttp/handler.go) from exact
  target, method, representation, and declared-trailer admission through `Executor.Execute`, then
  trace [`presentation.go`](../gateway/internal/modelturnhttp/presentation.go) from one completed
  provider result through full JSON preparation, headers, status, and one body write.
- **Failure/test path:** Trace [`handler_test.go`](../gateway/internal/modelturnhttp/handler_test.go)
  for precedence and zero body reads, [`presentation_test.go`](../gateway/internal/modelturnhttp/presentation_test.go)
  for observed-zero usage, and [`server_test.go`](../gateway/internal/modelturnhttp/server_test.go)
  for exact context abort, declared trailer rejection, and loopback cleanup.
- **Invariant:** No transport or admission rejection starts provider work, and every admitted,
  non-terminated outcome becomes exactly one bounded response without reinterpreting provider
  observations or retrying work.
- **Deferred:** Runtime mounting, concrete loopback `*net.TCPListener` enforcement, Host policy, interactive demo
  provider, concurrent fake use, cancellation conformance, streaming, deadlines, cleanup,
  backpressure, authentication, TLS, retries, telemetry, and live providers.

## Documentation impact

- Complete the Markdown lesson linked above with exact source and test evidence; no visual companion
  is required.
- Update the story and lesson indexes, roadmap, architecture, root/FastGate status, model-turn
  contract, and prior handoffs without claiming runtime binding.
- Add an evidence-backed ICGT-010 note with review discoveries and validation.
- Keep Code Assist Harness unchanged. Its future FastGate client adapter remains a separately
  reviewed CAH-owned story.

## Out of scope

- Changing `gateway/cmd/fastgate`, `gateway/internal/service`, or mounting the inference handler in
  the default executable.
- Actual-listener loopback enforcement or Host-authority policy; ICGT-011 owns runtime binding.
- Making the strict deterministic fake concurrency-safe or using it interactively.
- Authentication, TLS, CORS, compression, proxying, redirects, request queues, rate policy, retries,
  idempotency, or content negotiation.
- SSE, partial output, cancellation acknowledgement or races, upstream deadlines, cleanup grace,
  slow-client backpressure, telemetry, or live-provider behavior.
- Changing Code Assist Harness types, workflow state, tools, approvals, transcripts, retries, or
  correctness evaluation.
- Creating a visual lesson.
