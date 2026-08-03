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
- Define when an optional visual lesson companion may be selected and how it is verified.

## Acceptance criteria

1. Every implementation-ready story links exactly one lesson.
2. The first six stories identify review priority and a human review checkpoint.
3. Root guidance requires a code-free plan, small diff, focused review, separate review, full gate,
   and lesson update.
4. Completed code lessons require exact important production and failure/test paths.
5. Planned lessons label pseudocode and future behavior honestly.
6. Every implementation-ready unit requires its source-backed Markdown lesson. A visual companion
   never gates Done and is created only after an explicit user request or a separately reviewed
   learning justification; if selected, it is rendered and inspected after written evidence is
   stable.
7. Story and lesson templates contain three teach-back questions.

## Human review checkpoint

- **Production path:** None; this unit governs future implementation review.
- **Failure/test path:** Review status-parity and teach-back enforcement in the repository policy
  tests.
- **Invariant:** A story cannot be Done while its lesson still describes only planned behavior.
- **Deferred:** No visual is promised for a future unit. A selected visual waits for stable code and
  written evidence, then uses the reviewed rendering and inspection workflow.

## Validation

- Run `./scripts/check`.
- Verify every linked implementation-ready lesson exists.
- Compare lesson status with linked story status.
- Verify the story, lesson, root/scoped guidance, templates, and indices agree that visuals are
  optional.

## Documentation impact

Creates the lesson index, template, scoped lesson guidance, initial companions, and review rules.

## Out of scope

- Creating a presentation before code and written evidence are stable.
- Treating a visual as a completion requirement without a new reviewed policy decision.
- Mandating a specific diagram or deck style.
- Replacing human review with generated summaries.
