# ICGT-009 lesson: Bounded model-turn admission before provider dispatch

- **Unit:** ICGT-009
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; ICGT-008 is delivered, but no ICGT-009 runtime code or test
  evidence exists yet
- **Story:** [ICGT-009](../../user-stories/icgt-009-admit-and-execute-model-turn.md)
- **Review priority:** High
- **Visual companion:** Not required; no visual is planned for this unit
- **Related architecture:** [ADR 0003](../adr/0003-fastgate-api-surface.md),
  [model-turn v1](../../gateway/contracts/model-turn/v1/README.md), and
  [provider contracts](../../gateway/internal/provider/provider.go)

> This lesson describes accepted design and planned behavior. Every future path and code sample is
> labeled as planned or pseudocode until implementation and tests exist.

## Quick summary

ICGT-009 will connect the model-turn v1 wire meaning to the existing provider-neutral port without
exposing an HTTP route. It will accept a bounded `io.Reader`, strictly parse and validate one request,
reject unsupported requirements or aliases before dispatch, map an admitted request into
`provider.Request`, and invoke one injected `provider.Invoker`. The deterministic fake proves the
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

ICGT-009 becomes that policy boundary. ICGT-010 can then concentrate on HTTP binding and provider
outcome presentation instead of mixing parsing, correlation, routing, dispatch, and transport in one
handler.

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

The unbounded allocation already happened before the check. The planned path instead retains at most
8 MiB plus one byte. Each read receives only the remaining buffer capacity, but a short-reading source
may be offered multiple buffers over time; the meaningful bound is bytes successfully obtained and
retained, not the sum of every buffer length offered. The extra retained byte is evidence of overflow;
it is never part of an admitted document.

Go readers may return data and an error together. The planned loop counts every `n > 0` before
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
strict-v1 rule, including duplicate-name and invalid Unicode handling. The repository therefore plans
small, inspectable request-specific checks using stable standard-library primitives. It will not
enable the experimental `encoding/json/v2` package or add a general schema dependency for this unit.

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

| Input state | Safe ID? | Planned outcome |
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

The planned package exposes two main types and these entry points:

```go
NewExecutor(provider.Invoker) (*Executor, error)
(*Executor).Execute(context.Context, io.Reader) (Outcome, error)
```

**PLANNED SIGNATURES — the package does not exist yet.** `Outcome` is a concrete value type with
private fields. Every `Outcome` returned with a nil ordinary error has exactly one of four
alternatives:

1. an uncorrelated admission failure with exact `invalid request\n` bytes;
2. a correlated admission failure with a fixed schema-valid body;
3. an admitted provider outcome with the safe request ID and exact valid result or error; or
4. a correlated fixed internal failure after mapping inconsistency or an invalid provider return.

The planned accessors are `RequestID() (string, bool)`, `FailureBody() ([]byte, bool)`,
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

The planned ownership rules are:

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
- an invalid provider alternative becomes a fixed internal outcome without unsafe traversal; and
- no goroutine, timer, retry, queue, SDK, credential, network call, or routing registry is
  introduced.

HTTP method, media type, status, headers, listener binding, request-body closure, authentication,
provider-outcome serialization, streaming, and cancellation behavior remain deferred.

## Practical walkthrough

Implementation will proceed in five review pauses:

1. **Outcome boundary:** define the exact API, closed outcomes, nil behavior, and fixed safe failures.
2. **Bounded syntax:** implement the limit-plus-one read, no-progress behavior, and strict JSON
   preflight.
3. **Schema and correlation:** enforce exact v1 fields and bounds, then recover safe IDs and allocate
   admitted provider-domain slices.
4. **Semantic gates:** reject capability before alias and prove zero dispatch with an empty fake.
5. **One invocation:** map into `provider.Request`, validate the only return, preserve valid outcomes,
   and fail closed on a deliberately invalid recording invoker.

Each pause gets focused tests before the next begins. The complete offline gate and separate review
still run after the whole story is assembled.

## Planned control flow

**PSEUDOCODE ONLY — ICGT-009 is not implemented:**

```text
execute(ctx, reader):
    raw = bounded_read(reader, 8 MiB + 1 byte)
    if read cannot produce an admissible body:
        return uncorrelated("invalid request\\n")

    document = strict_parse(raw, maximum_container_depth = 16)
    if strict parsing fails:
        return uncorrelated("invalid request\\n")

    request_id = recover_safe_request_id(document)
    if request_id is unavailable:
        return uncorrelated("invalid request\\n")

    if document does not satisfy model-turn v1:
        return correlated invalid_request(request_id)

    if document requires tool_calls:
        return correlated unsupported_capability(request_id)

    if document.model_alias != "learning-text":
        return correlated invalid_request(request_id)

    provider_request = build_provider_request(document)
    if mapping unexpectedly fails:
        return correlated internal_error(request_id)

    result, provider_error = invoke_provider_once(ctx, provider_request)
    if validate_invocation(ctx, result, provider_error) fails:
        return correlated internal_error(request_id)

    return provider_outcome(request_id, result, provider_error) unchanged
```

The completed lesson must replace this pseudocode with focused exact source excerpts after the code
exists and validation passes.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Future bounded admission entry point | Owns the ordered zero-or-one-dispatch flow | Where can provider work first begin? |
| Future strict parser and request decoder | Protects bounds, ambiguity, and correlation | Why can malformed bytes never leak an ID? |
| [Provider request](../../gateway/internal/provider/provider.go) | Receives only admitted provider-domain meaning | Why do ID and alias stop above this boundary? |
| [Deterministic fake](../../gateway/internal/provider/fake/fake.go) and future admission tests | Prove empty-script and one-exchange behavior | How are zero and exactly-one dispatch evidenced? |
| Future recording-invoker tests | Prove exact context and invalid-return handling | Where is the port's return validated without leaking it? |
| [Model-turn fixtures](../../gateway/contracts/model-turn/v1/fixtures) | Anchor language-neutral parse and shape cases | Which schema-valid cases still fail semantic admission? |

Replace every future-path placeholder with an exact link after implementation.

## Implementation code samples

No implementation excerpt exists yet. The completed lesson must include:

- the bounded-read-to-dispatch production path;
- the closed outcome accessors and fixed nil behavior;
- the safe-ID recovery decision;
- the `tool_calls` zero-dispatch failure path; and
- focused tests proving exact-limit acceptance, one-over rejection, one admitted call, and one invalid
  provider return becoming a fixed internal outcome.

## Failure scenarios to study

- A body contains an apparent request ID but exceeds 8 MiB.
- Duplicate `request_id` keys create two possible correlation values.
- Invalid UTF-8 or a lone surrogate would otherwise be repaired or interpreted inconsistently.
- One valid JSON value is followed by another value or non-whitespace.
- Nesting exceeds 16 containers before schema validation.
- The whole document parses and has a safe ID, but violates another v1 rule.
- `tool_calls` and an unknown alias appear together.
- An unknown alias is copied into a diagnostic.
- A rejected request invokes the fake despite returning an admission failure.
- An admitted request invokes the fake twice.
- Provider failure or usage information is reinterpreted instead of preserved unchanged.
- An invoker returns a raw secret-bearing error, typed-nil normalized failure, fabricated cancellation,
  or result and error together.
- Validation formats or retains that invalid return instead of replacing it with the fixed internal
  failure.

Every rejection test should use sentinel prompt, instruction, alias, and reader-error text and prove
that diagnostics and fixed failures contain none of it.

## Speed and size implications

Admission adds bounded linear work before provider dispatch. The runtime successfully obtains and
retains at most 8 MiB plus one byte and performs no retry, network call, or background work of its own.
This planned unit makes no benchmark or latency claim.

The 8 MiB cap affects accepted request size, not generated provider-response size. ICGT-009 does not
serialize provider results. Its uncorrelated rejection is the fixed 16-byte `invalid request\n` body;
ICGT-010 later assigns HTTP presentation.

## What changed during implementation

No implementation evidence exists yet. Replace this section with observed constraints, failed
assumptions, review findings, and resulting design changes after implementation.

The accepted starting decisions are:

- exactly 8 MiB for the raw body;
- `io.Reader` plus a stable-standard-library, request-specific strict decoder;
- maximum JSON container depth of 16;
- safe request ID recovery only after complete strict parsing;
- fixed `invalid request\n` when no safe ID exists;
- capability admission before alias selection; and
- one fixed alias, `learning-text`, admitting the injected invoker;
- a two-type executor/outcome API that keeps HTTP presentation deferred;
- fixed nil-interface programming errors without reflection-based typed-nil detection;
- immediate `provider.ValidateInvocation` with a fixed safe internal outcome for invalid returns; and
- one coherent 450-650-production-line unit, with five review pauses and a mandatory split review
  above 650 lines or two main exported abstractions.

## Production expansion

### Example production scenario

A deployed gateway may have many aliases, changing model availability, authenticated callers,
configurable request limits, and multiple provider adapters. That needs a reviewed configuration
authority and operational visibility, not a larger conditional hidden inside this learning unit.

### Representative capabilities and tools

- Go [`io.LimitReader`](https://pkg.go.dev/io#LimitReader) demonstrates a stable bounded-reader
  primitive; the implementation still needs the extra-byte overflow check.
- Go [`net/http.MaxBytesReader`](https://pkg.go.dev/net/http#MaxBytesReader) is a future HTTP-boundary
  comparison for ICGT-010, not an ICGT-009 dependency.
- [RFC 8259](https://www.rfc-editor.org/rfc/rfc8259) defines JSON syntax and explains duplicate-name
  and unpaired-surrogate interoperability risks.
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12) remains the language-neutral
  public artifact vocabulary; production ICGT-009 will not load a schema engine.

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
