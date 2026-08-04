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
| 4 | [ICGT-004: Select the Go toolchain and module strategy](icgt-004-select-go-toolchain.md) | [Go toolchain and modules](../docs/lessons/icgt-004-go-toolchain-and-modules.md) | M1 | Done | ICGT-003 | High |
| 5 | [ICGT-005: Bootstrap the FastGate service lifecycle](icgt-005-bootstrap-fastgate-service.md) | [Go service lifecycle](../docs/lessons/icgt-005-go-service-lifecycle.md) | M1 | Done | ICGT-004 | High |
| 6 | [ICGT-006: Specify and version FastGate's model-turn v1 contract](icgt-006-select-fastgate-api.md) | [Defining FastGate model-turn v1](../docs/lessons/icgt-006-selecting-client-protocol.md) | M1 | Done | ICGT-005 | High |
| 7 | [ICGT-007: Define provider contracts](icgt-007-define-provider-contracts.md) | [Provider contracts](../docs/lessons/icgt-007-provider-contracts.md) | M1 | Done | ICGT-006 | High |
| 8 | [ICGT-008: Build the basic deterministic fake upstream](icgt-008-build-basic-fake-upstream.md) | [Basic deterministic fake](../docs/lessons/icgt-008-basic-deterministic-fake.md) | M1 | Done | ICGT-007 | High |
| 9 | [ICGT-009: Admit and execute one fake-backed model turn](icgt-009-admit-and-execute-model-turn.md) | [Bounded model-turn admission](../docs/lessons/icgt-009-bounded-model-turn-admission.md) | M1 | Done | ICGT-008 | High |

ICGT-001 through ICGT-009 are delivered and validated. Promote each later row only after its named
dependency and start conditions are satisfied; no live provider or public inference endpoint is
implementation-ready.

## Later outcome slices

These identifiers preserve small sequencing but are not implementation-ready contracts. Promote
one only by adding a reviewed story and lesson after its dependency evidence exists.

| ID | Outcome slice | Depends on |
| --- | --- | --- |
| ICGT-010 | Expose one loopback-only fake-backed model turn and map every admitted provider outcome | ICGT-009 |
| ICGT-011 | Define and test the FastGate-owned model-turn SSE grammar | ICGT-010 |
| ICGT-012 | Extend the fake with deterministic stream gates and cancellation | ICGT-011 |
| ICGT-013 | Stream deterministic fake output through the endpoint | ICGT-012 |
| ICGT-014 | Propagate client cancellation | ICGT-013 |
| ICGT-015 | Enforce an upstream deadline, cleanup grace, and bounded FastGate-local reaping | ICGT-014 |
| ICGT-016 | Bound slow-client backpressure | ICGT-015 |
| ICGT-017 | Record low-cardinality latency and usage metrics | ICGT-016 |
| ICGT-018 | Add the opt-in OpenAI upstream adapter | ICGT-017 |
| ICGT-019 | Compare direct OpenAI and FastGate at the infrastructure boundary | ICGT-018 |
| ICGT-020 | Define the versioned harness-to-FastGate v1 handoff | ICGT-019 plus an adapter-ready harness contract |
| ICGT-021 | Publish FastGate v1 conformance artifacts and client integration guidance | ICGT-020 |
| ICGT-022 | Add the first limited Groq upstream path | ICGT-019 |

The reviewed [ICGT-009 contract](icgt-009-admit-and-execute-model-turn.md) owns the complete body and
semantic admission boundary: its exact raw cap, strict JSON/schema profile, safe-correlation rule,
fixed `learning-text` alias for the injected provider port, capability-first rejection, zero-dispatch
failures, exactly one admitted invocation, and validation of that provider return. The deterministic
fake supplies the primary zero/one-call evidence. ICGT-009 does not bind the public inference route or
map a valid provider outcome to wire transport.

ICGT-010 is the next outcome slice. Once promoted and approved, it binds the first loopback-only,
fake-backed route, defines method/media-type behavior, and
exhaustively maps both completed and normalized failed provider outcomes, including optional failure
usage. It owns HTTP status mapping for admission and provider outcomes, repeats the `tool_calls`
ordering test against the fake, and must observe zero calls. It may
not ignore a valid provider failure. It must also reject wrong method/media type with zero fake calls
and prevent the inference route from starting on a non-loopback listener. Caller authentication
remains unimplemented, so the route is not an externally deployable client surface. The first
ICGT-020/021 handoff stays loopback-only and unauthenticated; it does not advertise an unimplemented
authentication scheme. A later reviewed FastGate auth implementation and profile must precede
non-loopback use. Later capability work owns discovery, support, emulation, and routing—not
ICGT-009's mandatory v1 rejection.

ICGT-015 owns only FastGate-local upstream goroutine/stream reaping after its bounded cleanup grace.
Completion is local cleanup evidence, never proof that a remote provider stopped work or billing.

ICGT-019 uses a repository-owned measurement client to compare the same bounded workload directly
and through FastGate; it does not depend on a harness adapter or claim coding-task parity. ICGT-020
pins the then-current immutable harness provider contract and exact FastGate schema/fixture source
artifacts, then defines a candidate joint handoff with server and client ownership separated.
ICGT-021 remains
FastGate-owned: it packages and validates those exact artifacts, records their manifest/digest, and
cannot alter semantics without a new version. A later Code Assist Harness story implements
`FastGateProvider`, accepts the client side of the profile, and precedes any coding-task parity claim.
ICGT-020 must re-audit the then-current harness provider contract rather than assume the ICGT-006
snapshot is adapter-ready. The first profile is no-tool only; full coding-agent parity requires a
separately reviewed, versioned FastGate tool extension and matching CAH contract.

See [the outcome backlog](backlog.md) for LatencyLab, operator, TenantPlane, and FleetSim milestones.

## Review priorities

**High** means the user should personally trace the important production and failure paths.
**Normal** still requires the written lesson and review checkpoint but has a smaller personal-review
surface. Every implementation-ready story requires exactly one source-backed Markdown lesson.
Visual companions are optional for both priorities and are created only when explicitly requested or
separately justified; they are never a requirement for Done.

## Notes

- [Bootstrap decisions](notes/2026-07-31-bootstrap-decisions.md) records the recovered conversation,
  harness reconciliation, local tool availability, and intentionally open decisions.
- [ICGT-004 toolchain and module decision](notes/2026-08-01-icgt-004-toolchain-module-decision.md)
  records the official-source checks, local preflight, accepted design, and ICGT-005 handoff.
- [ICGT-005 FastGate lifecycle](notes/2026-08-02-icgt-005-fastgate-lifecycle.md) records the
  implementation choices, lifecycle and gate evidence, environment constraints, and ICGT-006
  handoff.
- [ICGT-006 model-turn v1 contract](notes/2026-08-02-icgt-006-model-turn-contract.md) records the
  schema, mapping, strict parse profile, fixture/validator evidence, review discoveries, and
  ICGT-007 handoff.
- [ICGT-007 provider contracts](notes/2026-08-03-icgt-007-provider-contracts.md) records the
  internal request/result boundary, reviewed outcome alternatives, validation evidence, and
  ICGT-008 handoff.
- [ICGT-008 deterministic fake](notes/2026-08-03-icgt-008-basic-deterministic-fake.md) records the
  strict script design, sticky safe diagnostics, validation evidence, and ICGT-009 handoff.
- [ICGT-009 bounded model-turn admission](notes/2026-08-03-icgt-009-bounded-model-turn-admission.md)
  records the strict admission design, implementation checkpoints, review findings, validation, and
  ICGT-010 handoff.
