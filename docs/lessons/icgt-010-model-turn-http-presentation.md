# ICGT-010 lesson: Presenting model-turn v1 over HTTP

- **Unit:** ICGT-010
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Implemented and validated; no inference route is mounted
- **Story:** [ICGT-010](../../user-stories/icgt-010-present-model-turn-over-http.md)
- **Review priority:** High
- **Visual companion:** Not required; the Markdown lesson is the implementation learning artifact
- **Related architecture:** [ADR 0002](../adr/0002-fake-first-openai-first-live.md),
  [ADR 0003](../adr/0003-fastgate-api-surface.md),
  [model-turn v1](../../gateway/contracts/model-turn/v1/README.md), and
  [ICGT-009 bounded admission](../../user-stories/icgt-009-admit-and-execute-model-turn.md)

> This lesson links the exact implementation and focused validation delivered by ICGT-010. The
> default FastGate command still serves only `GET /healthz`; ICGT-010 deliberately does not register
> the handler with that command.

## Quick summary

ICGT-009 already accepts bounded request bytes, enforces the strict model-turn v1 contract, rejects
unsupported requests before provider work, and returns one closed `modelturn.Outcome`. It deliberately
does not decide HTTP paths, methods, media types, statuses, headers, or response serialization.

ICGT-010 implements the next layer: one small HTTP handler that presents those existing outcomes
without changing their meaning. The handler recognizes exactly `POST /v1/model-turns`, rejects
transport mistakes before it calls `Request.Body.Read`, passes the request context and body directly
to the existing executor, and maps each valid non-terminated closed outcome to one bounded HTTP
response. A matching caller-context termination instead aborts the response without inventing
protocol meaning.

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
| Wrong target or query | HTTP handler | No | No |
| Wrong method | HTTP handler | No | No |
| Any content encoding header or declared trailer | HTTP handler | No | No |
| Missing, repeated, unsupported, or trailer-declared media type | HTTP handler | No | No |
| Malformed or schema-invalid JSON | Existing model-turn executor | Yes, within ICGT-009's bound | No |
| Supported request | Existing executor and provider port | Yes | Exactly once |

A `415 Unsupported Media Type` does not mean the JSON was malformed. It means the HTTP layer refused
the representation before asking model-turn admission to interpret it.

The `No` entries describe calls made by the handler and executor. Go's HTTP server may already have
buffered bytes or may drain unread bytes while closing a request. A recording body in a direct
handler test proves application ownership; it cannot prove what happened on the network.

### A handler is not automatically a runnable endpoint

A Go `http.Handler` can be exercised directly or through `httptest.NewServer` without being
registered in a production command. ICGT-010 implements exactly that: an independently testable handler.

The current [`service.New`](../../gateway/internal/service/service.go) still installs only its health
handler, and the current [`main`](../../gateway/cmd/fastgate/main.go) constructs that service. ICGT-010
does not change either path. Therefore `go run ./gateway/cmd/fastgate` remains a health-only process
during and after this unit.

### The server owns request-body closure

The existing executor consumes but deliberately does not close its `io.Reader`. For server requests,
Go's `http.Server` owns closing `Request.Body` after the handler returns. The handler passes
`request.Body` directly to the executor and does not add a second close owner.

This distinction matters in tests. A direct `handler.ServeHTTP` call does not reproduce every
resource action performed by `http.Server`. A direct handler test can prove that a transport
rejection does not read the body; it must not claim to prove server-owned closure.

## Key concepts

### One constructor creates the boundary

The package exports one constructor:

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

It also has no query marker or query value. These different targets are rejected:

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

### Declared representation trailers are preflight input

HTTP trailers are fields declared before the body whose values arrive after the body. Go exposes the
declared keys in `Request.Trailer` when the handler begins, even though their values are not available
until the body reaches EOF. The handler therefore rejects a declared trailer named `Content-Encoding`
or `Content-Type` during preflight. This closes a representation-ambiguity path without reading the
body or starting provider work.

An undeclared content-format trailer is invalid under RFC 9110 because representation metadata needed
to process the content cannot arrive only after that content. This profile neither merges nor
interprets such invalid undeclared fields. Discovering one after EOF and changing the result to a late
`415` would require the HTTP layer to consume the body itself, breaking direct body ownership and the
rule that representation rejection occurs before provider work. ICGT-010 consequently owns declared
representation-trailer rejection, not repair of malformed HTTP messages after admission.

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

The handler classifies these alternatives through the accessors. It does not parse an
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

The response tests inspect `httptest.ResponseRecorder.Result().Header`, not the recorder's mutable
header map. `Result()` exposes the header snapshot captured when `WriteHeader` committed the response,
so those assertions prove the safety and media headers existed before commitment.

### Transport failures use fixed plain text

| Condition | Status | Body | Media type |
| --- | ---: | --- | --- |
| Wrong path, encoded alias, trailing slash, or query | `404` | `not found\n` | `text/plain; charset=utf-8` |
| Exact target with wrong method | `405` | `method not allowed\n` | `text/plain; charset=utf-8` |
| Any content encoding, declared representation trailer, or invalid media profile | `415` | `unsupported media type\n` | `text/plain; charset=utf-8` |
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
`net/http` synthesize an empty `200`, which would be a false response. For that outcome, the
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

A handler cannot reliably replace an HTTP status after writing headers or body bytes. The
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

Implementation completed the four approved review pauses. The final package has one exported API and
321 production lines across two files, [`handler.go`](../../gateway/internal/modelturnhttp/handler.go)
and [`presentation.go`](../../gateway/internal/modelturnhttp/presentation.go).

### Checkpoint 1: Exact transport preflight

The handler implemented exact raw-and-parsed target, method, content-encoding, declared-trailer, and
media-type checks. Table-driven tests proved precedence, reciprocal raw/parsed target disagreement,
`Allow: POST`, the two accepted JSON media forms, case-insensitive header inventory, and zero
handler/executor body reads and provider calls for every rejection.

### Checkpoint 2: Existing admission outcomes

Accepted requests pass their context and body directly to the executor. The implementation maps
uncorrelated, correlated admission, and correlated internal failures through the existing body and
code accessors. The canonical `tool_calls` plus unknown-alias case remains a zero-call `422` at the
HTTP boundary.

### Checkpoint 3: Provider result and failure presentation

Typed private structures marshal completed results and all seven provider failure codes. Tests
preserve request ID, escaped output, retryability, and usage absence versus observed zero, and compare
representative values semantically with the committed completed and unsupported-upstream-output
fixtures.

### Checkpoint 4: Terminal transport behavior

Direct tests prove matching cancellation and deadline outcomes produce only `http.ErrAbortHandler`,
touch no response writer, and reach the invoker once. A real-server cancellation test additionally
proves the result did not pass merely because the client timeout fired. Other tests prove headers are
present in the commit-time snapshot, a failing writer receives no second write, declared
representation trailers are rejected before dispatch, and one proxy-disabled serial loopback request
performs bounded response cleanup before final fake verification. The command and service remain
unchanged.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [`NewHandler`, `ServeHTTP`, and transport preflight](../../gateway/internal/modelturnhttp/handler.go) | Own the one exported boundary, exact check order, and final write | Where can the body first reach the executor? |
| [Outcome and JSON presentation](../../gateway/internal/modelturnhttp/presentation.go) | Exhaustively maps closed outcomes and preserves optional usage | Why is a nil usage pointer different from a pointer to zero values? |
| [Target, media, and zero-read tests](../../gateway/internal/modelturnhttp/handler_test.go) | Pin reciprocal raw/parsed checks, lexical media rules, declared trailers, and commit-time headers | Which assertion proves a transport rejection did not read or dispatch? |
| [Admission tests](../../gateway/internal/modelturnhttp/admission_test.go) | Prove ICGT-009 failures cross HTTP unchanged | Why does `tool_calls` still win over an unknown alias? |
| [Result, failure, and fixture tests](../../gateway/internal/modelturnhttp/presentation_test.go) | Pin all seven provider failures, usage presence, escaping, and compact framing | Which pair distinguishes absent usage from observed zero? |
| [Abort, writer, trailer, HEAD, and loopback tests](../../gateway/internal/modelturnhttp/server_test.go) | Exercise direct-writer semantics and real `net/http` behavior | Why are both direct and real-server tests necessary? |
| [`modelturn.Executor`](../../gateway/internal/modelturn/executor.go) and [`modelturn.Outcome`](../../gateway/internal/modelturn/outcome.go) | Continue to own admission, the only provider call, and closed alternatives | Which meaning is reused rather than duplicated by HTTP? |
| [`service.New`](../../gateway/internal/service/service.go) and [`main`](../../gateway/cmd/fastgate/main.go) | Show the executable remains health-only | What does ICGT-011 still need to assemble? |

## Implementation code samples

These focused excerpts are copied from the implementation. Read them as ownership boundaries rather
than as independent snippets to paste elsewhere.

### 1. Construct the handler and finish preflight before execution

[`NewHandler`, `ServeHTTP`, and `transportRejection`](../../gateway/internal/modelturnhttp/handler.go)
keep construction, preflight, execution, and writing in one visible order:

```go
func NewHandler(executor *modelturn.Executor) (http.Handler, error) {
	if executor == nil {
		return nil, errors.New("model-turn HTTP executor is required")
	}
	return modelTurnHandler{executor: executor}, nil
}

func (handler modelTurnHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	response, rejected := transportRejection(request)
	if !rejected {
		outcome, err := handler.executor.Execute(request.Context(), request.Body)
		response = prepareOutcome(outcome, err)
	}
	writePreparedResponse(responseWriter, response)
}

func transportRejection(request *http.Request) (preparedResponse, bool) {
	if !hasExactTarget(request) {
		return prepareTextResponse(http.StatusNotFound, notFoundBody, ""), true
	}
	if request.Method != http.MethodPost {
		return prepareTextResponse(http.StatusMethodNotAllowed, methodNotAllowedBody, http.MethodPost), true
	}
	_, hasEncoding := headerFieldValues(request.Header, "Content-Encoding")
	_, hasEncodingTrailer := headerFieldValues(request.Trailer, "Content-Encoding")
	if hasEncoding || hasEncodingTrailer {
		return prepareTextResponse(http.StatusUnsupportedMediaType, unsupportedMediaTypeBody, ""), true
	}
	contentTypes, _ := headerFieldValues(request.Header, "Content-Type")
	_, hasContentTypeTrailer := headerFieldValues(request.Trailer, "Content-Type")
	if hasContentTypeTrailer ||
		len(contentTypes) != 1 || !acceptsJSONMediaType(contentTypes[0]) {
		return prepareTextResponse(http.StatusUnsupportedMediaType, unsupportedMediaTypeBody, ""), true
	}
	return preparedResponse{}, false
}
```

The handler does not ask the writer for its header map until execution has produced one complete
`preparedResponse`. More importantly, target, method, ordinary headers, and already declared trailer
keys all reject before `Executor.Execute` can read the body or invoke a provider.

### 2. Require reciprocal raw and parsed target agreement

[`hasExactTarget`](../../gateway/internal/modelturnhttp/handler.go) accepts only one origin-form
spelling. Both representations must agree:

```go
func hasExactTarget(request *http.Request) bool {
	if request == nil || request.RequestURI != modelTurnPath || request.URL == nil {
		return false
	}
	return request.URL.Path == modelTurnPath &&
		request.URL.RawPath == "" &&
		request.URL.RawQuery == "" &&
		!request.URL.ForceQuery &&
		request.URL.Scheme == "" &&
		request.URL.Host == "" &&
		request.URL.Opaque == ""
}
```

This is stronger than checking only the decoded path. The target suite contains reciprocal cases:
an exact `RequestURI` with a different parsed path, and an exact parsed URL with a different
`RequestURI`. Either incomplete implementation fails one side of that pair.

### 3. Count fields lexically before normalizing the media type

[`headerFieldValues` and `acceptsJSONMediaType`](../../gateway/internal/modelturnhttp/handler.go) make
field presence, case-insensitive key matching, and the intentionally narrow parameter grammar
explicit:

```go
func headerFieldValues(header http.Header, name string) ([]string, bool) {
	var values []string
	present := false
	for fieldName, fieldValues := range header {
		if strings.EqualFold(fieldName, name) {
			present = true
			values = append(values, fieldValues...)
		}
	}
	return values, present
}

func acceptsJSONMediaType(value string) bool {
	semicolonCount := strings.Count(value, ";")
	if semicolonCount > 1 {
		return false
	}
	if semicolonCount == 1 {
		_, rawParameter, _ := strings.Cut(value, ";")
		rawName, _, hasValue := strings.Cut(rawParameter, "=")
		if !hasValue || !strings.EqualFold(strings.TrimSpace(rawName), "charset") {
			return false
		}
	}

	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || !strings.EqualFold(mediaType, jsonMediaType) {
		return false
	}
	if semicolonCount == 0 {
		return len(parameters) == 0
	}
	if len(parameters) != 1 {
		return false
	}
	for name, parameterValue := range parameters {
		return strings.EqualFold(name, "charset") && strings.EqualFold(parameterValue, "utf-8")
	}
	return false
}
```

The lexical pass prevents `mime.ParseMediaType` from broadening the endpoint through duplicate or
RFC 2231 parameters. The parser still owns syntax and normalized value validation.

### 4. Classify the closed outcome without reparsing it

[`prepareOutcome` and `prepareProviderOutcome`](../../gateway/internal/modelturnhttp/presentation.go)
read every public accessor once, validate the allowed combination, and treat matching termination as
control flow:

```go
func prepareOutcome(outcome modelturn.Outcome, executeErr error) preparedResponse {
	if executeErr != nil {
		return prepareInternalResponse()
	}

	requestID, hasRequestID := outcome.RequestID()
	failureBody, hasFailureBody := outcome.FailureBody()
	failureCode, hasFailureCode := outcome.FailureCode()
	result, providerErr, hasProviderOutcome := outcome.ProviderOutcome()

	switch {
	case hasFailureBody && !hasRequestID && !hasFailureCode && !hasProviderOutcome:
		return prepareBytesResponse(http.StatusBadRequest, textMediaType, failureBody)
	case hasFailureBody && hasRequestID && hasFailureCode && !hasProviderOutcome:
		status, ok := admissionFailureStatus(failureCode)
		if !ok || requestID == "" {
			return prepareInternalResponse()
		}
		return prepareBytesResponse(status, jsonMediaType, failureBody)
	case !hasFailureBody && hasRequestID && !hasFailureCode && hasProviderOutcome:
		if requestID == "" {
			return prepareInternalResponse()
		}
		return prepareProviderOutcome(requestID, result, providerErr)
	default:
		return prepareInternalResponse()
	}
}

func prepareProviderOutcome(requestID string, result provider.Result, providerErr error) preparedResponse {
	if providerErr == nil {
		return prepareCompletedResponse(requestID, result)
	}
	if providerErr == context.Canceled || providerErr == context.DeadlineExceeded {
		panic(http.ErrAbortHandler)
	}
	failure, ok := providerErr.(*provider.Failure)
	if !ok || failure == nil {
		return prepareInternalResponse()
	}
	return prepareProviderFailureResponse(requestID, failure)
}
```

The HTTP layer never parses `FailureBody` to rediscover its code and never unwraps or formats a
provider error. Because `prepareProviderOutcome` panics before returning a prepared response,
`ServeHTTP` never reaches `writePreparedResponse` for a matching caller termination.

### 5. Preserve usage absence with a typed pointer

The private response types in [`presentation.go`](../../gateway/internal/modelturnhttp/presentation.go)
use a pointer plus `omitempty`; [`prepareUsage`](../../gateway/internal/modelturnhttp/presentation.go)
creates the pointer even when both observed counters are zero:

```go
type usageResponse struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type completedResponse struct {
	Version    string         `json:"version"`
	Kind       string         `json:"kind"`
	RequestID  string         `json:"request_id"`
	OutputText string         `json:"output_text"`
	Usage      *usageResponse `json:"usage,omitempty"`
}

func prepareUsage(usage provider.Usage, present bool) *usageResponse {
	if !present {
		return nil
	}
	return &usageResponse{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}
}
```

`nil` means the provider supplied no usage observation. `&usageResponse{}` means it explicitly
reported zero. `encoding/json` omits only the first state.

### 6. Commit one already prepared response

[`writePreparedResponse`](../../gateway/internal/modelturnhttp/handler.go) is deliberately small:

```go
func writePreparedResponse(responseWriter http.ResponseWriter, response preparedResponse) {
	headers := responseWriter.Header()
	headers.Set("Cache-Control", "no-store")
	headers.Set("Content-Type", response.mediaType)
	headers.Set("X-Content-Type-Options", "nosniff")
	if response.allowMethod != "" {
		headers.Set("Allow", response.allowMethod)
	}
	responseWriter.WriteHeader(response.status)
	_, _ = responseWriter.Write(response.body)
}
```

Every serializer runs before this function. It obtains the header map once, commits status once, and
attempts one body write. There is no alternate terminal write, provider reinvocation, response retry,
or error formatting.

## Test excerpts: prove the boundary, not only the returned body

### Transport rejection reads and dispatches zero times

The shared assertion loop in
[`TestHandlerRejectsTransportBeforeBodyReadOrProviderDispatch`](../../gateway/internal/modelturnhttp/handler_test.go)
uses a recording body and a fresh empty fake:

```go
upstream := newTestFake(t)
handler := newTestHandler(t, upstream)
body := &recordingRequestBody{}
request := httptest.NewRequest(test.method, test.target, body)
request.Header = test.headers.Clone()
request.Trailer = test.trailers.Clone()
response := httptest.NewRecorder()

handler.ServeHTTP(response, request)

assertHTTPResponse(
	t,
	response,
	test.wantStatus,
	textMediaType,
	test.wantBody,
	test.wantAllow,
)
if body.reads != 0 {
	t.Fatalf("request body Read() calls = %d, want 0", body.reads)
}
if body.closes != 0 {
	t.Fatalf("request body Close() calls = %d, want server-owned closure", body.closes)
}
if err := upstream.VerifyComplete(); err != nil {
	t.Fatalf("zero-dispatch fake was not complete: %v", err)
}
```

The cases include target/method/media precedence, empty and noncanonical encoding fields, repeated
content types, lexical media traps, and declared representation trailer keys.

### Header assertions read the commit-time snapshot

[`assertHTTPMetadata`](../../gateway/internal/modelturnhttp/handler_test.go) calls `Result()` before it
checks headers:

```go
result := response.Result()
if result.StatusCode != wantStatus {
	t.Fatalf("status = %d, want %d", result.StatusCode, wantStatus)
}
if got := result.Header.Get("Content-Type"); got != wantMediaType {
	t.Fatalf("Content-Type = %q, want %q", got, wantMediaType)
}
if got := result.Header.Get("Cache-Control"); got != "no-store" {
	t.Fatalf("Cache-Control = %q, want no-store", got)
}
if got := result.Header.Get("X-Content-Type-Options"); got != "nosniff" {
	t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
}
if got := result.Header.Get("Allow"); got != wantAllow {
	t.Fatalf("Allow = %q, want %q", got, wantAllow)
}
```

`httptest.ResponseRecorder.Result().Header` is the snapshot captured at `WriteHeader`; reading
`response.Header()` instead would inspect the mutable map and would not prove header timing.

### Direct and real-server tests prove the abort path differently

The direct test in
[`TestHandlerAbortsMatchingContextTerminationBeforeWriterAccess`](../../gateway/internal/modelturnhttp/server_test.go)
recovers the exact sentinel and proves even `Header()` was untouched:

```go
panicValue, panicked := capturePanic(func() {
	handler.ServeHTTP(writer, request)
})

if !panicked || panicValue != http.ErrAbortHandler {
	t.Fatalf("ServeHTTP() panic = (%v, %t), want exact http.ErrAbortHandler", panicValue, panicked)
}
if invoker.calls.Load() != 1 {
	t.Fatalf("Invoke() calls = %d, want 1", invoker.calls.Load())
}
if writer.headerCalls != 0 || writer.writeHeaderCalls != 0 || writer.writeCalls != 0 {
	t.Fatalf(
		"writer calls = (Header %d, WriteHeader %d, Write %d), want all zero",
		writer.headerCalls,
		writer.writeHeaderCalls,
		writer.writeCalls,
	)
}
```

The real-server regression in
[`TestRealServerAbortReachesInvokerAndDoesNotBecomeImplicitOK`](../../gateway/internal/modelturnhttp/server_test.go)
also proves the client deadline did not manufacture the passing result:

```go
response, requestErr := client.Do(request)
transport.CloseIdleConnections()

if requestCtx.Err() != nil {
	t.Fatalf("request context ended before server abort was observed: %v", requestCtx.Err())
}
if requestErr == nil || response != nil {
	if response != nil {
		_ = response.Body.Close()
	}
	t.Fatalf("client.Do() = (response %v, error %v), want no HTTP response", response, requestErr)
}
if invoker.calls.Load() != 1 {
	t.Fatalf("Invoke() calls = %d, want reached exactly once", invoker.calls.Load())
}
```

The server wrapper cancels a child of the accepted request context before calling the real handler.
The exact-one counter prevents a vacuous pass where the request never reached execution; the nil
client-context error proves the observed failure came from the server abort rather than timeout.

### A real server exposes declared trailer keys before body admission

[`TestRealServerRejectsDeclaredRepresentationTrailersBeforeDispatch`](../../gateway/internal/modelturnhttp/server_test.go)
sends a chunked request whose trailer name is declared up front:

```go
request.Header.Set("Content-Type", jsonMediaType)
request.ContentLength = -1
request.Trailer = http.Header{test.trailerName: {"declared-value"}}

response, err := client.Do(request)
if err != nil {
	t.Fatalf("request with declared trailer returned an error: %v", err)
}
body := readAndCloseResponse(t, response)
transport.CloseIdleConnections()

assertServerResponseHeaders(
	t,
	response,
	http.StatusUnsupportedMediaType,
	textMediaType,
	"",
)
if got := string(body); got != unsupportedMediaTypeBody {
	t.Fatalf("body = %q, want %q", got, unsupportedMediaTypeBody)
}
if calls.Load() != 0 {
	t.Fatalf("Invoke() calls = %d, want 0", calls.Load())
}
```

Both `Content-Encoding` and a trailer-declared second `Content-Type` take this path. The test proves
real `net/http` request parsing exposes the declaration early enough for zero-dispatch preflight.

### The serial loopback test closes every resource before fake verification

The happy path in
[`TestSerialChunkedLoopbackModelTurnCleansUpAndCompletesFake`](../../gateway/internal/modelturnhttp/server_test.go)
uses an explicit timeout and proxy-free transport, then performs response cleanup before
`VerifyComplete`:

```go
client, transport := newLoopbackClient()

requestCtx, cancelRequest := context.WithTimeout(context.Background(), testClientTimeout)
defer cancelRequest()
request, err := http.NewRequestWithContext(
	requestCtx,
	http.MethodPost,
	server.URL+modelTurnPath,
	bytes.NewReader(marshalTestDocument(t, document)),
)
if err != nil {
	t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
}
request.Header.Set("Content-Type", jsonMediaType)
request.ContentLength = -1
response, err := client.Do(request)
if err != nil {
	t.Fatalf("model-turn request returned an error: %v", err)
}
body := readAndCloseResponse(t, response)
transport.CloseIdleConnections()

assertServerResponseHeaders(t, response, http.StatusOK, jsonMediaType, "")
assertJSONDocument(t, body, map[string]any{
	"version":     modelTurnVersion,
	"kind":        completedKind,
	"request_id":  document.RequestID,
	"output_text": result.OutputText(),
	"usage":       expectedUsage(*usage),
})
if err := upstream.VerifyComplete(); err != nil {
	t.Fatalf("VerifyComplete() after response cleanup returned an error: %v", err)
}
```

`newLoopbackClient` sets `Proxy: nil`. `readAndCloseResponse` reads through an
`io.LimitReader`, rejects more than 1 MiB, and closes the body before idle connections are closed.
The fresh strict fake is used serially; this test intentionally proves no concurrent runtime policy.

## Failure scenarios to study

| Failure | Responsible boundary | Observed safe result | Deterministic evidence |
| --- | --- | --- | --- |
| Encoded path, trailing slash, query, or raw/parsed disagreement | HTTP target admission | Fixed `404`; no read or call | [`TestExactTargetRequiresOneOriginFormSpelling`](../../gateway/internal/modelturnhttp/handler_test.go) and the transport matrix |
| Wrong method on the exact target | HTTP method admission | Fixed `405` and `Allow: POST`; no read or call | [`TestHandlerRejectsTransportBeforeBodyReadOrProviderDispatch`](../../gateway/internal/modelturnhttp/handler_test.go) |
| Missing, repeated, malformed, or unsupported media type | HTTP representation admission | Fixed `415`; no read or call | [`TestJSONMediaTypeProfile`](../../gateway/internal/modelturnhttp/handler_test.go) and the transport matrix |
| Encoding header or declared representation trailer | HTTP representation admission | Fixed `415`; no decompression, body read, or call | Direct matrix plus [`TestRealServerRejectsDeclaredRepresentationTrailersBeforeDispatch`](../../gateway/internal/modelturnhttp/server_test.go) |
| Malformed, unsafe-ID, read-failed, or oversized body | Existing executor | Existing `invalid request\n` with `400` | [`TestHandlerPresentsUncorrelatedAdmissionFailures`](../../gateway/internal/modelturnhttp/admission_test.go) |
| Valid ID with invalid shape or alias | Existing executor | Existing `invalid_request` with `400`; zero calls | [`TestHandlerPresentsCorrelatedAdmissionFailuresWithoutParsingTheirBodies`](../../gateway/internal/modelturnhttp/admission_test.go) |
| `tool_calls` plus unknown alias | Existing executor | `unsupported_capability` with `422`; capability wins | Same correlated-admission table and empty fake |
| Completed result with absent versus observed-zero usage | HTTP presentation | `200`; usage omitted versus present with zero counters | [`TestHandlerPresentsCompletedResultsAndUsagePresence`](../../gateway/internal/modelturnhttp/presentation_test.go) |
| Normalized provider failure | HTTP presentation | Locked status/message; exact code, retryability, and optional usage | [`TestHandlerPresentsEveryNormalizedProviderFailure`](../../gateway/internal/modelturnhttp/presentation_test.go) |
| Matching canceled or expired context | Request control flow | Exact response abort; no writer access or implicit `200` | Direct and reached-once, non-timeout real-server tests in [`server_test.go`](../../gateway/internal/modelturnhttp/server_test.go) |
| Ordinary executor error or invalid outcome/accessor state | HTTP safety fallback | Fixed plain-text `500`; unsafe error not formatted | [`TestHandlerFallsBackSafelyForOrdinaryExecutorAndInvalidOutcomeState`](../../gateway/internal/modelturnhttp/admission_test.go) |
| Marshal or body-write failure | HTTP response boundary | Precommit fixed `500`, or return after exactly one committed write; no retry | [`TestMarshalFailureFallsBackBeforeResponseCommitWithoutFormatting`](../../gateway/internal/modelturnhttp/presentation_test.go) and [`TestHandlerDoesNotRetryAfterResponseWriteFailure`](../../gateway/internal/modelturnhttp/server_test.go) |
| Concurrent strict-fake use | Outside ICGT-010 | Not attempted | Explicit deferral and race gate |

## Test and validation evidence

The focused suite is divided by review boundary:

| Test file | Observed evidence |
| --- | --- |
| [`handler_test.go`](../../gateway/internal/modelturnhttp/handler_test.go) | Constructor error, reciprocal exact-target inventory, transport precedence, ordinary and trailer field presence, zero body reads/closes, lexical media profile, direct HEAD body, and commit-time header snapshot |
| [`admission_test.go`](../../gateway/internal/modelturnhttp/admission_test.go) | Uncorrelated and correlated ICGT-009 failures, capability-before-alias behavior, invalid provider replacement, safe ordinary fallback, and direct body ownership |
| [`presentation_test.go`](../../gateway/internal/modelturnhttp/presentation_test.go) | Escaped completed output, absent/zero/nonzero usage, all seven provider failures, semantic fixture parity, compact framing, and safe precommit marshal fallback |
| [`server_test.go`](../../gateway/internal/modelturnhttp/server_test.go) | Direct and real abort semantics, non-timeout/reached-once evidence, exact writer calls, direct versus wire HEAD behavior, real declared-trailer rejection, bounded loopback cleanup, and complete fake consumption |

These focused commands passed:

```text
GOCACHE=/tmp/icgt010-go-cache TMPDIR=/tmp go test ./gateway/internal/modelturnhttp
GOCACHE=/tmp/icgt010-go-cache TMPDIR=/tmp go test -count=20 ./gateway/internal/modelturnhttp
GOCACHE=/tmp/icgt010-go-cache TMPDIR=/tmp go vet ./gateway/internal/modelturnhttp
GOCACHE=/tmp/icgt010-go-cache-race TMPDIR=/tmp go test -race ./gateway/internal/modelturnhttp
```

They ran outside the restricted tool sandbox only because `httptest.NewServer` needed to bind local
loopback sockets; they remained offline and credential-free. The final repository-wide
`TMPDIR=/tmp GOCACHE=/tmp/icgt010-full-gocache ./scripts/check` also passed: it checked 122 repository
files, ran 52 policy tests, validated the model-turn fixtures, and passed all Go tests and race tests.

## Speed and size implications

ICGT-010 adds no provider call, retry, timer, queue, or background goroutine beyond the one invocation
already owned by ICGT-009.

The handler marshals one complete non-streaming provider outcome before committing a status. That
creates one response-sized byte slice proportional to an already bounded result. It also means the
first response byte cannot be written until the complete result is available and serialized. This is
expected for the current non-streaming contract; later streaming work owns incremental time to first
byte.

The handler creates no second whole-request buffer. The existing 8 MiB-plus-one admission bound
remains unchanged. The final implementation is 321 production lines across two files and exposes only
`NewHandler`. It uses the Go standard library plus existing repository packages, adds no external
dependency, and produces no `go.sum`. No benchmark was run, so this unit makes no measured latency,
throughput, allocation, or exact serialized-size claim.

## What changed during implementation

- The first complete handler/presenter draft was 364 production lines. A split review simplified the
  explicit control flow to 319 lines without introducing a framework, reflection, or a second public
  abstraction. Representation-trailer safety review then added two lines, producing the final
  321-line package.
- HTTP review found that representation metadata can be declared as trailer keys before body values
  arrive. The handler now detects `Content-Encoding` and `Content-Type` in `Request.Trailer` during
  preflight. Direct tests prove fixed `415` with zero handler/executor body reads and zero provider
  calls; the real-server test proves declared keys are visible during preflight and dispatch remains
  at zero calls.
- The same review distinguished declared trailers from invalid undeclared content-format trailers.
  RFC 9110 does not permit processing-critical representation metadata to arrive only after the
  content. This profile does not merge or interpret such malformed late fields, and it does not read
  the body merely to invent a late `415`; doing so would break direct executor body ownership and the
  zero-work preflight invariant.
- Target review added reciprocal disagreement cases: exact raw target with a different parsed path,
  and exact parsed URL with a different raw target. These make a one-sided target check observably
  fail instead of relying only on encoded-path examples.
- Header-timing review changed assertions to read `httptest.ResponseRecorder.Result().Header`, the
  snapshot captured at `WriteHeader`, rather than the recorder's mutable header map. The counting
  writer separately proves exactly one `Header`, `WriteHeader`, and `Write` call on an ordinary
  completed response.
- The first real-server abort assertion could have passed for the wrong reason if its client deadline
  expired. The final test asserts the client request context is still live and the termination invoker
  ran exactly once before accepting the no-response transport error.
- The strict fake remains fresh and serial in every test. Real loopback coverage does not turn it into
  a concurrency-safe runtime provider, and neither the health-only service nor executable changed.
- No provider SDK, external dependency, route registration, listener policy, retry, goroutine, timer,
  queue, logger, visual lesson, `go.sum`, or `go.work` was introduced.

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
| Exposure | Implemented, independently tested handler only | Explicitly registered and operated route |
| Provider | Fresh serial strict fake in tests | Runtime-selected concurrency-safe adapter |
| Network boundary | Focused `httptest` loopback evidence | Enforced listener, TLS, and authentication policy |
| Concurrency | Deliberately unclaimed | Bounded active requests and owned rejection policy |
| Response mode | Fully buffered non-streaming JSON | Versioned SSE with cancellation and backpressure |
| Failures | Fixed safe presentation | Operational correlation, metrics, and incident evidence |
| Retries | None | Later narrowly reviewed provider/network policy |
| Client integration | No CAH adapter | Separately owned and pinned CAH `FastGateProvider` |

### Trade-offs and graduation signals

The implemented handler stays small because it trusts already validated internal values and does not
own runtime concurrency. This makes HTTP mapping personally reviewable and prevents the strict fake
from quietly becoming a production server component.

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
- Explain why a declared `Content-Encoding` trailer can be rejected during preflight but an invalid
  undeclared late field cannot be turned into a new `415` without changing body ownership.
- Write the expected completed JSON for absent usage, then for present zero usage.
- Trace `authentication_failed` from `provider.Failure` to its fixed message and explain why the
  result is not a caller `401`.
- Simulate a writer that fails after `WriteHeader` and explain why another response or provider call
  would be unsafe.
- Compare a handler tested through `httptest.NewServer` with one registered by the FastGate command.

## Key takeaways

- HTTP target and representation checks happen before model-turn body admission.
- Declared representation trailer keys are preflight metadata; malformed undeclared late metadata is
  neither merged nor reclassified after provider work.
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
- **Declared trailer:** A field name announced before the body whose value is supplied after the body;
  Go exposes the declared key in `Request.Trailer` when the handler begins.
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
