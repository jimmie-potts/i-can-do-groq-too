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
- If the accepted ICGT-006 schema includes capability-requirement fields, represent them only as
  bounded provider-neutral request data; defer capability discovery and admission behavior.
- Define one context-aware, bounded, non-streaming provider invocation.
- Add compile-time/interface and validation tests with no SDK or network dependency.

## Locked behavior

- Domain contracts contain no OpenAI, Groq, HTTP, SDK, credential, endpoint, tenant-policy, or raw
  error type.
- This unit performs no provider capability discovery, admission, emulation, or routing.
- Failure values contain project-owned enumerated codes and retryability only. A separate local
  presentation mapping owns fixed safe messages.
- The invocation receives the caller's context, but this unit does not invent an asynchronous
  operation, stream grammar, explicit cancel method, or cleanup-confirmation state.

## Acceptance criteria

1. One small interface captures the behavior the next fake story must implement.
2. Requests and results validate bounds and immutable ownership expectations.
3. Any capability-requirement field selected by ICGT-006 is bounded and vendor-neutral; no
   capability-admission policy or service is introduced.
4. Normalized failure values have no arbitrary string field that could carry raw provider
   exceptions, response bodies, headers, or credentials.
5. One context-aware call returns exactly one bounded result or normalized failure; it defines no
   streaming or asynchronous operation lifecycle.
6. Contract, validation, and compile-time tests run without an SDK, credentials, or network.
7. The lesson compares this non-streaming port with the harness's asynchronous port without copying
   its Python types or lifecycle prematurely.

## Human review checkpoint

- **Production path:** Request validation through one bounded non-streaming invocation and result.
- **Failure/test path:** Malformed/unbounded request and unsafe/unbounded failure rejection.
- **Invariant:** FastGate domain code depends on its own vocabulary, never a vendor vocabulary.
- **Deferred:** Streaming event grammar; asynchronous operation, cancellation, and cleanup
  lifecycle; capability discovery and admission; concrete-adapter SDK dependency enforcement;
  retries; routing; and live providers.

## Validation

- Focused Go domain/interface tests.
- `./scripts/check`.

## Documentation impact

Update architecture ownership, FastGate package map, glossary, and lesson with exact contracts.

## Out of scope

- Implementing the fake or a live adapter.
- Defining streaming, explicit cancellation, or cleanup-confirmation behavior.
- Implementing provider capability discovery, admission, emulation, or routing.
- Enforcing concrete provider SDK dependency policy before a concrete adapter exists.
- Provider selection, aliases, rate limits, or telemetry.
