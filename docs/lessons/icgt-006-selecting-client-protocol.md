# ICGT-006 lesson: Defining FastGate model-turn v1 before provider code

- **Unit:** ICGT-006
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; ADR 0003 selects Option C, but no schema, fixtures, validator,
  endpoint, or runtime behavior exists
- **Story:** [ICGT-006](../../user-stories/icgt-006-select-fastgate-api.md)
- **Review priority:** High
- **Visual companion:** Planned after implementation because the offline validator makes this a
  code-bearing high-review-priority unit
- **Related architecture:** [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md) and
  [Architecture](../architecture.md)

> This lesson explains the selected protocol direction and the evidence still required before code
> may depend on it. It does not describe an implemented API.

## Quick summary

ADR 0003 chooses a small FastGate-owned model-turn protocol. This unit turns that direction into the
first versioned non-streaming client contract before provider-facing types are frozen. It teaches
northbound versus southbound contracts, compatibility claims, protocol versioning, and semantic
loss.

## Learning objectives

You should be able to explain why the project selected a FastGate-owned model-turn protocol; compare
it with Chat Completions and Responses; build a field-by-field compatibility matrix; identify
information each preserves or loses; and explain why FastGate owns wire conformance while Code
Assist Harness owns its adapter.

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
- **Conformance fixture:** language-neutral valid or invalid example pinned to one schema version.

## Architecture and invariants

```text
client contract -> FastGate transport/domain mapping -> provider port -> provider adapter
```

The client contract drives what the provider port must express. Provider-specific extensions remain
adapter-local or explicitly capability-gated. The accepted direction does not settle the exact v1
fields. ICGT-006 proves non-streaming representability and records future streaming/cancellation
gaps; it does not claim that those later behaviors work.

## Practical walkthrough

Start with the selected protocol rather than designing three candidate schemas. Keep the ADR's short
Chat Completions and Responses comparison as decision context; those rejected options are not
additional v1 endpoints or fixture corpora. Build a matrix against the current harness request,
operation, and event contract:
ordered conversation and repository instructions; text; provider-emitted tool events; optional
non-authoritative usage; normalized failure code, bounded control-safe message, and retryability;
exactly-one terminal behavior; cancellation; no-later-event behavior; local cleanup; and upstream
cleanup certainty. Mark every row exact, lossy, unsupported, or deferred. Required semantic loss
blocks ICGT-006 completion until the schema preserves it, a named versioned extension owns it, or a
later ADR changes the direction.

The following rows are illustrative, non-normative examples of the classification method. They are
not the future v1 schema or completed ICGT-006 evidence:

| Meaning under review | Example classification | Reason |
| --- | --- | --- |
| Ordered user text | Exact | A project-owned field can preserve order and text without a vendor type. |
| Raw provider error body | Lossy | The client receives a bounded normalized failure, not unsafe vendor content. |
| Provider-managed durable conversation state | Unsupported | FastGate does not own durable workflow history. |
| Stream cleanup certainty | Deferred | Later streaming and cleanup stories must define and test runtime evidence. |

The committed harness request cannot declare tools. Keep two later cases separate: a future
client-declared unsupported capability can fail before dispatch, while unsolicited tool output from
an upstream can only become a bounded failure after paid work may have begun.

The selected FastGate protocol then receives a canonical non-streaming schema plus valid and invalid
language-neutral fixtures. Streaming fixtures arrive with the stories that implement streaming;
ICGT-020 later turns observed behavior and an adapter-ready harness snapshot into the normative
cross-repository handoff. A committed offline contract test validates schema syntax, accepts every
valid fixture, rejects every invalid fixture, and runs from `./scripts/check`; this is contract
evidence, not endpoint behavior.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md) | Records Option C and rejected alternatives | Why does v1 use a FastGate-owned protocol, and what remains unproved? |
| Planned compatibility/gap matrix | Makes cross-repository meaning reviewable | Is every harness semantic exact, unsupported, lossy, or deferred? |
| Planned v1 schema and fixtures | Drives provider-domain code and later conformance | What happens to every unknown or unsupported field? |
| Planned offline fixture test | Proves the contract artifacts agree | Does each invalid fixture fail for its intended rule? |

## Implementation code samples

None yet. After the spike, show one focused fixture-validator excerpt. That validator is contract
tooling, not endpoint implementation or runtime conformance.

## Failure scenarios to study

- A client requests provider-managed state the selected upstream lacks.
- A proposed FastGate v1 field mapping cannot preserve a required harness terminal semantic.
- A future tool-capable request is dispatched even though FastGate cannot support its declared
  capability.
- An upstream emits an unsolicited tool event and the adapter falsely labels the post-dispatch
  failure as a preflight rejection.
- An invalid fixture is mislabeled or accidentally accepted by the offline validator.
- An unknown field is silently discarded before a paid request.
- A disconnect occurs after partial semantic output and a retry duplicates it.

## What changed during implementation

No spike evidence exists yet. Architecture review moved this decision before provider contracts so
the southbound port does not constrain the northbound API accidentally. On 2026-08-01, ADR 0003
selected the FastGate-owned protocol direction; the schema, fixtures, validator, and runtime remain
planned.

## Production expansion

Production gateways may expose several compatibility facades over one versioned internal protocol.
That helps ecosystem adoption but multiplies schema versions, fixture corpora, conformance tests,
and deprecation obligations. Add a second facade only when a real client cannot use the first
adapter economically.

## Practical exercises

- Map one provider-emitted tool request into the selected protocol and mark semantic loss.
- Explain why comparing the rejected formats remains useful after Option C is selected.
- Explain why the current harness request cannot request pre-dispatch tool capability admission.
- Classify local stream closure and confirmed versus unconfirmed upstream cleanup separately.
- Write the narrowest compatibility sentence you could defend with contract tests.
- Predict whether an unknown optional field should be rejected or capability-gated.

## Key takeaways

- Northbound client meaning should drive the southbound provider seam.
- FastGate v1 owns its protocol instead of claiming Chat Completions or Responses compatibility.
- Compatibility is a tested semantic contract, not a URL shape.
- FastGate owns its versioned wire schema and fixtures; Code Assist Harness owns the separate client
  adapter that consumes them.

## Glossary

- **Northbound:** interface exposed to clients.
- **Southbound:** interface used to call dependencies/providers.
- **Conformance fixture:** version-pinned example with an expected valid or invalid outcome.
- **Semantic loss:** meaning dropped or changed during translation.

## Teach-back questions

1. Why does the client protocol decision precede FastGate provider-domain types?
2. Why does accepting Option C not make ICGT-006 or the FastGate endpoint complete?
3. Which evidence belongs in ICGT-006, ICGT-020, and a later harness-owned adapter story?

## Further reading

- [ICGT-006 delivery contract](../../user-stories/icgt-006-select-fastgate-api.md)
- [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md)
