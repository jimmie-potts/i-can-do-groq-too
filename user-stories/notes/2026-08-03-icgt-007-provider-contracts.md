# ICGT-007 provider-contract implementation note

## Outcome

ICGT-007 introduces `gateway/internal/provider` as FastGate's provider-neutral, non-streaming
southbound boundary. It defines bounded immutable request and result values, optional usage,
content-safe normalized failures, one synchronous context-aware `Invoker`, and executable outcome
validation. It introduces no adapter, provider call, endpoint, routing, retry, telemetry, goroutine,
or visual lesson.

## Accepted ownership decisions

- `Request` contains ordered conversation, ordered generic instructions, and selected required
  capabilities only.
- Model-turn wire framing and correlation (`version`, `kind`, `request_id`), logical routing
  (`model_alias`, provider IDs), credentials, endpoints, and provider SDK values stay outside the
  provider port.
- `tool_calls` is bounded provider-neutral requirement data here. ICGT-009 owns the mandatory v1
  admission rejection; later stories own discovery, support, emulation, and routing.
- `Invoker.Invoke(context.Context, Request) (Result, error)` is synchronous and non-streaming.
- Valid outcomes are a valid result plus `nil`, a zero result plus a direct `*Failure`, or a zero
  result plus the exact caller-context cancellation/deadline sentinel.
- Wrapped failures, wrapped context errors, raw provider errors, fabricated cancellation,
  typed-nil failures, and simultaneous result/error values are contract violations.
- Adapter-returnable codes are `authentication_failed`, `rate_limited`, `request_rejected`,
  `unavailable`, `invalid_response`, `unsupported_upstream_output`, and `internal_error`.
  Admission-owned `invalid_request` and `unsupported_capability` are rejected.
- Success and failure usage preserve absent versus observed zero and enforce the model-turn v1
  JavaScript-safe integer maximum. Usage is not billing proof or retry authority.

## Review-driven changes

The initial constructor cloned caller slices before cardinality validation. Review identified that
an already invalid input could force allocation proportional to an unbounded slice. `NewRequest`
now validates the three cardinalities first, then clones, then validates the owned contents.

The initial text helper scanned each string twice. It now performs one UTF-8 decoding pass, stops at
the first invalid sequence or scalar above the maximum, and applies the instruction-source control
rule during that pass.

The initial outcome helper accepted `context.Canceled` and `context.DeadlineExceeded` without
observing the caller context. It now receives that context and requires exact agreement with
`ctx.Err()`. A direct type assertion rejects every non-`*Failure` error without invoking
provider-controlled wrapping behavior.

The initial schema parity helper treated enum order as meaningful. It now compares sorted copies,
matching JSON Schema's set semantics. The repository PR-review checklist records these reusable
missed-case classes.

The final privacy review replaced a denylist of raw-detail field types with the exact allowed
`Failure` field names and types. A recording raw error also proves rejection does not execute its
`Error`, `Unwrap`, or `As` methods. The same review corrected stale model-turn documentation:
ICGT-010 copies provider-origin code, retryability, and usage rather than reinterpreting them.

## Evidence

- Production contract: [`gateway/internal/provider/provider.go`](../../gateway/internal/provider/provider.go)
- Focused tests: [`gateway/internal/provider/provider_test.go`](../../gateway/internal/provider/provider_test.go)
- Learning companion: [`docs/lessons/icgt-007-provider-contracts.md`](../../docs/lessons/icgt-007-provider-contracts.md)
- Review regression checklist: [`docs/pr-review-checklist.md`](../../docs/pr-review-checklist.md)

The production file stays within the story heuristic at 323 nonblank, non-comment lines. It adds one
domain value family and one behavior interface. Tests cover ownership, all duplicated schema bounds,
Unicode scalar and control rules, optional usage on success/failure, exact failure ownership, and
the complete terminal alternative matrix.

Validation:

```text
GOCACHE=/tmp/icgt-go-build go test ./gateway/internal/provider
./scripts/check
```

Both commands completed successfully after review fixes. The temporary Go build cache is an
environment workaround only and is not repository configuration.

## Human review checkpoint

Personally trace `NewRequest` through `Invoker.Invoke` and `ValidateInvocation`. Then read
`TestRequestPreservesOrderAndOwnsItsSlices`,
`TestValidateInvocationAllowsOnlyOneReviewedAlternative`, and
`TestProviderBoundsMatchModelTurnV1Schemas`.

The reviewer should be able to explain this invariant: an invoked provider receives only bounded,
owned FastGate values and returns exactly one bounded, content-safe terminal alternative.

## ICGT-008 handoff

ICGT-008 may implement only the basic deterministic fake for this port. It must match every request
field exactly, return scripted results or failures, use safe mismatch diagnostics, and prove complete
script consumption. It must not add HTTP, SDKs, routing, retries, capability admission, streaming,
wall-clock timing, or live-provider behavior.
