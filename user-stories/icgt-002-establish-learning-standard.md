# ICGT-002 - Establish the learning and personal-review standard

- **Status:** Done
- **Milestone:** M0 - Repository and learning foundation
- **Dependencies:** ICGT-001
- **Lesson:** [Learning workflow](../docs/lessons/icgt-002-learning-workflow.md)
- **Review priority:** High

## User story

> As a learner using Codex to implement unfamiliar infrastructure, I want each unit to be small and
> paired with an implementation-backed lesson so that I can personally review and explain the most
> important code.

## Scope

- Define one-story-at-a-time planning and review behavior.
- Add a one-to-one implementation-ready story-to-lesson rule.
- Add human review checkpoints to story contracts.
- Define status-honest lesson evidence and teach-back requirements.
- Create root and scoped `AGENTS.md` guidance.
- Define when a visual lesson companion is required.

## Acceptance criteria

1. Every implementation-ready story links exactly one lesson.
2. The first six stories identify review priority and a human review checkpoint.
3. Root guidance requires a code-free plan, small diff, focused review, separate review, full gate,
   and lesson update.
4. Completed code lessons require exact important production and failure/test paths.
5. Planned lessons label pseudocode and future behavior honestly.
6. High-priority completed units require a rendered and inspected visual companion.
7. Story and lesson templates contain three teach-back questions.

## Human review checkpoint

- **Production path:** None; this unit governs future implementation review.
- **Failure/test path:** Review status-parity and teach-back enforcement in the repository policy
  tests.
- **Invariant:** A story cannot be Done while its lesson still describes only planned behavior.
- **Deferred:** The first visual deck and its rendering toolchain arrive with the first completed
  high-priority code unit.

## Validation

- Run `./scripts/check`.
- Verify every linked implementation-ready lesson exists.
- Compare lesson status with linked story status.

## Documentation impact

Creates the lesson index, template, scoped lesson guidance, initial companions, and review rules.

## Out of scope

- Creating a presentation before code exists.
- Mandating a specific diagram or deck style.
- Replacing human review with generated summaries.
