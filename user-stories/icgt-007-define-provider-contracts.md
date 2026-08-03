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
- Define enumerated normalized failure categories, retryability observations, and optional bounded
  usage observed before failure, with no arbitrary upstream-authored message field.
- Represent ICGT-006's selected capability requirement only as bounded provider-neutral request
  data; ICGT-009 owns mandatory static rejection, while later stories own discovery and support.
- Define one context-aware, bounded, non-streaming provider invocation.
- Add compile-time/interface and validation tests with no SDK or network dependency.

## Locked behavior

- Domain contracts contain no OpenAI, Groq, HTTP, SDK, credential, endpoint, tenant-policy, or raw
  error type.
- This unit performs no provider capability discovery, admission, emulation, or routing.
- A failed outcome contains a project-owned enumerated code, retryability, and optional bounded
  usage. Usage remains separate from raw error details; a separate local presentation mapping owns
  fixed safe messages.
- Failure-side usage uses the v1 non-negative JavaScript-safe integer bound. Absence remains distinct
  from zero, and the observation is neither billing proof nor retry permission.
- `invalid_request` and `unsupported_capability` are admission outcomes created above the provider
  port. An adapter cannot return either category for work that was already invoked.
- The invocation receives the caller's context, but this unit does not invent an asynchronous
  operation, stream grammar, explicit cancel method, or cleanup-confirmation state.

## Acceptance criteria

1. One small interface captures the behavior the next fake story must implement.
2. Requests and results validate bounds and immutable ownership expectations.
3. The selected `tool_calls` requirement is bounded and vendor-neutral; no capability-admission
   policy or service is introduced in this unit.
4. Normalized failed outcomes have no arbitrary string field that could carry raw provider
   exceptions, response bodies, headers, or credentials. They preserve optional usage unchanged and
   test absent, zero, maximum, and above-maximum cases.
5. One context-aware call returns exactly one bounded result or normalized failure; it defines no
   streaming or asynchronous operation lifecycle.
6. Contract, validation, and compile-time tests run without an SDK, credentials, or network.
7. The lesson compares this non-streaming port with the harness's asynchronous port without copying
   its Python types or lifecycle prematurely.

## Human review checkpoint

- **Production path:** Request validation through one bounded non-streaming invocation and result.
- **Failure/test path:** Malformed/unbounded request, unsafe failure rejection, and failure-side usage
  overflow without losing a valid observation.
- **Invariant:** FastGate domain code depends on its own vocabulary and never discards an observation
  the northbound v1 contract promises to preserve.
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
