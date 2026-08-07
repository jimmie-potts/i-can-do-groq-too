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
5. Bind a separate stateless fixed-output demo to the reviewed bounded loopback-only runtime while
   retaining the strict fake as the test oracle.
6. Define the stream grammar, then extend the fake with deterministic streaming and cancellation
   controls.
7. Add streaming, cancellation, bounds, and basic telemetry against those deterministic scenarios.
8. Add OpenAI as the first opt-in live provider through non-runnable test seams.
9. Select and implement the reviewed runnable-live-provider security profile, then establish the
   OpenAI performance/behavior baseline through FastGate.
10. Add Groq for limited, evaluated task categories.
11. Add self-hosted providers only when a specific runtime or caching experiment requires them.

The fake is the first provider implementation. OpenAI is the first live provider. Live credentials
never select a test implicitly, and default validation remains offline.

The finite scripted fake remains an assertion-oriented, single-owner test oracle; it is not the
default long-lived runtime dependency. [ADR 0005](0005-local-demo-runtime-profile.md) selects a
separate stateless fixed-output demo for the planned ICGT-011 local runnable profile. That demo makes
no network call and is not a live provider, so it preserves this sequence while allowing concurrent
unscripted local requests without weakening the strict fake.

## Consequences

- Rare failures and races are reproducible before billable calls exist.
- OpenAI integration is measured against a stable local contract rather than defining the domain.
- Groq compatibility differences become visible adapter and capability decisions.
- Some live-provider surprises will still require opt-in smoke evidence; the fake cannot prove an
  external service's current behavior.
- The local demo proves composition and bounded service behavior only; it does not prove model
  quality, provider compatibility, or live-network behavior.

## Rejected alternatives

### Add Groq first because of the repository name

Rejected because it would make one provider's current behavior the architecture before a portable
contract and baseline exist.

### Add OpenAI SDK types throughout FastGate

Rejected because other providers would then require translating into OpenAI objects instead of a
small FastGate-owned model.
