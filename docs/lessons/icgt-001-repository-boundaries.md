# ICGT-001 lesson: Repository and architecture boundaries

- **Unit:** ICGT-001
- **Milestone:** M0 - Repository and learning foundation
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; boundaries, component briefs, and ADRs are present and validated
- **Story:** [ICGT-001](../../user-stories/icgt-001-record-boundaries.md)
- **Review priority:** High
- **Visual companion:** Not required for this documentation-only foundation unit
- **Related architecture:** [Architecture](../architecture.md),
  [ADR 0001](../adr/0001-separate-learning-monorepo.md),
  [ADR 0002](../adr/0002-fake-first-openai-first-live.md), and
  [proposed ADR 0003](../adr/0003-fastgate-api-surface.md)

> This lesson describes the implemented documentation boundary, not implemented services.

## Quick summary

ICGT-001 separates a coding-agent application from the inference platform it may later call. The
central idea is ownership: each decision should have one authoritative layer, even when several
layers observe or enforce related information.

## Learning objectives

After this unit, you should be able to:

- distinguish agent workflow, inference data plane, and platform control plane;
- explain why FastGate is a provider adapter target rather than part of the harness loop;
- identify the Responses-versus-Chat-Completions integration tension; and
- explain why local limits, remote quota, workflow transcripts, and billing ledgers remain separate.

## Why this unit matters

Distributed systems often fail architecturally before they fail technically: two components both
believe they own retries, routing, state, or billing. Those overlaps cause duplicate cost, ambiguous
cancellation, inconsistent audits, and changes that require coordinated releases everywhere.

The harness already owns an explicit asynchronous provider seam. Preserving it lets FastGate evolve
independently while the harness remains safe and understandable.

## Junior engineer foundation

### A boundary is a decision about authority

A boundary is more than a folder or HTTP call. It answers, “Who makes the final decision?”

For example, both the harness and FastGate have limits:

- the harness stops a coding task after a local deadline or output budget;
- FastGate protects tenant and provider capacity.

These do not conflict when local limits can only make a request stricter. They conflict if FastGate
quota can silently authorize another harness turn.

### Data plane versus control plane

The data plane handles the hot request/response path. The control plane manages the policy and
desired state used by that path. FastGate is a data plane; TenantPlane is a control plane.

A common misconception is that the control plane must be called synchronously for every request.
That makes its outage a data-plane outage. A production design usually distributes a versioned,
expiring policy projection, with explicit behavior when it becomes stale.

### Similar APIs are not the same contract

OpenAI Responses and Chat Completions both carry model input and streamed output, but their items,
tool semantics, state, and terminal events differ. “Compatible” should mean a tested subset, not a
brand-shaped URL.

## Key concepts

- **Authority:** the component whose decision is final for a concern.
- **Adapter:** a translator that protects domain code from another system's shape.
- **Baseline:** direct OpenAI infrastructure behavior used to measure gateway semantics and overhead;
  coding-task parity waits for a harness-owned adapter.
- **Capability:** behavior a provider has proved it can support.
- **Handoff contract:** a versioned cross-repository mapping for requests, streams, errors, and
  cancellation.

## Architecture and invariants

```text
Harness workflow -> Harness Provider port -> CAH-owned FastGate adapter -> FastGate -> provider adapter
      owns task          owns turn meaning             translates              routes       owns vendor wire
```

Important invariants:

- FastGate never owns the coding-agent loop or tool approval.
- The harness never relies on provider-managed state as durable task history.
- FastGate does not retry after the first semantic output is committed.
- Local harness budgets remain active behind the gateway.
- Harness transcripts are not usage-ledger records.
- The direct OpenAI adapter is not repointed to FastGate.

## Practical walkthrough

1. Read the ownership matrix in [Architecture](../architecture.md).
2. Follow the harness integration section from direct OpenAI to a future CAH-owned FastGate adapter
   consuming FastGate-owned schema and fixtures.
3. Review ADR 0003 and list information that would be lost by pretending Responses and Chat
   Completions were identical.
4. Review each component README and find its explicit non-responsibilities.
5. Confirm current status says planned rather than inferring implementation from diagrams.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [Architecture](../architecture.md) | Establishes ownership across repositories and layers | Is any concern authoritative in two places? |
| [Proposed ADR 0003](../adr/0003-fastgate-api-surface.md) | Prevents an accidental incompatible endpoint | What evidence is needed before selecting the protocol? |
| [FastGate README](../../gateway/README.md) | Defines data-plane scope and harness handoff | What must FastGate never own? |

## Implementation code samples

No application code exists in this unit. The exact reviewed artifacts are the documents above. The
lesson must not fabricate package or endpoint examples as shipped code.

## Failure scenarios to study

### Reuse the direct OpenAI adapter for FastGate

The direct adapter's official endpoint, exact event automaton, model allowlist, and disabled retries
would be weakened or bypassed. The safe outcome is a separate adapter after FastGate's protocol is
versioned.

### Retry in both harness and gateway

A transient failure could trigger multiple paid operations, and partial output could be duplicated.
The safe ownership rule is gateway-only provider/network retry before output commitment, with
workflow retry remaining an explicit harness decision.

### Deploy the harness as a Kubernetes worker

The local workspace, keyboard approval, process boundary, and secret model would disappear without a
replacement. The operator MVP explicitly excludes this until a separate harness ADR exists.

### Overclaim Git/worktree behavior

Repository contribution guidance is not product functionality. The actual harness MVP excludes Git
state mutation, so this repository records future Git workflow as a possible harness concern, not
implemented behavior.

## What changed during implementation

The first draft inherited the conversation's aspirational worktree flow as if it were current
harness behavior. Reconciliation against the actual harness roadmap corrected that overclaim. The
review also identified the API mismatch as the only concrete integration conflict requiring an
upfront FastGate ADR.

## Production expansion

At production scale, cross-repository contracts would include compatibility tests, schema versions,
trace propagation, deprecation policy, and separately owned SLOs. That machinery costs release
coordination and operational ownership, so it should follow a working FastGate v1 rather than lead
the learning project.

## Practical exercises

- For a client disconnect, write which layer observes it, which layer decides task cancellation, and
  which layer closes the provider stream.
- Classify five example limits as local safety budgets or platform quotas.
- Compare one Responses tool-call sequence with one Chat Completions tool-call sequence before
  recommending ADR 0003.

## Key takeaways

- The harness owns agent behavior; this repository owns inference platform behavior.
- Similar provider APIs require explicit translation and capability evidence.
- Extra layers are justified only when ownership remains singular and measurable.

## Glossary

See [the project glossary](../glossary.md) for adapter, capability registry, control plane, data
plane, and provider adapter.

## Teach-back questions

1. Why should FastGate be a separate harness provider adapter instead of a new OpenAI base URL?
2. How can harness limits and TenantPlane quotas both apply without creating two authorities?
3. What evidence would justify turning FleetSim output into live routing policy?

## Further reading

- [ICGT-001 delivery contract](../../user-stories/icgt-001-record-boundaries.md)
- [Code Assist Harness relationship](../../README.md#relationship-to-code-assist-harness)
- [FastGate API decision](../adr/0003-fastgate-api-surface.md)
