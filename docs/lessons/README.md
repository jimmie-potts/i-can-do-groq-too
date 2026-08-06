# Unit lessons

Every implementation-ready story has exactly one source-backed Markdown learning companion. A story
defines what must be delivered; its lesson explains the concepts, important code, failure paths,
trade-offs, and personal review questions. Visual companions are optional and never determine
whether a story or lesson is complete.

## Evidence states

| Lesson status | Meaning |
| --- | --- |
| Planned | Concepts and intended review path only; no code is claimed. |
| Implementation companion | Work exists, but acceptance, validation, or review is incomplete. |
| Implementation companion - blocked | Work is partially evidenced and the named blocker prevents completion. |
| Verified against implementation | Exact source/test paths, observed behavior, validation, and review are complete. |

The linked story status and lesson status must agree. A future diagram or pseudocode is not shipped
evidence.

## Lesson sequence

| Unit | Lesson | Status | Visual companion |
| --- | --- | --- | --- |
| ICGT-001 | [Repository boundaries](icgt-001-repository-boundaries.md) | Verified against implementation | Not required - documentation unit |
| ICGT-002 | [Learning workflow](icgt-002-learning-workflow.md) | Verified against implementation | Not required - documentation unit |
| ICGT-003 | [Repository checks](icgt-003-repository-checks.md) | Verified against implementation | Not required - small tooling unit |
| ICGT-004 | [Go toolchain and modules](icgt-004-go-toolchain-and-modules.md) | Verified against implementation | Not required - decision unit |
| ICGT-005 | [Go service lifecycle](icgt-005-go-service-lifecycle.md) | Verified against implementation | [FastGate service lifecycle](assets/icgt-005-go-service-lifecycle.pptx) |
| ICGT-006 | [Defining FastGate model-turn v1](icgt-006-selecting-client-protocol.md) | Verified against implementation | Not required - Markdown contract lesson |
| ICGT-007 | [Provider contracts](icgt-007-provider-contracts.md) | Verified against implementation | Not required - Markdown contract lesson |
| ICGT-008 | [Basic deterministic fake upstream](icgt-008-basic-deterministic-fake.md) | Verified against implementation | Not required - Markdown implementation lesson |
| ICGT-009 | [Bounded model-turn admission](icgt-009-bounded-model-turn-admission.md) | Verified against implementation | Not required - Markdown implementation lesson |

Use [the lesson template](lesson-template.md) for new implementation-ready units and read
[the scoped guidance](AGENTS.md) before editing lesson evidence.

## Personal review standard

A verified code lesson identifies a small review map:

1. the entry point or primary production path;
2. the state or contract that carries the core invariant;
3. one failure, cancellation, or cleanup path;
4. the tests that prove those behaviors; and
5. code deliberately deferred to later units.

Exact excerpts are explained line-by-line or in small logical chunks. A summary alone is not a
substitute for reading the important source.

## Visual companions

Visual companions are created only when the user explicitly requests one or a separately reviewed
justification explains how it materially improves understanding. They never replace the required
Markdown lesson and are not completion evidence for the story itself. When selected, add the visual
under `assets/` only after written and code evidence is stable, render and inspect every slide, and
run the available overflow test before listing it as verified. The existing ICGT-005 deck remains
verified historical evidence for that unit.
