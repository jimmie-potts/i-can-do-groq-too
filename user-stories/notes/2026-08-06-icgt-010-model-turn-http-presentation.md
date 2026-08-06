# ICGT-010 model-turn HTTP presentation implementation note

## Outcome

ICGT-010 adds the standard-library-only `gateway/internal/modelturnhttp` package. Its one exported
API, `NewHandler(*modelturn.Executor) (http.Handler, error)`, constructs an injectable HTTP boundary
without mounting a route, choosing a provider, starting a listener, or claiming concurrent use of
the strict fake. The default FastGate executable therefore remains health-only.

The handler accepts only the reviewed `POST /v1/model-turns` transport profile, passes the request
context and body directly to the existing executor, and presents every non-terminated closed outcome
as one bounded response. Matching caller-context cancellation or deadline returns no application
response; the handler panics with the exact `http.ErrAbortHandler` sentinel before writer access.

The final production package contains 321 lines across two files. That is one line above the story's
rough 320-line review heuristic because adversarial review added declared representation-trailer
protection. The safety check remains beside the other transport checks so the complete preflight is
personally reviewable in one place.

## Important control flow and ownership

[`handler.go`](../../gateway/internal/modelturnhttp/handler.go) owns only HTTP target, method,
representation preflight, executor entry, common headers, status commitment, and one body write.
The checks run in the fixed order target, method, content encoding, then content type. Only the
accepted branch calls `Executor.Execute(request.Context(), request.Body)`.

[`presentation.go`](../../gateway/internal/modelturnhttp/presentation.go) reads only public
`modelturn.Outcome` and provider value accessors. It preserves admission-owned failure bodies,
constructs typed completed and provider-failure documents, distinguishes absent usage from observed
zero, and maps the seven normalized provider codes to reviewed statuses and fixed messages. It does
not parse failure JSON, unwrap or format provider errors, retry work, or log request/provider content.

ICGT-009 remains the sole owner of the 8 MiB-plus-one body bound, strict JSON, correlation,
capability-before-alias policy, mapping, and provider invocation. `http.Server` remains the owner of
server request-body closure. ICGT-011 owns route mounting, actual-listener loopback enforcement,
runtime-provider selection, and bounded concurrency.

## Human-sized checkpoints delivered

1. [`handler_test.go`](../../gateway/internal/modelturnhttp/handler_test.go) locks exact raw and parsed
   targets, method and representation precedence, case-insensitive header inventory, declared
   representation trailers, the narrow JSON media profile, fixed responses, committed headers, and
   zero application body reads on transport rejection.
2. [`admission_test.go`](../../gateway/internal/modelturnhttp/admission_test.go) proves uncorrelated,
   correlated invalid, unsupported-capability, alias, ordinary-error, and invalid-outcome mappings
   while preserving ICGT-009 behavior and direct body ownership.
3. [`presentation_test.go`](../../gateway/internal/modelturnhttp/presentation_test.go) covers completed
   output escaping, usage absence/zero/nonzero, all seven normalized provider failures, canonical
   fixture parity, compact JSON, and the pre-commit marshal fallback.
4. [`server_test.go`](../../gateway/internal/modelturnhttp/server_test.go) covers direct and real
   response abort, one-write failure behavior, direct versus wire HEAD semantics, declared trailers,
   accepted chunked framing, bounded response cleanup, and one exact serial fake exchange.

## Review-driven changes

- The first complete production draft was 364 lines. Removing one-use layers and keeping two direct
  source files reduced it to 319 without hiding state or control flow.
- Case-insensitive header iteration replaced canonical-key lookup so direct requests with differently
  cased map keys cannot bypass repeated-field or encoding checks.
- Adversarial review found that Go exposes declared trailer names through `Request.Trailer` with nil
  values before a handler reads the body. The handler now rejects predeclared `Content-Encoding` and
  `Content-Type` names, and direct plus real chunked tests prove rejection before provider dispatch.
- Valid chunked framing remains supported. An undeclared late content-format trailer is invalid under
  RFC 9110 and is not merged or interpreted. The handler does not issue a late `415` after execution,
  because provider work may already have occurred and that would falsely present a zero-work
  transport rejection.
- The original target matrix changed raw and parsed forms together. Reciprocal mutations now prove
  `RequestURI` and parsed `URL.Path` are independently required.
- Direct response tests originally inspected `ResponseRecorder.Header()`, whose live map can show
  changes made after commitment. They now inspect `ResponseRecorder.Result().Header`, the snapshot
  captured when `WriteHeader` ran.
- The real-server abort test originally accepted any client error after one invocation, including a
  possible test timeout. It now proves the request context is still live when the connection abort is
  observed; the direct test separately pins the exact sentinel and zero writer calls.
- Typed response documents make normal JSON marshal failure structurally unreachable. A pure response
  preparation helper tests the fixed fallback without adding a mutable production hook.

These reusable missed-case classes are recorded in
[`docs/pr-review-checklist.md`](../../docs/pr-review-checklist.md).

## Validation

The focused implementation and adversarial-review fixes passed:

```text
TMPDIR=/tmp GOCACHE=/tmp/icgt010-gocache go test -count=20 ./gateway/internal/modelturnhttp
TMPDIR=/tmp GOCACHE=/tmp/icgt010-gocache-race go test -race ./gateway/internal/modelturnhttp
TMPDIR=/tmp GOCACHE=/tmp/icgt010-gocache go vet ./...
```

The final `TMPDIR=/tmp GOCACHE=/tmp/icgt010-full-gocache ./scripts/check` also passed. It checked 122
repository files, ran 52 repository policy tests, validated the model-turn fixtures, and passed all
Go tests and race tests. Local-loopback tests ran outside the restricted network sandbox only so
`httptest.NewServer` could bind an ephemeral loopback port; they remained offline and
credential-free.

## Personal review checkpoint

Personally review `ServeHTTP` and `transportRejection` in `handler.go`, then `prepareOutcome`,
`prepareProviderOutcome`, `prepareUsage`, and `writePreparedResponse`. Pair those paths with the
transport zero-read table, completed usage table, exact abort tests, declared-trailer loopback test,
and final serial fake exchange named above.

The reviewer should be able to explain this invariant: transport and model-turn admission rejections
start no provider work; an admitted non-terminated outcome becomes exactly one prepared response;
and matching caller termination or a response-write failure can never authorize another provider
call or invented terminal outcome.

## ICGT-011 handoff

The next story may mount this handler only after it selects an explicit runtime provider or demo
policy, validates the actual bound listener as loopback-only, and bounds concurrent work. It must let
`http.ErrAbortHandler` reach `net/http`, keep the single-owner strict fake out of unscripted concurrent
traffic, and preserve the Code Assist Harness boundary: FastGate owns server transport, while a
future CAH adapter owns trusted endpoint selection and client-side workflow mapping.
