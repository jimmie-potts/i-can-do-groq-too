# ADR 0002: Use a fake first and OpenAI as the first live provider

- **Status:** Accepted
- **Date:** 2026-07-31
- **Scope:** FastGate provider implementation order

## Context

The source conversation contains two statements that can appear contradictory: start FastGate from
a known OpenAI workload, and create a deterministic fake before relying on an external provider.
The Code Assist Harness implementation has already demonstrated that a strict fake makes streaming,
failure, and cancellation behavior testable without credentials or network variability.

## Decision

Use the following sequence:

1. Use the FastGate-owned client protocol selected in ADR 0003.
2. Define the smallest FastGate-owned provider contracts required by that choice.
3. Implement a basic non-streaming deterministic fake upstream.
4. Prove one external request and normalized transport failures against the basic fake.
5. Define the stream grammar, then extend the fake with deterministic streaming and cancellation
   controls.
6. Add streaming, cancellation, bounds, and basic telemetry against those deterministic scenarios.
7. Add OpenAI as the first opt-in live provider and performance/behavior baseline.
8. Add Groq for limited, evaluated task categories.
9. Add self-hosted providers only when a specific runtime or caching experiment requires them.

The fake is the first provider implementation. OpenAI is the first live provider. Live credentials
never select a test implicitly, and default validation remains offline.

## Consequences

- Rare failures and races are reproducible before billable calls exist.
- OpenAI integration is measured against a stable local contract rather than defining the domain.
- Groq compatibility differences become visible adapter and capability decisions.
- Some live-provider surprises will still require opt-in smoke evidence; the fake cannot prove an
  external service's current behavior.

## Rejected alternatives

### Add Groq first because of the repository name

Rejected because it would make one provider's current behavior the architecture before a portable
contract and baseline exist.

### Add OpenAI SDK types throughout FastGate

Rejected because other providers would then require translating into OpenAI objects instead of a
small FastGate-owned model.
