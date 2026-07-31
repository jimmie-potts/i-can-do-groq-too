# Unit lessons

Every implementation-ready story has one written learning companion. A story defines what must be
delivered; its lesson explains the concepts, important code, failure paths, trade-offs, and personal
review questions.

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
| ICGT-004 | [Go toolchain and modules](icgt-004-go-toolchain-and-modules.md) | Planned | Not required - decision unit |
| ICGT-005 | [Go service lifecycle](icgt-005-go-service-lifecycle.md) | Planned | Required when verified |
| ICGT-006 | [Selecting a client protocol](icgt-006-selecting-client-protocol.md) | Planned | Required when verified - contract tooling |
| ICGT-007 | [Provider contracts](icgt-007-provider-contracts.md) | Planned | Required when verified |
| ICGT-008 | [Basic deterministic fake upstream](icgt-008-basic-deterministic-fake.md) | Planned | Required when verified |

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

High-review-priority code-bearing units add a PPTX under `assets/` after written/code evidence is
stable. Documentation-only foundation units do not require one unless a visual materially improves
the decision review. Render every slide, inspect each image, and run the available overflow test
before listing a deck as verified evidence. Visuals should make ownership, sequence, state, or
failure behavior easier to remember; decoration is not evidence.
