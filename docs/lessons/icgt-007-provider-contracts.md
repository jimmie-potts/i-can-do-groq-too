# ICGT-007 lesson: Provider contracts as a stable seam

- **Unit:** ICGT-007
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Provider-domain values and port implemented; adapters and inference execution deferred
- **Story:** [ICGT-007](../../user-stories/icgt-007-define-provider-contracts.md)
- **Review priority:** High
- **Visual companion:** Not created; the requested Markdown lesson is sufficient for this unit
- **Related architecture:** [Architecture](../architecture.md) and
  [ADR 0002](../adr/0002-fake-first-openai-first-live.md)

## Quick summary

ICGT-007 adds the internal language FastGate will use when it asks any upstream provider for one
non-streaming model turn. The new [`provider` package](../../gateway/internal/provider/provider.go)
contains bounded request, result, usage, and failure values plus one synchronous `Invoker` interface.
ICGT-007 itself performs no inference. ICGT-008 now proves the interface with a deterministic fake.

## Learning objectives

After reviewing this unit, you should be able to:

- distinguish FastGate's northbound wire protocol from its southbound provider port;
- explain why constructors copy and validate bounded values before an adapter sees them;
- enumerate the three valid `Invoke` outcome shapes;
- explain why normalized failures intentionally discard raw provider detail; and
- compare this small synchronous port with the harness's richer asynchronous provider lifecycle.

## Foundation: port versus adapter

A **port** is an interface owned by the application. It says what FastGate needs from an upstream.
An **adapter** is code that translates a fake, OpenAI, Groq, or another provider into that interface.
If FastGate copied the first vendor's SDK types into the port, every policy and transport caller
would become coupled to that vendor.

The two FastGate boundaries also have different jobs:

```text
northbound client JSON -> future admission/mapping -> provider.Request -> future adapter
```

The public model-turn document needs `version`, `kind`, `request_id`, and `model_alias` for wire
framing, correlation, and routing. The internal provider request does not. It carries only the
ordered conversation, generic instructions, and required capabilities needed after admission and
route selection. Credentials, endpoints, provider model IDs, SDK values, and raw errors stay in a
concrete adapter.

## The request owns bounded copies

The important construction path is `NewRequest` in
[`provider.go`](../../gateway/internal/provider/provider.go). It first rejects oversized slice
cardinalities, then copies the accepted slices, then validates their contents:

```go
if err := validateRequestCardinalities(
	len(conversation),
	len(instructions),
	len(requiredCapabilities),
); err != nil {
	return Request{}, err
}
request := Request{
	conversation:         slices.Clone(conversation),
	instructions:         slices.Clone(instructions),
	requiredCapabilities: slices.Clone(requiredCapabilities),
}
```

The order matters. Checking lengths before `slices.Clone` prevents an invalid oversized caller slice
from forcing an equally oversized allocation. Copying after that check means later caller mutation
cannot alter what an adapter sees. `Conversation`, `Instructions`, and `RequiredCapabilities` also
return copies, so a reader cannot mutate the stored request through an accessor.

Text validation decodes one Unicode scalar at a time. It rejects invalid UTF-8, stops as soon as the
maximum is exceeded, and applies the model-turn v1 control-safe rule only to instruction sources.
Conversation, instruction content, and output text may contain newlines or tabs because their
schemas allow them.

`tool_calls` is valid **requirement data** in this package. It is not evidence that a provider
supports tools, and this unit does not admit, reject, discover, emulate, or route that capability.
ICGT-009 owns v1's mandatory pre-invocation rejection.

## One method and three valid outcomes

The complete behavior port is deliberately small:

```go
type Invoker interface {
	Invoke(context.Context, Request) (Result, error)
}
```

The caller supplies the context so cancellation or a deadline can propagate into the future
adapter's blocking work. This interface is synchronous: the call returns once with exactly one of
these alternatives.

| Outcome | Result | Error |
| --- | --- | --- |
| Completion | valid bounded `Result` | `nil` |
| Provider failure | zero `Result` | direct project-owned `*Failure` |
| Caller termination | zero `Result` | exact `context.Canceled` or `context.DeadlineExceeded`, matching `ctx.Err()` |

`ValidateInvocation` makes that review rule executable. In particular, an adapter cannot invent a
cancellation while the caller context is still active:

```go
if err == context.Canceled || err == context.DeadlineExceeded {
	if ctx.Err() != err {
		return errors.New("provider invocation context termination does not match caller context")
	}
	return nil
}

failure, ok := err.(*Failure)
if !ok {
	return errors.New(
		"provider invocation error must be a direct normalized failure or context termination",
	)
}
```

The direct type assertion is intentional. It rejects wrapped failures and raw provider errors
without traversing provider-controlled `Unwrap` or `As` behavior. A nonzero result plus any error is
also rejected, so callers never have to guess which half of an ambiguous outcome to trust.

## Safe failure normalization

An adapter may return exactly seven provider-owned categories:

- `authentication_failed`
- `rate_limited`
- `request_rejected`
- `unavailable`
- `invalid_response`
- `unsupported_upstream_output`
- `internal_error`

`invalid_request` and `unsupported_capability` belong to admission above the provider port. If an
adapter could return them after invocation, FastGate could no longer tell whether paid work had
already started.

`Failure` keeps its fields private and its `Error` text fixed. It has no raw `error`, arbitrary
message, response body, header, endpoint, or credential field. That loss of detail is deliberate:
adapter-local diagnostics may record safe bounded operational facts later, while the stable domain
outcome cannot accidentally disclose provider-authored content.

Both a result and a failure can carry optional usage. `(Usage{}, false)` means no observation;
`(Usage{}, true)` means an observed zero. The distinction is preserved, but neither case proves a
bill, authorizes a retry, or confirms remote cleanup.

## Personal code review map

Review these focused paths in order:

1. [`provider.go`](../../gateway/internal/provider/provider.go): `NewRequest`, `validateText`,
   `Invoker`, and `ValidateInvocation` are the complete production control path.
2. [`provider_test.go`](../../gateway/internal/provider/provider_test.go):
   `TestRequestPreservesOrderAndOwnsItsSlices` and
   `TestValidateInvocationAllowsOnlyOneReviewedAlternative` prove ownership and outcome selection.
3. In the same test file, `TestFailureCodesAreProviderOwnedAndContentFree` and
   `TestProviderBoundsMatchModelTurnV1Schemas` prove privacy, ownership boundaries, and parity with
   the already accepted wire bounds.

The invariant to explain after review is: **an invoked provider sees only bounded FastGate-owned
values and can return only one bounded, unambiguous, content-safe outcome.**

## Meaningful failure paths

The tests deliberately challenge more than happy-path examples:

- oversized collections are rejected before copying;
- bounds count Unicode scalars rather than UTF-8 bytes;
- invalid UTF-8, including bytes that encode a surrogate, is rejected;
- all C0, DEL/C1, and U+2028/U+2029 instruction-source controls are rejected while adjacent
  allowed values remain accepted;
- caller-owned inputs and accessor outputs cannot mutate a stored request;
- absent, observed-zero, maximum, negative, and above-maximum usage are checked on success and
  failure;
- admission-owned and vendor-specific failure codes are rejected;
- raw errors, wrapped failures, wrapped context errors, fabricated cancellation, typed-nil
  failures, and simultaneous result/error outcomes are rejected; and
- a schema-parity test catches drift in duplicated bounds, enums, patterns, and usage limits while
  treating JSON Schema enum ordering as semantically irrelevant.

## What changed during implementation

The first implementation draft copied request slices before checking their maximum cardinalities.
Review caught that an invalid caller could force an unnecessary unbounded copy, so construction now
preflights the three lengths before allocating.

The first outcome validator accepted the two context sentinels without checking the supplied
context. Review showed that an adapter could fabricate cancellation while the caller remained
active. `ValidateInvocation` now receives the caller context and requires exact agreement with
`ctx.Err()`; tests include cancellation with a custom cause and mismatched or active contexts.

A separate parity review also caught an order-sensitive comparison of JSON Schema enums. Because
enum order has no semantic meaning, the helper now compares sorted copies as sets. These findings
are durable review questions in the repository checklist.

The final privacy review replaced a field-type denylist with an exact four-field `Failure`
inventory and added a recording raw error. The regression proves rejection does not call its
`Error`, `Unwrap`, or `As` methods. This is stronger than assuming a short list of unsafe container
types will never grow.

## How this differs from Code Assist Harness

The harness already needs a lazy asynchronous operation, a single-consumer event stream, explicit
awaited cancellation, cleanup confirmation, tool-call events, and workflow-safe terminal handling.
FastGate's first provider port needs none of those yet because model-turn v1 is non-streaming and no
adapter exists. Copying the harness lifecycle now would create states this repository cannot test.

Later FastGate streaming stories may introduce an asynchronous operation and cleanup barrier when
the SSE grammar and deterministic fake can exercise ownership, backpressure, cancellation, and
cleanup. That later contract can learn from the harness without sharing its Python types or moving
workflow authority into FastGate.

## Speed and size implications

The contract adds validation and defensive slice copies before an adapter call. Those costs are
linear in already bounded inputs and stop at the first exceeded text limit. The work is bounded and
occurs before any future network call; ICGT-007 includes no benchmark and makes no latency claim.

The bounds limit request, output, and usage values; they do not add fields to the public response.
Optional usage retains one small fixed-size Go value internally. Streaming response speed, HTTP
payload size, provider latency, retries, and buffering are all deferred.

## Production expansion

A real adapter will translate these values into its SDK, disable or bound SDK retries according to
FastGate policy, propagate the context, normalize unsafe errors, and close response resources. The
fake in ICGT-008 comes first so exact matching and safe failures can be proved without network or
wall-clock uncertainty. Capability admission, routing, HTTP mapping, telemetry, streaming, and live
providers each remain separate reviewed units.

## Validation evidence

The focused package tests and complete offline repository gate pass without an SDK, credentials, or
network:

```bash
go test ./gateway/internal/provider
./scripts/check
```

## Practical exercises

- Add a table row on paper for a simultaneous result and error, then explain why it is rejected.
- Classify `request_id`, `model_alias`, an API key, and `tool_calls` as wire, routing,
  adapter-configuration, or provider-request data.
- Trace the difference between absent usage and observed-zero usage through `NewFailure` and
  `Failure.Usage`.
- Sketch the minimum adapter-local information needed to diagnose a timeout without retaining a
  prompt or raw provider body.

## Key takeaways

- FastGate owns the provider vocabulary; vendors own translation details.
- The northbound wire document and southbound provider request have different responsibilities.
- Bounds must be enforced before allocation or invocation, not merely documented.
- Context termination is accepted only when it matches the caller's real context state.
- Normalized failures trade unsafe detail for stability and privacy.
- Optional usage is evidence to preserve, not retry authority or billing proof.

## Glossary

- **Bounded invocation:** one context-aware call with one reviewed terminal alternative.
- **Provider port:** the project-owned interface and values implemented by fake and live adapters.
- **Provider adapter:** provider-specific translation, transport, resource, and error ownership.
- **Normalization:** conversion of unstable external outcomes into bounded project-owned values.
- **Defensive copy:** an owned copy that prevents later caller mutation from changing stored state.

## Teach-back questions

1. Why are `request_id` and `model_alias` absent from `provider.Request` even though model-turn v1 requires them?
2. Why must `ValidateInvocation` compare a cancellation error with the supplied caller context?
3. Why can a `Failure` preserve optional usage but not a raw provider error message?

## Further reading

- [Go `context` package](https://pkg.go.dev/context)
- [Go `slices` package](https://pkg.go.dev/slices)
- [ICGT-007 delivery contract](../../user-stories/icgt-007-define-provider-contracts.md)
- [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md)
- [Code Assist Harness boundary lesson](icgt-001-repository-boundaries.md)
