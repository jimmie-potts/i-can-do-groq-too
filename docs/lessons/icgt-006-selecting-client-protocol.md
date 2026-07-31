# ICGT-006 lesson: Selecting a client protocol before provider code

- **Unit:** ICGT-006
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; ADR 0003 remains proposed
- **Story:** [ICGT-006](../../user-stories/icgt-006-select-fastgate-api.md)
- **Review priority:** High
- **Visual companion:** Not required for this decision/spike unit unless the comparison benefits
- **Related architecture:** [Proposed ADR 0003](../adr/0003-fastgate-api-surface.md) and
  [Architecture](../architecture.md)

> This lesson presents decision criteria, not a selected or implemented API.

## Quick summary

This unit chooses the language clients use to ask FastGate for model work before provider-facing
types are frozen. It teaches northbound versus southbound contracts, compatibility claims, protocol
versioning, and semantic loss.

## Learning objectives

You should be able to compare Chat Completions, Responses, and a project-owned model-turn protocol;
identify information each preserves or loses; and explain why a separate harness adapter is required.

## Why this unit matters

An internal provider port encodes assumptions about input, output, failures, tools, and termination.
If designed first, it can make the external API an awkward mirror of one provider rather than a
reviewed client contract.

## Junior engineer foundation

**Northbound** is the API FastGate exposes to clients. **Southbound** is the port FastGate uses to
call providers. An adapter translates between them. Similar JSON fields do not guarantee the same
state machine.

A common misconception is that URL compatibility proves behavioral compatibility. A tested subset
must define required fields, ignored or rejected fields, event order, errors, cancellation, and
versioning.

## Key concepts

- **Compatibility claim:** exact tested surface, not a broad marketing label.
- **Semantic loss:** information that cannot survive a translation.
- **Unknown-field policy:** reject, preserve, or ignore; silent ignore is unsafe here.
- **Protocol version:** identifier and change rule for client-visible meaning.

## Architecture and invariants

```text
client contract -> FastGate transport/domain mapping -> provider port -> provider adapter
```

The client contract drives what the provider port must express. Provider-specific extensions remain
adapter-local or explicitly capability-gated.

## Practical walkthrough

Create one bounded example request and result for each option, then sketch one future text stream and
tool call. Map each to the harness's provider events. List every unsupported or lossy field. Select
the smallest protocol whose compatibility statement can be proved honestly.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [Proposed ADR 0003](../adr/0003-fastgate-api-surface.md) | Frames alternatives and evidence | Which option loses the least required meaning? |
| Planned client v1 contract | Drives provider-domain code | What happens to every unknown or unsupported field? |

## Implementation code samples

None. Examples created by this spike are contract examples, not endpoint implementation.

## Failure scenarios to study

- A client requests provider-managed state the selected upstream lacks.
- A Chat Completions translation cannot represent a required Responses terminal item.
- An unknown field is silently discarded before a paid request.
- A disconnect occurs after partial semantic output and a retry duplicates it.

## What changed during implementation

No spike evidence exists yet. Architecture review moved this decision before provider contracts so
the southbound port does not constrain the northbound API accidentally.

## Production expansion

Production gateways may expose several compatibility facades over one versioned internal protocol.
That helps ecosystem adoption but multiplies conformance tests and deprecation obligations. Add a
second facade only when a real client cannot use the first adapter economically.

## Practical exercises

- Map one tool request through all three options and mark semantic loss.
- Write the narrowest compatibility sentence you could defend with contract tests.
- Predict whether an unknown optional field should be rejected or capability-gated.

## Key takeaways

- Northbound client meaning should drive the southbound provider seam.
- Compatibility is a tested semantic contract, not a URL shape.
- A separate FastGate harness adapter preserves the direct OpenAI baseline.

## Glossary

- **Northbound:** interface exposed to clients.
- **Southbound:** interface used to call dependencies/providers.
- **Semantic loss:** meaning dropped or changed during translation.

## Teach-back questions

1. Why does the client protocol decision precede FastGate provider-domain types?
2. What must a compatibility claim say about unknown and unsupported fields?
3. When would multiple compatibility facades justify their added maintenance cost?

## Further reading

- [ICGT-006 delivery contract](../../user-stories/icgt-006-select-fastgate-api.md)
- [Proposed ADR 0003](../adr/0003-fastgate-api-surface.md)
