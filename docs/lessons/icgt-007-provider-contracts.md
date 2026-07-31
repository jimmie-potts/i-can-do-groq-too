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

This unit will define the smallest vocabulary FastGate needs to invoke an upstream without importing
OpenAI or Groq types into gateway policy. It teaches ports, adapters, capabilities, normalization,
and lazy operation ownership.

## Learning objectives

You should be able to distinguish a port from an adapter, define capability requirements, explain
why failure normalization is lossy by design, and identify provider data that must not cross the
boundary.

## Why this unit matters

The first live SDK can easily become the accidental domain model. A small FastGate-owned contract
allows the fake and live providers to share tested meaning while keeping provider differences
visible.

## Junior engineer foundation

A port is an interface the application owns. An adapter implements that interface by translating an
external system. The port should describe what FastGate needs, not everything any vendor can do.

A common misconception is that a universal interface must expose the union of every provider
feature. That produces an unstable mega-interface. Capability requirements let a small request say
what behavior it needs and fail early when an adapter cannot provide it.

## Key concepts

- **Port:** project-owned behavioral promise.
- **Adapter:** provider-specific translator and resource owner.
- **Capability:** explicit proven support for requested behavior.
- **Normalization:** converting unsafe, unstable external values into bounded domain values.
- **Lazy operation:** construction that performs no network work until consumed.

## Architecture and invariants

```text
FastGate transport/policy -> FastGate provider port <- fake/OpenAI/Groq adapters
```

Vendor SDK objects, endpoints, credentials, raw exceptions, headers, and response bodies remain
outside the port. Capability absence is decided before paid work begins.

## Practical walkthrough

Start from accepted ADR 0003 and the next fake story's exact needs. Define only values necessary to
match one non-streaming request, return one result or enumerated failure, and own cleanup. Defer
routing, streaming events, retries, and telemetry.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Planned request/result values | Define stable meaning and bounds | Does any field belong to a vendor or later policy layer? |
| Planned provider operation interface | Defines laziness and cleanup | What can happen before consumption? |
| Planned negative boundary tests | Prevent dependency leakage | How does CI detect an SDK import in domain code? |

## Implementation code samples

None. Add exact Go types and validation tests after implementation.

## Failure scenarios to study

- A capability required by the accepted non-streaming request contract is absent.
- A raw upstream exception contains an authorization header.
- The adapter performs network I/O during construction.
- A new provider requires a meaningful semantic difference the current port cannot express.

The safe response to the last case is a reviewed contract change, not silent flattening.

## What changed during implementation

No implementation evidence exists yet.

## Production expansion

Production gateways may version capability schemas, advertise them through discovery, generate
client contracts, and run provider conformance suites. Adopt that machinery when more than one
client/provider creates demonstrated drift.

## Practical exercises

- Classify ten proposed fields as domain, adapter, transport, or TenantPlane policy.
- Design a failure code plus locally-authored presentation text without carrying raw exception text.
- Explain why `retryable` is an observation rather than permission to retry.

## Key takeaways

- FastGate owns the provider vocabulary; vendors own translation details.
- Capability absence fails explicitly before paid work.
- Normalized failures trade detail for stability, privacy, and safe cross-layer handling.

## Glossary

See [the project glossary](../glossary.md) for capability registry, deterministic fake, and provider
adapter.

## Teach-back questions

1. Why should a FastGate provider port not reuse OpenAI SDK response classes?
2. What invariant does lazy operation construction protect?
3. When is adding a capability flag better than widening every request with an optional field?

## Further reading

- [ICGT-007 delivery contract](../../user-stories/icgt-007-define-provider-contracts.md)
- [Code Assist Harness boundary lesson](icgt-001-repository-boundaries.md)
