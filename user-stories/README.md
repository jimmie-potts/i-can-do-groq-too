# User stories

This directory is the dependency-ordered delivery source of truth. The roadmap describes outcomes;
an individual story is the reviewed contract for one small implementation unit.

## How to use this backlog

- Select only a dependency-ready story.
- Read its lesson and relevant ADRs before planning code.
- Review a code-free plan before implementation.
- Keep the change within the story's scope and exclusions.
- Personally review the named production path, failure/test path, and invariant.
- Update the lesson with exact implementation evidence before marking the story Done.
- Run focused validation and `./scripts/check`.
- Record durable surprises in `notes/`; do not turn notes into a second backlog.

## Status vocabulary

| Status | Meaning |
| --- | --- |
| Planned | Scoped but not started. |
| In progress | Work has begun, but evidence or review is incomplete. |
| Blocked | A named dependency or external decision prevents progress. |
| Done | Acceptance criteria, review checkpoint, validation, and lesson evidence are complete. |

## Reviewed planned sequence

| Order | Story | Lesson | Milestone | Status | Depends on | Review priority |
| ---: | --- | --- | --- | --- | --- | --- |
| 1 | [ICGT-001: Record boundaries and decisions](icgt-001-record-boundaries.md) | [Repository boundaries](../docs/lessons/icgt-001-repository-boundaries.md) | M0 | Done | None | High |
| 2 | [ICGT-002: Establish the learning standard](icgt-002-establish-learning-standard.md) | [Learning workflow](../docs/lessons/icgt-002-learning-workflow.md) | M0 | Done | ICGT-001 | High |
| 3 | [ICGT-003: Establish repository checks](icgt-003-establish-repository-checks.md) | [Repository checks](../docs/lessons/icgt-003-repository-checks.md) | M0 | Done | ICGT-002 | Normal |
| 4 | [ICGT-004: Select the Go toolchain and module strategy](icgt-004-select-go-toolchain.md) | [Go toolchain and modules](../docs/lessons/icgt-004-go-toolchain-and-modules.md) | M1 | Planned | ICGT-003 | High |
| 5 | [ICGT-005: Bootstrap the FastGate service lifecycle](icgt-005-bootstrap-fastgate-service.md) | [Go service lifecycle](../docs/lessons/icgt-005-go-service-lifecycle.md) | M1 | Planned | ICGT-004 | High |
| 6 | [ICGT-006: Select FastGate's first client protocol](icgt-006-select-fastgate-api.md) | [Selecting a client protocol](../docs/lessons/icgt-006-selecting-client-protocol.md) | M1 | Planned | ICGT-005 | High |
| 7 | [ICGT-007: Define provider contracts](icgt-007-define-provider-contracts.md) | [Provider contracts](../docs/lessons/icgt-007-provider-contracts.md) | M1 | Planned | ICGT-006 | High |
| 8 | [ICGT-008: Build the basic deterministic fake upstream](icgt-008-build-basic-fake-upstream.md) | [Basic deterministic fake](../docs/lessons/icgt-008-basic-deterministic-fake.md) | M1 | Planned | ICGT-007 | High |

ICGT-001 through ICGT-003 were delivered and validated by the repository bootstrap. Of the planned
rows, only ICGT-004 is currently dependency-ready. ICGT-005 is the first code-bearing unit and also
requires a reviewed, locally available Go toolchain. Promote each later row only after its named
dependency is accepted; no live provider or public inference endpoint is implementation-ready.

## Later outcome slices

These identifiers preserve small sequencing but are not implementation-ready contracts. Promote
one only by adding a reviewed story and lesson after its dependency evidence exists.

| ID | Outcome slice | Depends on |
| --- | --- | --- |
| ICGT-009 | Expose one non-streaming fake-backed inference request | ICGT-008 |
| ICGT-010 | Normalize request and upstream failures at the transport boundary | ICGT-009 |
| ICGT-011 | Define and test the FastGate SSE grammar | ICGT-010 |
| ICGT-012 | Extend the fake with deterministic stream gates and cancellation | ICGT-011 |
| ICGT-013 | Stream deterministic fake output through the endpoint | ICGT-012 |
| ICGT-014 | Propagate client cancellation | ICGT-013 |
| ICGT-015 | Enforce an upstream deadline and cleanup grace | ICGT-014 |
| ICGT-016 | Bound slow-client backpressure | ICGT-015 |
| ICGT-017 | Record low-cardinality latency and usage metrics | ICGT-016 |
| ICGT-018 | Add the opt-in OpenAI upstream adapter | ICGT-017 |
| ICGT-019 | Compare direct OpenAI with FastGate-to-OpenAI | ICGT-018 |
| ICGT-020 | Define the harness-to-FastGate v1 handoff | ICGT-019 |
| ICGT-021 | Add a separate FastGate adapter to the harness | ICGT-020 plus harness readiness |
| ICGT-022 | Add the first limited Groq upstream path | ICGT-019 |

See [the outcome backlog](backlog.md) for LatencyLab, operator, TenantPlane, and FleetSim milestones.

## Review priorities

**High** means the user should personally trace the important production and failure paths. A high
code-bearing unit requires a visual companion after implementation; a documentation-only foundation
unit does not. **Normal** still requires the written lesson and review checkpoint but may remain
text-only unless a visual would materially improve understanding.

## Notes

- [Bootstrap decisions](notes/2026-07-31-bootstrap-decisions.md) records the recovered conversation,
  harness reconciliation, local tool availability, and intentionally open decisions.
