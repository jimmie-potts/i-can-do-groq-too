# ICGT-007 lesson: Provider contracts as a stable seam

- **Unit:** ICGT-007
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; no FastGate provider types exist
- **Story:** [ICGT-007](../../user-stories/icgt-007-define-provider-contracts.md)
- **Review priority:** High
- **Visual companion:** Planned after implementation
- **Related architecture:** [Architecture](../architecture.md) and
  [ADR 0002](../adr/0002-fake-first-openai-first-live.md)

> This lesson describes intended concepts only. It contains no shipped Go contract.

## Quick summary

This unit will define the smallest non-streaming vocabulary FastGate needs to invoke an upstream
without importing OpenAI or Groq types into gateway policy. It teaches ports, adapters, safe
normalization, and a bounded context-aware call.

## Learning objectives

You should be able to distinguish a port from an adapter, trace one context-aware non-streaming
invocation, explain why failure normalization is lossy by design, and identify provider data that
must not cross the boundary.

## Why this unit matters

The first live SDK can easily become the accidental domain model. A small FastGate-owned contract
allows the fake and live providers to share tested meaning while keeping provider differences
visible.

## Junior engineer foundation

A port is an interface the application owns. An adapter implements that interface by translating an
external system. The port should describe what FastGate needs, not everything any vendor can do.

A common misconception is that a universal interface must expose the union of every provider
feature. That produces an unstable mega-interface. If the selected client schema carries a
capability requirement, this unit treats it only as bounded request data. A later admission story
compares requirements with tested provider behavior before paid work.

## Key concepts

- **Port:** project-owned behavioral promise.
- **Adapter:** provider-specific translator and resource owner.
- **Normalization:** converting unsafe, unstable external values into bounded domain values.
- **Bounded invocation:** one context-aware call that returns one reviewed result or normalized
  failure within the limits owned by its caller.

## Architecture and invariants

```text
FastGate transport/policy -> FastGate provider port <- fake/OpenAI/Groq adapters
```

Vendor SDK objects, endpoints, credentials, raw exceptions, headers, and response bodies remain
outside the port. Provider capability discovery and admission remain a separate later behavior.

## Practical walkthrough

Start from accepted ADR 0003, the ICGT-006 schema, and the next fake story's exact needs. Define only
values necessary to validate one non-streaming request and return one result or enumerated failure.
Pass the caller context through the port, but defer the asynchronous operation, explicit
cancellation, cleanup confirmation, routing, streaming events, retries, and telemetry.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Planned request/result values | Define stable meaning and bounds | Does any field belong to a vendor or later policy layer? |
| Planned non-streaming invocation | Defines one small provider behavior | Can it produce anything other than one bounded result or normalized failure? |
| Planned validation/failure tests | Prove safe early rejection | Can malformed or unbounded work reach the adapter? |

## Implementation code samples

None. Add exact Go types and validation tests after implementation.

## Failure scenarios to study

- A capability-requirement field selected by ICGT-006 is unbounded or vendor-specific.
- A raw upstream exception contains an authorization header.
- A canceled caller context is discarded before the bounded invocation.
- A new provider requires a meaningful semantic difference the current port cannot express.

The safe response to the last case is a reviewed contract change, not silent flattening.

## What changed during implementation

No implementation evidence exists yet.

## Production expansion

Production gateways may add an asynchronous stream-operation port, explicit cleanup barriers,
versioned capability discovery/admission, generated client contracts, and provider conformance
suites. Add those mechanisms in the stories that can test their actual lifecycle or dependency
boundary.

## Practical exercises

- Classify ten proposed fields as domain, adapter, transport, or TenantPlane policy.
- Design a failure code plus locally-authored presentation text without carrying raw exception text.
- Explain why `retryable` is an observation rather than permission to retry.

## Key takeaways

- FastGate owns the provider vocabulary; vendors own translation details.
- Capability requirement data does not force capability admission into the same story.
- A small non-streaming port should not predesign the later streaming lifecycle.
- Normalized failures trade detail for stability, privacy, and safe cross-layer handling.

## Glossary

- **Bounded invocation:** one context-aware call with one result or normalized failure.
- **Provider port:** a project-owned interface implemented by fake and live adapters.
- **Preflight admission:** deferred comparison of request requirements with tested provider
  capabilities before paid work.

## Teach-back questions

1. Why should a FastGate provider port not reuse OpenAI SDK response classes?
2. Why does ICGT-007 pass a context without defining an asynchronous cancellation/cleanup API?
3. Why should provider capability admission be separate from defining request data?

## Further reading

- [ICGT-007 delivery contract](../../user-stories/icgt-007-define-provider-contracts.md)
- [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md)
- [Code Assist Harness boundary lesson](icgt-001-repository-boundaries.md)
