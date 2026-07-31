# ICGT-007 - Define FastGate provider contracts

- **Status:** Planned
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-006
- **Lesson:** [Provider contracts](../docs/lessons/icgt-007-provider-contracts.md)
- **Review priority:** High

## User story

> As a gateway developer, I want a small FastGate-owned provider port so that fake, OpenAI, Groq,
> and later self-hosted adapters cannot leak vendor types into gateway policy or transport code.

## Scope

- Define the minimum provider-neutral request and non-streaming result required by accepted ADR
  0003.
- Define enumerated normalized failure categories and retryability observations with no arbitrary
  upstream-authored message field.
- Define explicit provider capability values required by the modeled request.
- Define a lazy operation/cleanup contract suitable for later streaming and cancellation without
  implementing those behaviors prematurely.
- Add compile-time/interface and validation tests with no SDK or network dependency.

## Locked behavior

- Domain contracts contain no OpenAI, Groq, HTTP, SDK, credential, endpoint, tenant-policy, or raw
  error type.
- Provider capabilities describe tested behavior, not marketing compatibility.
- A missing required capability is rejected before an adapter starts paid work.
- Failure values contain project-owned enumerated codes and retryability only. A separate local
  presentation mapping owns fixed safe messages.

## Acceptance criteria

1. One small interface captures the behavior the next fake story must implement.
2. Requests and results validate bounds and immutable ownership expectations.
3. Capability requirements and absence behavior are explicit and tested.
4. Normalized failure values have no arbitrary string field that could carry raw provider
   exceptions, response bodies, headers, or credentials.
5. Construction remains network-free and lazy.
6. Source-policy tests reject provider SDK imports from provider-neutral packages when a concrete
   adapter is eventually added.
7. The lesson compares this port with the harness port without copying its Python types blindly.

## Human review checkpoint

- **Production path:** Request validation and provider operation construction.
- **Failure/test path:** Missing capability and unsafe/unbounded failure rejection.
- **Invariant:** FastGate domain code depends on its own vocabulary, never a vendor vocabulary.
- **Deferred:** Streaming event grammar, retries, routing, and live providers.

## Validation

- Focused Go domain/interface tests.
- Negative source or dependency-boundary probe.
- `./scripts/check`.

## Documentation impact

Update architecture ownership, FastGate package map, glossary, and lesson with exact contracts.

## Out of scope

- Implementing the fake or a live adapter.
- Provider selection, aliases, rate limits, or telemetry.
