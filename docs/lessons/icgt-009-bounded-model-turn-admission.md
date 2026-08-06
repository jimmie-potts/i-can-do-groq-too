# ICGT-009 lesson: Bounded model-turn admission before provider dispatch

- **Unit:** ICGT-009
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; FastGate has a bounded model-turn v1 admission path and one
  validated provider-neutral invocation
- **Story:** [ICGT-009](../../user-stories/icgt-009-admit-and-execute-model-turn.md)
- **Review priority:** High
- **Visual companion:** Not required; the Markdown lesson is the reviewed learning artifact
- **Related architecture:** [ADR 0003](../adr/0003-fastgate-api-surface.md),
  [model-turn v1](../../gateway/contracts/model-turn/v1/README.md), and
  [provider contracts](../../gateway/internal/provider/provider.go)

> The examples below link to the exact implementation and tests delivered by ICGT-009.

## Quick summary

ICGT-009 connects the model-turn v1 wire meaning to the existing provider-neutral port without
exposing an HTTP route. It accepts a bounded `io.Reader`, strictly parses and validates one request,
rejects unsupported requirements or aliases before dispatch, maps an admitted request into
`provider.Request`, and invokes one injected `provider.Invoker`. The deterministic fake proves the
ordinary zero/one-call cases; small recording invokers prove exact context forwarding and safe handling
of an invalid provider return.

The central invariant is: **the provider is called exactly once only after the whole request passes
admission; every admission rejection causes zero provider calls.**

## Learning objectives

After completing this unit, you should be able to:

- distinguish raw-body, strict-JSON, schema, capability, and alias admission;
- explain why an `io.Reader` and a limit-plus-one read bound memory before parsing;
- determine when a client-supplied `request_id` is safe to echo;
- trace accepted v1 fields into the smaller provider-neutral request;
- explain why capability rejection occurs before alias selection; and
- prove zero-or-one dispatch and validate the one returned provider alternative.

## Why this unit matters

ICGT-006 defines valid language-neutral documents, but its offline checker neither receives service
traffic nor calls a provider. ICGT-007 defines safe provider-domain values, but it does not decide
whether client input is admissible. ICGT-008 can verify an interaction, but nothing yet connects a
client document to that fake.

ICGT-009 is that policy boundary. ICGT-010 can concentrate on HTTP preflight and outcome presentation
instead of mixing parsing, correlation, routing, dispatch, and transport in one handler. ICGT-011
separately owns service binding, runtime-provider selection, and concurrency.

## Junior engineer foundation

A request can pass one check and still fail a later one:

- **Raw-body admission** limits encoded bytes before decoding.
- **Strict JSON parsing** proves the bytes form exactly one unambiguous JSON document.
- **Schema admission** proves the decoded values match model-turn v1.
- **Semantic admission** applies current server policy, such as supported capabilities and aliases.
- **Provider dispatch** begins upstream work only after all earlier stages succeed.

A common misconception is that “valid JSON” means “valid request.” A document can be valid JSON but
contain an unknown field, request `tool_calls`, name an unknown alias, or exceed the runtime body cap.

The accepted raw-body limit is exactly 8 MiB, or 8,388,608 bytes. A bounded loop successfully obtains
and retains at most one extra byte. That distinguishes an exactly-at-limit body from an oversized one
without first accepting an unbounded allocation.

## Key concepts

### A transport cap is separate from schema bounds

Model-turn v1 bounds decoded fields. It does not promise that every escaped or multibyte encoding of
those fields fits in 8 MiB. The largest permitted ASCII conversation and instruction content totals
roughly 6.0 MiB before JSON framing, so 8 MiB leaves room for that straightforward representation.
A heavily escaped or multibyte but shape-conforming document can still exceed the raw cap and be
rejected.

This cap is an intentional runtime admission rule, not accidental reuse of the offline checker's
1,000,000-byte fixture-artifact guard. The two limits have different owners and purposes.

### The limit must act before the allocation

This unsafe sequence does not protect memory:

**PSEUDOCODE ONLY — unsafe example:**

```text
raw = read_everything(reader)
if length(raw) > 8 MiB:
    reject
```

The unbounded allocation already happened before the check. The implemented path retains at most
8 MiB plus one byte. Each read receives only the remaining buffer capacity, but a short-reading source
may be offered multiple buffers over time; the meaningful bound is bytes successfully obtained and
retained, not the sum of every buffer length offered. The extra retained byte is evidence of overflow;
it is never part of an admitted document.

Go readers may return data and an error together. The loop counts every `n > 0` before
interpreting `io.EOF` or another error. EOF at or below the exact cap completes the body. A non-EOF
error discards the partial prefix, and 100 consecutive `(0, nil)` reads become an internal
`io.ErrNoProgress` condition so a broken reader cannot spin forever. Neither raw error reaches the
outcome.

### Strict parsing removes ambiguity

The request-specific Go boundary must reject:

- invalid UTF-8;
- duplicate decoded object names at every depth;
- lone Unicode surrogate escapes;
- trailing or multiple JSON values;
- non-RFC 8259 numeric spellings; and
- more than 16 simultaneously open objects and arrays.

The stable Go `encoding/json` package has compatibility behaviors that do not alone satisfy every
strict-v1 rule, including duplicate-name and invalid Unicode handling. The implementation therefore
uses small, inspectable request-specific checks built from stable standard-library primitives. It does not
enable the experimental `encoding/json/v2` package or add a general schema dependency for this
unit.

The root object or array starts at container depth 1, and each nested object or array adds one. The
16-container rule is a runtime resource guard. A valid v1 request needs only three nested containers,
so this is not a new schema allowance or a copy of the offline checker's depth guard.

The body cap bounds parser input. Stable standard-library decoding may still allocate intermediate
values in proportion to that bounded input. The stronger collection rule applies at the provider
seam: conversation, instruction, and capability slices are allocated or copied into provider-domain
values only after decoded cardinalities pass.

### Correlation requires a proven-safe identifier

A `request_id` is safe to echo only after the complete body passes strict parsing and the identifier
itself matches the exact v1 rule. The runtime must never scrape an ID from a malformed prefix.

| Input state | Safe ID? | Implemented outcome |
| --- | --- | --- |
| Oversized, read-failed, or malformed body | No | Exact `invalid request\n` body |
| Duplicate key, invalid UTF-8, or lone surrogate anywhere | No | Exact `invalid request\n` body |
| Strict document but missing, wrong-type, or invalid ID | No | Exact `invalid request\n` body |
| Strict document with valid ID but another v1 violation | Yes | Correlated `invalid_request` |
| Valid v1 shape requiring `tool_calls` | Yes | Correlated `unsupported_capability` |
| Fully admitted `learning-text` request | Yes | Exactly one provider invocation |

The fixed uncorrelated body is 16 ASCII bytes: the text `invalid request` followed by one line feed,
written as `invalid request\n`. It is deliberately not a `model_turn.failed` JSON document because
that schema requires a safe `request_id`. ICGT-010 will later choose its HTTP status and media type.

### Capability admission precedes alias selection

`tool_calls` is schema-valid so callers can declare the requirement honestly, but base v1 has no
tool definitions or tool-result grammar. The requirement therefore produces the canonical
`unsupported_capability` failure with `retryable: false`, no usage, and zero provider calls.

This check runs before alias selection. A request containing both `tool_calls` and an unknown alias
deterministically reports the capability failure without consulting a route.

### An alias is not a provider model ID

ICGT-009 recognizes only the logical alias `learning-text`, already used by the canonical request
fixtures. It does not expose an OpenAI, Groq, or other vendor model name. An unknown alias produces a
correlated, generic `invalid_request` and zero dispatch; the supplied alias never appears in the
failure or diagnostic.

`learning-text` admits the invoker supplied to the executor. Production code does not import or
construct the deterministic fake. Tests inject that fake because its script makes zero and exactly one
call easy to prove.

### A closed outcome keeps HTTP out of admission

The package exposes two main types and these entry points:

```go
NewExecutor(provider.Invoker) (*Executor, error)
(*Executor).Execute(context.Context, io.Reader) (Outcome, error)
```

These are the exact implemented signatures. `Outcome` is a concrete value type with
private fields. Every `Outcome` returned with a nil ordinary error has exactly one of four
alternatives:

1. an uncorrelated admission failure with exact `invalid request\n` bytes;
2. a correlated admission failure with a fixed schema-valid body;
3. an admitted provider outcome with the safe request ID and exact valid result or error; or
4. a correlated fixed internal failure after mapping inconsistency or an invalid provider return.

The accessors are `RequestID() (string, bool)`, `FailureBody() ([]byte, bool)`,
`FailureCode() (string, bool)`, and
`ProviderOutcome() (provider.Result, error, bool)`. Every correlated failure reports its exact
model-turn code. The uncorrelated rejection reports a failure body but no request ID and no protocol
code because it is not a `model_turn.failed` document. A provider outcome reports neither a failure
body nor a failure code. ICGT-010 can therefore choose a status from the accessor combination without
parsing or byte-matching the body. A failure body is copied on access, valid provider result/error
identity is retained, and private fields prevent outside callers from constructing a valid or mixed
state. Go callers can still construct the zero value; it is invalid, all four accessors report `false`,
and `Execute` returns it only with a nonnil ordinary error. ICGT-009 owns the canonical failure payload;
ICGT-010 will write it and choose the HTTP status and media type.

The ordinary Go `error` return is only for fixed caller-programming failures. A nil invoker interface
is rejected by `NewExecutor` with `model-turn invoker is required`. A nil context or reader interface
is rejected before reading or dispatch with `model-turn context is required` or
`model-turn body reader is required`. The boundary does not use reflection to detect an interface that
contains a typed nil; supplying one remains the caller's programming error.

### The provider's return is validated too

Admission proves the request, but the injected invoker is still an interface implementation that can
break its own contract. Immediately after the one `Invoke` call, ICGT-009 passes the exact caller
context, result, and error to `provider.ValidateInvocation`.

A valid result, direct normalized failure, or matching context sentinel remains identical for
ICGT-010. Validation necessarily inspects the result/error alternative, but an invalid error is never
formatted, unwrapped through `errors.Is`/`errors.As`, retained, or exposed, and invalid result content
is never exposed. The invalid alternative becomes a correlated `internal_error` with the fixed message
`The request could not be processed.`, `retryable: false`, and no usage. Provider work already started
once, so this is a post-dispatch internal failure—not an admission rejection.

## Architecture and invariants

The implemented ownership rules are:

- admission accepts `context.Context` and `io.Reader`, not `http.Request`;
- this package consumes but does not close the reader;
- strict parsing completes before any request ID is trusted;
- capability admission completes before alias selection;
- only `learning-text` admits the executor's injected invoker;
- wire-only `version`, `kind`, and `request_id` do not enter `provider.Request`;
- routing-only `model_alias` does not enter `provider.Request`;
- conversation and instructions preserve caller order and exact Unicode;
- an admitted request calls the provider port exactly once;
- every returned provider alternative is validated immediately;
- a valid provider result or error remains unchanged for ICGT-010;
- an invalid provider alternative becomes a fixed internal outcome without formatting, unwrapping,
  retaining, or exposing the invalid error; and
- no goroutine, timer, retry, queue, SDK, credential, network call, or routing registry is
  introduced.

HTTP method, media type, status, headers, listener binding, request-body closure, authentication,
provider-outcome serialization, streaming, and cancellation behavior remain deferred.

## Practical walkthrough

Implementation used the five approved review pauses:

1. **Outcome boundary:** the exact API, closed outcomes, nil behavior, and fixed safe failures passed
   focused tests before parsing work was reviewed.
2. **Bounded syntax:** the limit-plus-one reader and strict JSON preflight passed exact-limit,
   no-progress, duplicate-name, Unicode, number, framing, and depth tests.
3. **Schema and correlation:** the request-specific decoder passed every committed request fixture,
   exact bounds, and the safe-ID matrix.
4. **Semantic gates:** `tool_calls` precedes alias selection, and empty fake scripts prove zero
   dispatch for both capability and alias rejection.
5. **One invocation:** one exact fake exchange proves admitted mapping, while recording invokers prove
   exact context forwarding and fixed handling of invalid return alternatives.

The completed package remains 643 production lines across four focused files, below the reviewed
650-line stop threshold. It has only the two approved main exported abstractions: `Executor` and
`Outcome`.

## Implemented control flow

The main path is [`Executor.Execute`](../../gateway/internal/modelturn/executor.go). Read it as an
ordered admission pipeline:

```go
raw, err := readRequestBody(body)
if err != nil || !validateStrictDocument(raw) {
	return newUncorrelatedFailure(), nil
}
root, ok := decodeObject(raw)
if !ok {
	return newUncorrelatedFailure(), nil
}
requestID, ok := safeRequestID(root)
if !ok {
	return newUncorrelatedFailure(), nil
}
request, ok := decodeWireRequest(root)
if !ok {
	return newInvalidRequestFailure(requestID), nil
}
if len(request.requiredCapabilities) != 0 {
	return newUnsupportedCapabilityFailure(requestID), nil
}
if request.modelAlias != supportedAlias {
	return newInvalidRequestFailure(requestID), nil
}
return executor.executeAdmitted(ctx, request), nil
```

The order is the safety property. A strict document may yield a safe ID before the rest of its v1
shape passes, which enables a correlated `invalid_request`. Provider work is still impossible until
all syntax, shape, capability, and alias checks return successfully.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [`Executor.Execute`](../../gateway/internal/modelturn/executor.go) | Owns the ordered zero-or-one-dispatch flow | Where can provider work first begin? |
| [`parse.go`](../../gateway/internal/modelturn/parse.go) and [`request.go`](../../gateway/internal/modelturn/request.go) | Protect bounds, ambiguity, schema shape, and correlation | Why can malformed bytes never leak an ID? |
| [`Outcome`](../../gateway/internal/modelturn/outcome.go) | Keeps valid result alternatives closed without HTTP | How does ICGT-010 distinguish them without parsing a body? |
| [Provider request](../../gateway/internal/provider/provider.go) | Receives only admitted provider-domain meaning | Why do ID and alias stop above this boundary? |
| [Semantic and invocation tests](../../gateway/internal/modelturn/executor_test.go) with the [deterministic fake](../../gateway/internal/provider/fake/fake.go) | Prove empty-script and one-exchange behavior | How are zero and exactly-one dispatch evidenced? |
| [Bounded parser tests](../../gateway/internal/modelturn/parse_test.go) | Pin exact/one-over reads and hostile JSON | Which failure occurs before any ID is trusted? |
| [Schema and fixture tests](../../gateway/internal/modelturn/request_test.go) | Pin exact v1 parity and safe correlation | Which schema-valid cases still fail semantic admission? |
| [Model-turn fixtures](../../gateway/contracts/model-turn/v1/fixtures) | Anchor language-neutral parse and shape cases | Which schema-valid cases still fail semantic admission? |

## Implementation code samples

### 1. Bound the read before parsing

[`readRequestBody`](../../gateway/internal/modelturn/parse.go) offers only the remaining capacity
needed to reach the cap plus one byte:

```go
remaining := maxRetainedBytes - len(request)
if remaining == 0 {
	return nil, errRequestBodyTooLarge
}
offered := min(remaining, len(buffer))
n, readErr := reader.Read(buffer[:offered])
if n < 0 || n > offered {
	return nil, errReaderContract
}
if n > 0 {
	needed := len(request) + n
	if needed > cap(request) {
		capacity := min(maxRetainedBytes, max(needed, cap(request)*2))
		grown := make([]byte, len(request), capacity)
		copy(grown, request)
		request = grown
	}
	request = append(request, buffer[:n]...)
	consecutiveNoReads = 0
	if len(request) > MaxRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
}
```

`n` is processed before `readErr`, which is why `(n > 0, io.EOF)` can complete an exact-limit body
and `(n > 0, another error)` still counts those bytes before discarding the prefix. The implementation
uses a 32 KiB scratch buffer; short reads may cause many calls, but no call can offer bytes past the
one-byte overflow boundary.

### 2. Make JSON unambiguous before trusting correlation

[`validateStrictDocument`](../../gateway/internal/modelturn/parse.go) first checks raw UTF-8 and
escaped surrogate pairing, then walks `json.Decoder.Token` values with `UseNumber`. Each object gets
a decoded-name set, and the walk refuses a seventeenth open container. Only an immediate EOF after
the first value proves full-document framing.

After that whole pass, [`safeRequestID`](../../gateway/internal/modelturn/request.go) applies the
independent ASCII identifier rule:

```go
func safeRequestID(root map[string]json.RawMessage) (string, bool) {
	requestID, ok := decodeString(root["request_id"])
	if !ok || !validIdentifier(requestID) {
		return "", false
	}
	return requestID, true
}
```

This small function is safe only because `Execute` calls it after the complete strict-document pass.
Calling it on a map produced from malformed or duplicate-key input would not establish that guarantee.

### 3. Allocate provider collections only after cardinality checks

[`decodeWireRequest`](../../gateway/internal/modelturn/request.go) first decodes an array into bounded
`json.RawMessage` values and checks its length. Only then does it allocate the provider-domain slice:

```go
conversationRaw, ok := decodeArray(root["conversation"])
if !ok || len(conversationRaw) < 1 || len(conversationRaw) > provider.MaxConversationMessages {
	return wireRequest{}, false
}
conversation := make([]provider.Message, len(conversationRaw))
for index, raw := range conversationRaw {
	message, valid := decodeWireMessage(raw)
	if !valid {
		return wireRequest{}, false
	}
	conversation[index] = message
}
```

Conversation and instruction order and Unicode content remain exact. Wire-only version, kind, ID,
and alias never enter `provider.Request`. The admitted capability list must be empty, so mapping passes
`nil` to `provider.NewRequest` rather than silently dropping a requirement.

### 4. Keep failure representation closed and copied

[`Outcome.FailureBody`](../../gateway/internal/modelturn/outcome.go) exposes only a body for the two
failure families and constructs fresh bytes on every call:

```go
func (outcome Outcome) FailureBody() ([]byte, bool) {
	switch outcome.kind {
	case outcomeUncorrelatedFailure:
		return []byte(uncorrelatedFailureBody), true
	case outcomeAdmissionFailure, outcomeInternalFailure:
		return correlatedFailureBody(outcome.requestID, outcome.failureCode), true
	default:
		return nil, false
	}
}
```

The zero value has no valid kind, so every accessor returns `false`. Ordinary Go errors are reserved
for nil caller-programming inputs; request and provider boundary failures remain closed outcomes.

### 5. Invoke once, then validate immediately

The only provider-work site is [`executeAdmitted`](../../gateway/internal/modelturn/executor.go):

```go
result, invokeErr := executor.invoker.Invoke(ctx, providerRequest)
if provider.ValidateInvocation(ctx, result, invokeErr) != nil {
	return newInternalFailure(request.requestID)
}
return newProviderOutcome(request.requestID, result, invokeErr)
```

There is no loop, retry, goroutine, timer, or fallback. A valid result, direct normalized failure, or
matching context sentinel is retained exactly. An invalid alternative becomes only the fixed internal
outcome. The invalid error is never formatted or unwrapped by this package.

The exact-sentinel check in
[`provider.ValidateInvocation`](../../gateway/internal/provider/provider.go) compares only with the
two fixed context sentinels before accepting a direct normalized failure:

```go
if err == context.Canceled || err == context.DeadlineExceeded {
	if ctx.Err() != err {
		return errors.New("provider invocation context termination does not match caller context")
	}
	return nil
}

failure, ok := err.(*Failure)
```

Go interface equality compares dynamic types first and compares dynamic values only when those types
are identical. A slice-backed raw error and either fixed context sentinel have different dynamic
types, so equality is safely false; an error with the sentinel's dynamic type is comparable. The
non-comparable-error test pins that this path rejects the raw error without invoking its `Error`
method. See the Go specification's
[comparison rules](https://go.dev/ref/spec#Comparison_operators).

### 6. Tests prove both sides of dispatch

[`TestExecutorRejectsSemanticPolicyBeforeDispatch`](../../gateway/internal/modelturn/executor_test.go)
uses a fresh empty fake for `tool_calls`, combined tool/unknown-alias, and unknown-alias cases. Any
unexpected call would panic, and `VerifyComplete` independently proves the empty script remains
complete. [`TestExecutorMapsOneAdmittedRequestAndPreservesResult`](../../gateway/internal/modelturn/executor_test.go)
uses exactly one exchange and verifies complete script consumption.

[`TestExecuteAdmitsExactLimitAndRejectsOneByteOver`](../../gateway/internal/modelturn/parse_test.go)
proves an otherwise valid exact-limit request reaches one invocation while one extra byte reaches
zero. [`TestExecutorReplacesInvalidInvocationAlternativesWithoutTraversal`](../../gateway/internal/modelturn/executor_test.go)
uses trap errors whose `Error` and `Unwrap` methods panic; all invalid alternatives still become the
same fixed `internal_error` after exactly one call.

## Failure scenarios to study

| Failure class | Observable result | Evidence |
| --- | --- | --- |
| Apparent ID in an oversized, malformed, duplicate-key, invalid-UTF-8, lone-surrogate, or multi-value body | Exact uncorrelated `invalid request\n`; no ID or protocol code | [`parse_test.go`](../../gateway/internal/modelturn/parse_test.go) and [`TestExecutorCorrelatesOnlyAfterTheWholeStrictDocumentIsSafe`](../../gateway/internal/modelturn/executor_test.go) |
| Strict object with safe ID but another v1 violation | Correlated fixed `invalid_request`; zero calls | [`request_test.go`](../../gateway/internal/modelturn/request_test.go) and the public correlation matrix |
| `tool_calls` plus an unknown alias | Correlated fixed `unsupported_capability`; capability wins; zero calls | Semantic-policy test and canonical failure-fixture comparison |
| Unknown alias containing sentinel text | Correlated generic `invalid_request`; supplied alias is absent; zero calls | Semantic-policy privacy assertions |
| Unreachable schema-to-domain mismatch | Correlated fixed `internal_error`; zero calls | Private `executeAdmitted` mapping-inconsistency test |
| Raw/wrapped error, typed-nil failure, fabricated cancellation, invalid result, or mixed result/error | Correlated fixed `internal_error`; exactly one call | Invalid-invocation table with `Error`/`Unwrap` traps |
| Valid result, direct failure, or matching context sentinel | Exact provider alternative and request ID retained | Result, failure, usage-presence, and context tests |

Distinct prompt, instruction, alias, capability, and reader-error sentinels are absent from every
fixed body. The package has no logging path, so rejected content is neither returned nor logged here.

## Test and validation evidence

The focused package has four review-oriented suites:

| Boundary | Test file | Important evidence |
| --- | --- | --- |
| Closed outcomes and programming errors | [`outcome_test.go`](../../gateway/internal/modelturn/outcome_test.go) | Zero value, copied bodies, exact IDs/codes, provider identity, nil inputs, typed-nil limit |
| Raw input and strict syntax | [`parse_test.go`](../../gateway/internal/modelturn/parse_test.go) | Exact 8 MiB/one-over, offered sizes, mixed reads, no progress, duplicate names, Unicode, numbers, depth |
| V1 shape and correlation | [`request_test.go`](../../gateway/internal/modelturn/request_test.go) | Every request fixture, exact field/type/bound rules, control profile, safe-ID phases, mapping copies |
| Semantic policy and one invocation | [`executor_test.go`](../../gateway/internal/modelturn/executor_test.go) | Capability precedence, empty fake, one exchange, exact context, valid identity, invalid-return replacement |

Focused tests were repeated, and `go vet` plus the race detector passed for the package. The complete
credential-free offline `./scripts/check` gate also passed, rerunning repository policy, contract
fixtures, all Go tests, vet, and race checks.

## Speed and size implications

Admission adds bounded linear work before provider dispatch. The runtime successfully obtains and
retains at most 8 MiB plus one byte and performs no retry, network call, or background work of its own.
This unit makes no benchmark or latency claim.

The 8 MiB cap affects accepted request size, not generated provider-response size. ICGT-009 does not
serialize provider results. Its uncorrelated rejection is the fixed 16-byte `invalid request\n` body;
ICGT-010 later assigns HTTP presentation.

## What changed during implementation

- The first complete production draft was 718 lines, above the story's mandatory split-review
  threshold. Work stopped at that checkpoint. Reusing the already-validated `provider.Message` and
  `provider.Instruction` values after array cardinality admission removed duplicate wire structs and
  conversion loops without weakening the provider request boundary.
- Removing one-use wrappers and error sentinels while storing only the closed failure code reduced the
  final package to 643 lines. This was a design simplification, not compressed parser formatting.
- Plain struct decoding was rejected because `encoding/json` accepts case-insensitive field matches,
  duplicate names, and repaired lone surrogates. A token preflight plus exact `json.RawMessage` maps
  keeps those decisions visible without adding a schema runtime.
- Huge exponents such as `1e999999` are syntactically valid JSON. `UseNumber` preserves that syntax
  without float conversion; request shape rejects the number only when it appears where v1 does not
  allow it.
- Safe ID recovery remains a separate phase after the whole strict document and before complete v1
  shape admission. That phase split makes wrong-version and unknown-field requests correlatable while
  malformed or ambiguous documents remain uncorrelated.
- The admitted capability list is necessarily empty. Mapping passes no capabilities to
  `provider.NewRequest`; it never silently discards a caller requirement because the semantic gate
  already rejected every nonempty schema-valid list.
- Independent review initially proposed a comparability guard around the context-sentinel equality.
  Follow-up review applied the complete Go interface-equality rule: the dynamic types differ for a
  slice-backed raw error, while the sentinel's own dynamic type is comparable. The redundant guard
  was removed, and the non-comparable-error test remains as direct evidence that rejection neither
  panics nor calls `Error`.
- Parser review found that default `append` growth could retain backing capacity beyond the exact
  byte claim. The reader now grows capacity explicitly up to the cap plus one, and its test checks
  both logical length and backing capacity.
- Adversarial test review made mixed data/error ordering and consecutive no-progress reset observable
  with overflow-plus-error and 99-stalls/progress/99-stalls cases.
- A read-only home cache initially blocked local Go commands in this WSL workspace. Validation uses
  a persistent `/tmp` Go build cache; this changed the command environment, not repository behavior.
- No dependency, `go.sum`, `go.work`, visual lesson, HTTP type, provider SDK, goroutine, timer, retry,
  queue, logging path, filesystem dependency, or network behavior was introduced.

## Production expansion

### Example production scenario

A deployed gateway may have many aliases, changing model availability, authenticated callers,
configurable request limits, and multiple provider adapters. That needs a reviewed configuration
authority and operational visibility, not a larger conditional hidden inside this learning unit.

### Representative capabilities and tools

- Go [`io.LimitReader`](https://pkg.go.dev/io#LimitReader) demonstrates a stable bounded-reader
  primitive. The local loop stays explicit because it also owns mixed data/error and no-progress
  behavior.
- Go [`net/http.MaxBytesReader`](https://pkg.go.dev/net/http#MaxBytesReader) is a future HTTP-boundary
  comparison for ICGT-010, not an ICGT-009 dependency.
- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) defines JSON syntax and explains duplicate-name
  and unpaired-surrogate interoperability risks.
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12) remains the language-neutral
  public artifact vocabulary; the runtime does not load a schema engine.

### Local versus production

| Dimension | ICGT-009 | Production expansion |
| --- | --- | --- |
| Body policy | One fixed 8 MiB cap | Reviewed profile or tenant-aware limits |
| Routing | One fixed `learning-text` alias | Versioned immutable routing snapshots |
| Validation | Small v1-specific standard-library code | Audited general-purpose validation |
| Security | Content-safe local admission | Authentication, authorization, and abuse controls |
| Operations | Deterministic offline tests | Metrics, alerts, rollout, and capacity evidence |

### Trade-offs and graduation signals

The local design minimizes dependencies and keeps each admission decision inspectable. A general
validator or routing subsystem adds upgrade, configuration, and operational ownership. Graduation is
justified when another protocol version, multiple real aliases, measured validation cost, or a
non-loopback authenticated deployment makes the small fixed implementation inadequate.

## Practical exercises

- Build an exact-limit request on paper, add one byte, and predict where dispatch stops.
- Compare `(n > 0, io.EOF)`, `(n > 0, another error)`, and 100 `(0, nil)` reads.
- Classify malformed JSON, unknown alias, declared `tool_calls`, and provider failure by owner.
- Trace which request fields stay at admission and which enter `provider.Request`.
- Classify a valid provider failure versus a raw provider error after the one invocation.
- Compare a fixed alias conditional with a future immutable routing table without implementing it.

## Key takeaways

- Resource bounds must constrain the read before parsing or allocation grows.
- Strict JSON, schema validation, semantic admission, and provider dispatch are separate stages.
- A request ID becomes correlation data only after the whole document proves it is safe.
- Unsupported capability rejection occurs before routing and before provider work.
- Passing admission permits exactly one provider invocation, never an implicit retry.
- The provider's result/error alternative is untrusted until the port contract validates it.
- HTTP presentation remains separate from provider-neutral execution.

## Glossary

- **Admission:** checks that must pass before provider work may begin.
- **Raw-body cap:** the maximum encoded byte count accepted before JSON parsing.
- **Strict parse:** decoding that rejects ambiguous or invalid JSON representations.
- **Safe request ID:** a fully parsed identifier satisfying the v1 correlation rules.
- **Correlated failure:** a failure envelope that safely echoes the admitted request ID.
- **Uncorrelated rejection:** a fixed bounded response used when no safe ID exists.
- **Semantic admission:** policy checks after schema conformance, such as capability and alias support.
- **Dispatch:** the point at which the provider port is invoked.
- **Closed outcome:** a result whose valid alternatives are fixed and whose internal state cannot be
  assembled inconsistently by outside callers.
- **Post-dispatch internal failure:** a fixed safe result used after one provider call returns an
  invalid alternative; it is not an admission rejection.
- **Interface equality:** Go first compares dynamic types. When they are identical, their dynamic
  values must be comparable or equality panics; different dynamic types compare unequal without
  comparing the dynamic values.

See the shared [glossary](../glossary.md) for repository-wide terms.

## Teach-back questions

1. Why must ICGT-009 read through an 8 MiB plus one-byte bound before parsing instead of checking the size after reading everything?
2. Which rejected documents may safely receive a correlated failure, and why does `tool_calls` take precedence over an unknown alias?
3. Why must ICGT-009 validate the provider's return, and how does a valid provider outcome differ from an invalid post-dispatch alternative?

## Further reading

- [ICGT-009 delivery contract](../../user-stories/icgt-009-admit-and-execute-model-turn.md)
- [ADR 0003](../adr/0003-fastgate-api-surface.md)
- [Model-turn v1 contract](../../gateway/contracts/model-turn/v1/README.md)
- [ICGT-007 provider lesson](icgt-007-provider-contracts.md)
- [ICGT-008 deterministic fake lesson](icgt-008-basic-deterministic-fake.md)
- [RFC 8259: JSON](https://www.rfc-editor.org/rfc/rfc8259)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [Go `io` package](https://pkg.go.dev/io)
- [Go `encoding/json` package](https://pkg.go.dev/encoding/json)
- [Go `unicode/utf8` package](https://pkg.go.dev/unicode/utf8)
