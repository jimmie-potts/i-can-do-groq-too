# ICGT-009 - Admit and execute one fake-backed model turn

- **Status:** Done
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-008
- **Lesson:** [Bounded model-turn admission](../docs/lessons/icgt-009-bounded-model-turn-admission.md)
- **Review priority:** High

## User story

> As a gateway developer, I want one bounded model-turn v1 admission path in front of the
> deterministic fake so that ambiguous, unsupported, or unknown requests cannot start provider work
> and one admitted request produces exactly one provider-neutral invocation.

## Primary concept and invariant

This story owns one ordered boundary from untrusted request bytes to the existing provider port. Its
single invariant is:

> Provider work starts exactly once only after the complete raw body, strict JSON profile, v1 request
> shape, capability policy, and logical alias have all been admitted. Every admission rejection starts
> zero provider work.

Keeping admission and the first invocation in one story lets the deterministic fake prove both sides
of that invariant. The reviewed pre-implementation estimate is 450 through 650 changed production
lines: roughly 80-110 for bounded input and outcomes, 160-240 for strict parsing, 150-220 for exact v1
decoding and mapping, and 60-80 for execution. This is a bounded exception to the usual roughly
400-line review heuristic. Splitting the parser into a separately delivered story would create an
intermediate trusted-request seam before the zero-or-one-dispatch invariant can be proved and would
duplicate the adversarial corpus across that seam. The five review pauses below keep the coherent unit
personally reviewable. Stop and review a split before continuing if production code would exceed 650
changed lines or require more than the two main exported abstractions locked below.

## Scope

- Introduce one internal model-turn admission/execution package above `provider.Invoker` and below the
  future HTTP handler.
- Accept a nonnil caller `context.Context` and an `io.Reader`; consume but do not close the reader.
- Bound the raw encoded request before parsing can retain more than the cap; allocate provider-domain
  collections only after their decoded cardinalities pass.
- Enforce the complete model-turn v1 strict JSON and request-schema profile at runtime without
  loading the Python checker or schema files in production.
- Recover a client `request_id` only when the complete strict document makes it safe to echo.
- Create fixed, bounded admission failures without copying request content, aliases, parser text, or
  reader errors into diagnostics or responses.
- Reject the declared `tool_calls` requirement and unknown logical aliases before provider dispatch.
- Map the admitted wire request into the existing provider-neutral request and invoke the executor's
  injected provider port exactly once; use the ICGT-008 fake as the primary zero/one-call evidence.
- Validate the one returned provider alternative, preserving an exact valid result or error beside the
  admitted request ID and converting an invalid provider return into one fixed internal outcome.

## Locked behavior

### Small Go boundary

- The package is `gateway/internal/modelturn` and remains standard-library-only.
- Its exact entry points are `NewExecutor(provider.Invoker) (*Executor, error)` and
  `(*Executor).Execute(context.Context, io.Reader) (Outcome, error)`. `Executor` and `Outcome` are the
  only two main exported abstractions. Private strict-parse, wire-shape, and mapping helpers do not
  become a generic JSON or routing framework.
- `Outcome` is a concrete exported value type with unexported fields. Every `Outcome` returned with a
  nil ordinary error has exactly one of four alternatives: an uncorrelated admission failure with the
  exact canonical 16-byte body; a correlated admission failure with a schema-valid fixed body; an
  admitted provider outcome with its request ID and exact valid result or error; or a correlated fixed
  internal failure after mapping detects an inconsistency or one invocation returns an invalid
  provider alternative.
- The public outcome surface exposes `RequestID() (string, bool)`,
  `FailureBody() ([]byte, bool)`, and
  `FailureCode() (string, bool)`, plus
  `ProviderOutcome() (provider.Result, error, bool)`. Every correlated failure exposes its exact
  model-turn error code. The uncorrelated failure reports a body but no request ID and no protocol
  failure code because it is not a `model_turn.failed` document. A provider outcome reports no failure
  body or code. This lets ICGT-010 distinguish and map every valid state without parsing or
  byte-matching a failure body.
  Failure-body access returns a copy, provider result/error identity is unchanged for a valid provider
  alternative, and callers outside the package cannot construct a valid or mixed state. The
  constructible zero value is invalid: all four accessors report `false`. `Execute` returns that zero
  value with a nonnil ordinary error, and callers ignore the value on that path.
- `Execute` returns a nonnil ordinary error only for the fixed, content-safe caller-programming cases
  below. Request rejections, mapping inconsistencies, and valid or invalid post-dispatch provider
  alternatives are closed `Outcome` values with a nil ordinary error.
- The executor owns one injected `provider.Invoker` and recognizes exactly one logical alias,
  `learning-text`. That FastGate-owned learning name admits the configured invoker; it is not a provider
  model ID and does not hard-code the deterministic fake in production.
- The input is an `io.Reader`, not a preallocated `[]byte`, so this package can enforce the body cap
  before proportional allocation. It never closes the reader; the future transport owner does.
- `NewExecutor` rejects a nil `provider.Invoker` interface with exactly
  `model-turn invoker is required`. `Execute` rejects a nil context or nil reader interface before
  reading or dispatch with exactly `model-turn context is required` or
  `model-turn body reader is required`. No reflection-based typed-nil detection is added: an interface
  holding a typed nil is a caller programming error outside this generic boundary's guarantee.
- The caller's nonnil context is passed unchanged to the one invocation. This story does not add
  cancellation checkpoints, deadlines, retries, goroutines, or cleanup policy.

### Raw-body and parser bounds

- `MaxRequestBodyBytes` is exactly 8 MiB: `8_388_608` bytes.
- The implementation successfully obtains and retains at most `MaxRequestBodyBytes + 1`, or
  `8_388_609`, bytes. Each `Read` call offers only the remaining capacity needed to reach that limit;
  short reads may make the cumulative offered buffer sizes larger, so no cumulative-request claim is
  made. An otherwise valid document of exactly the limit is admissible; one byte over is rejected
  before strict parsing and provider dispatch.
- Every returned `n > 0` counts even when the same `Read` also returns `io.EOF` or another error.
  `io.EOF` after no more than the exact limit completes the body. Any non-EOF error discards the
  partial prefix and yields the fixed uncorrelated admission failure without exposing the raw error.
  After 100 consecutive `(0, nil)` reads, the implementation treats the reader as making no progress,
  uses `io.ErrNoProgress` only internally, and returns that same fixed failure instead of spinning.
- The root object or array has container depth 1; every nested `{` or `[` increments the depth. The
  runtime accepts at most 16 simultaneously open containers. Model-turn v1 needs only three; this
  independent guard bounds hostile invalid input and is not copied from the offline checker's
  artifact-only depth limit.
- The 8 MiB runtime body cap is independent of the offline fixture checker's 1,000,000-byte artifact
  guard. A schema-conforming document with heavy escaping or multibyte text can still exceed the raw
  cap and be rejected.

### Strict JSON and exact v1 shape

- Production admission uses small request-specific Go code built from stable standard-library
  primitives. It does not enable experimental `encoding/json/v2`, add a general JSON Schema
  dependency, execute Python, or read repository schema/fixture files at runtime.
- Before schema admission, it rejects invalid UTF-8, a lone escaped UTF-16 high or low surrogate,
  duplicate decoded object names at any depth, non-RFC 8259 numeric spellings, trailing non-whitespace,
  and a second top-level JSON value.
- A valid escaped surrogate pair remains valid and counts as one Unicode scalar value. Duplicate
  detection compares decoded member names, so differently escaped spellings of the same name still
  conflict.
- The decoded root must match the exact committed v1 request field inventory, required fields,
  constants, types, enums, identifier rules, string bounds, control-safe instruction-source rule,
  and collection bounds. Unknown fields and case variants are rejected.
- Raw parser input is bounded by the accepted body. Standard-library intermediate decoded state may
  allocate in proportion to that bounded input; provider-domain conversation, instruction, and
  capability slices are allocated or copied only after their decoded cardinalities pass. Accepted
  order and exact Unicode content are preserved into `provider.NewRequest` without normalization.
- Every committed request fixture is exercised through the Go admission path. Production code does
  not depend on the fixture corpus; tests use it to detect drift between the frozen language-neutral
  contract and the runtime implementation.

### Safe correlation and fixed admission failures

- A request ID is safe to echo only after the entire body is within the cap, the whole document
  passes the strict JSON profile, the root is one object with one `request_id`, and that value is a
  string satisfying the exact 1-through-128-character ASCII identifier rule.
- Never scrape an ID from a prefix or echo one from an oversized, read-failed, malformed,
  duplicate-key, invalid-UTF-8, lone-surrogate, non-object, missing-ID, wrong-type, or lexically
  invalid-ID document.
- When no safe ID exists, `Outcome.FailureBody()` returns the exact 16-byte ASCII value
  `invalid request\n`: the text `invalid request` followed by one line-feed byte. It is deliberately
  not JSON and does not pretend to satisfy the
  `model_turn.failed` schema, which requires a request ID. ICGT-009 owns this canonical payload;
  ICGT-010 later writes it and assigns its HTTP status and media type.
- Once a safe ID exists, any remaining strict v1 shape or ordinary semantic-admission failure uses a
  schema-valid `model_turn.failed` envelope with the exact ID, code `invalid_request`, message
  `The request is invalid.`, `retryable: false`, and no usage.
- If schema-to-provider mapping reveals an internal contract inconsistency after schema admission,
  it fails closed with the safe ID, code `internal_error`, fixed message
  `The request could not be processed.`, `retryable: false`, no usage, and zero dispatch. It never
  exposes the underlying validation error or mislabels the defect as client input.

### Semantic ordering and exactly-once dispatch

- Complete raw-body, strict-JSON, safe-ID, and v1 schema admission occur before semantic policy.
- Capability policy runs before alias selection. A schema-valid `tool_calls` requirement returns
  `unsupported_capability` with the exact admitted ID, message
  `The required capability is not supported.`, `retryable: false`, and no usage. It is semantically
  equivalent to the committed canonical failure fixture.
- Therefore, a request containing both `tool_calls` and an unknown alias deterministically returns
  `unsupported_capability`; alias lookup and provider dispatch do not occur.
- With no required capability, only `learning-text` is accepted. Any other alias returns the fixed
  correlated `invalid_request` failure without exposing the supplied alias.
- `learning-text` admits the executor's injected invoker. Rejection tests use a fresh empty
  deterministic fake script and leave `VerifyComplete` successful; admitted-request tests use that
  fake to consume exactly one expected exchange.
- Wire-only `version`, `kind`, and `request_id`, plus routing-only `model_alias`, never enter
  `provider.Request`. Ordered conversation, ordered generic instructions, and the empty admitted
  capability list map exactly.
- Immediately after the only `Invoke` call, the executor passes the exact caller context, result, and
  error to `provider.ValidateInvocation`. A valid `provider.Result`, direct `*provider.Failure`,
  optional usage presence, or exact caller-context sentinel crosses unchanged beside the admitted
  request ID. This story does not serialize any valid provider outcome or reinterpret retryability.
- If validation rejects a raw or wrapped error, fabricated context termination, invalid result,
  simultaneous result/error, or any other invalid alternative, the executor does not traverse,
  format, retain, or expose that error. After exactly one dispatch it returns a correlated fixed
  `internal_error` outcome with `The request could not be processed.`, `retryable: false`, and no
  usage. This post-dispatch internal failure is not an admission rejection.

## Human-sized implementation checkpoints

These checkpoints are review pauses inside one story, not separate public contracts:

1. **Outcome boundary:** introduce the exact constructor, executor, closed outcome accessors, nil
   behavior, and fixed safe failure bodies; review that no HTTP behavior appears.
2. **Bounded syntax:** add the limit-plus-one reader and strict JSON preflight; review exact and
   one-over reads, no-progress and mixed `n`/error behavior, duplicate names, Unicode, full-document
   framing, and the 16-level guard.
3. **Schema and correlation:** add the v1 request-specific decoder, all field bounds, fixture parity,
   and the safe-ID matrix; review provider-domain allocation only after cardinality admission.
4. **Semantic gates:** add capability-before-alias ordering and empty-fake zero-dispatch evidence;
   review exact failure bodies and content-safe diagnostics.
5. **One invocation:** map admitted values into `provider.Request`, consume one fake exchange, validate
   the returned alternative, preserve valid outcomes, and fail closed on a deliberately invalid
   recording invoker.

Each checkpoint must pass its focused tests before the next begins. The completed unit still uses one
scoped story commit and one ready-for-review pull request unless review makes a split necessary.

## Acceptance criteria

1. Recording readers prove the implementation obtains and retains no more than 8,388,609 bytes,
   limits each offered buffer to remaining capacity, counts `n > 0` before `io.EOF` or another error,
   accepts an otherwise valid exact-limit body, rejects one extra byte, discards a non-EOF partial
   prefix, and terminates after 100 consecutive `(0, nil)` reads, all before parsing or dispatch.
2. A separate exact/one-over depth test pins root depth 1 and the 16-container guard, and no runtime
   guard is presented as a model-turn schema rule or copied from the offline artifact checker.
3. Runtime admission enforces the complete strict JSON profile and exact request v1 schema. Every
   request fixture receives the expected parse/schema classification, with additional nested
   duplicate, decoded-name collision, invalid-UTF-8, both lone-surrogate directions, trailing-value,
   and non-finite-number cases.
4. Safe-ID tests cover every correlated and uncorrelated class. Each correlated failure code matches
   its fixed body; the uncorrelated class reports a failure body but no ID or protocol code; and a
   provider outcome reports no failure body or code. ICGT-010 can distinguish every valid state without
   body parsing. Response and diagnostic assertions use sentinel body, prompt, instruction, alias, and
   reader-error text and prove none is exposed.
5. A schema-valid `tool_calls` request produces the canonical `unsupported_capability` envelope and
   leaves an empty fake complete. A combined unknown-alias/tool request proves capability precedence.
6. `learning-text` is the only admitted alias and admits the injected invoker. Unknown-alias tests
   produce a fixed safe failure with zero fake calls; a direct private mapping-helper test proves an
   otherwise unreachable schema-to-domain inconsistency becomes the fixed correlated internal failure
   before dispatch.
7. A nil invoker interface, context, or reader returns its exact fixed programming error with no read
   or call. Tests also document that typed-nil dynamic values are outside generic nil detection.
8. An admitted request maps every provider-domain field exactly. A small recording invoker proves the
   exact caller context reaches the one call. Valid result, direct normalized failure, usage absent
   versus observed zero, and matching context sentinels retain exact identity after
   `provider.ValidateInvocation`.
9. Deliberately invalid recording invokers cover raw, wrapped, and non-comparable errors without
   formatting or unwrapping, direct typed-nil `*provider.Failure`, fabricated cancellation, invalid
   result, and simultaneous result/error. Each is called once and becomes only the fixed correlated
   `internal_error` outcome with no unsafe content or usage.
10. The implementation contains no HTTP request/response/status/media-type/listener type, route,
   provider SDK, credential, filesystem/runtime-schema dependency, network I/O, logging of body
   content, retry, timer, goroutine, queue, or dynamic routing registry.
11. The lesson contains exact production and test links, focused happy/failure
   excerpts, observed trade-offs and review changes, exercises, glossary, and exactly three
   teach-back questions.
12. Focused tests, repeated deterministic tests, applicable `go vet` and race checks, the PR review
    regression checklist, independent adversarial review, and `./scripts/check` all pass.

## Human review checkpoint

- **Production path:** Trace the bounded `io.Reader` entry point through strict parse, safe-ID
  recovery, exact v1 admission, capability and alias gates, `provider.NewRequest`, and exactly one
  `provider.Invoker.Invoke` call followed immediately by `provider.ValidateInvocation`.
- **Failure/test path:** Trace exact-limit versus one-over reading and the schema-valid `tool_calls`
  request through its fixed correlated failure while an empty ICGT-008 fake proves zero dispatch;
  then verify malformed no-ID input returns only `invalid request\n` (text plus one line feed) and an
  invalid recording-invoker return becomes the fixed internal failure after one call.
- **Invariant:** No rejected body can start provider work, and every admitted body starts exactly one
  provider-neutral invocation; every valid provider alternative is preserved and every invalid one is
  replaced without unsafe formatting, unwrapping, retention, or disclosure.
- **Deferred:** HTTP path/method/media type/status, request-body closure and listener binding,
  provider-result/failure wire mapping, fake concurrency policy, authentication, dynamic routing,
  retries, streaming, cancellation behavior, deadlines, cleanup, backpressure, telemetry, and live
  adapters.

## Validation

Implementation added and ran:

- focused Go tests for the new admission/execution package;
- fixture-parity, privacy-sentinel, exact-bound, zero-dispatch, and exactly-once tests;
- deterministic repeated focused tests;
- `go vet` and the race detector for the affected packages; and
- `./scripts/check`.

## Documentation impact

- The story, lesson, roadmap, root, FastGate, contract, and index status documents describe the
  implemented boundary without claiming an HTTP endpoint.
- The evidence-backed implementation note records review discoveries and the original combined
  ICGT-010 handoff, now clarified as ICGT-010 presentation plus ICGT-011 runtime binding.
- The PR-review checklist records reusable allocation-capacity, observable ordering/reset, and
  actual-operands interface-equality review classes exposed by implementation review.

## Out of scope

- Binding or changing any HTTP endpoint, including `/healthz`.
- HTTP method, media type, status, header, listener, proxy, redirect, authentication, TLS, or
  request-body lifecycle behavior.
- Serializing completed results or any provider-origin failure.
- Adding a provider SDK, live provider, credential, provider model ID, configuration framework,
  alias registry, capability discovery, emulation, fallback, or routing table.
- Changing Code Assist Harness types, workflow state, tools, approvals, transcripts, retries, or
  correctness evaluation.
- Retrying, streaming, concurrency, cancellation policy, deadlines, cleanup, backpressure, metrics,
  quota, tenancy, or billing behavior.
- Creating a visual lesson or modifying the ICGT-008 deterministic fake.
