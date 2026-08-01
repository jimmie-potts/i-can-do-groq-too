# ICGT-002 lesson: Learning through small reviewable units

- **Unit:** ICGT-002
- **Milestone:** M0 - Repository and learning foundation
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; scoped guidance, stories, lessons, and review checkpoints are validated
- **Story:** [ICGT-002](../../user-stories/icgt-002-establish-learning-standard.md)
- **Review priority:** High
- **Visual companion:** Not required for this documentation-only foundation unit
- **Related architecture:** [Root agent guidelines](../../AGENTS.md) and
  [story guidelines](../../user-stories/AGENTS.md)

> This lesson describes an implemented documentation workflow. It does not claim that FastGate code
> or a visual lesson deck exists.

## Quick summary

ICGT-002 turns “learn while Codex builds” into a review contract. Each unit is planned, predicted,
implemented, personally reviewed, adversarially reviewed, validated, and taught back before its
lesson becomes verified.

## Learning objectives

After this unit, you should be able to:

- distinguish a roadmap outcome from an implementation-ready story;
- use a human review checkpoint to focus on the most important code;
- explain why lessons change status with implementation evidence; and
- decide when a unit is too large to understand safely.

## Why this unit matters

An agent can generate more code than a person can meaningfully review. That is a throughput failure,
not a success. Small units constrain the number of new concepts and make it possible to predict
behavior, inspect the core invariant, and connect tests to design.

## Junior engineer foundation

### A story is a behavioral contract

A roadmap can say “add streaming.” A story must say which stream, who owns it, what terminal state
means, how cancellation behaves, what is bounded, and which test proves failure. The additional
detail turns ambition into reviewable work.

### Tests and lessons answer different questions

- A test asks, “Did this behavior happen for this scenario?”
- A lesson asks, “Why is the system shaped this way, where is the important code, and what trade-off
  did we choose?”

A common misconception is that generated documentation automatically improves understanding.
Understanding requires reading the exact important source, predicting outcomes, and answering
questions without the summary open.

## Key concepts

- **Review budget:** a heuristic bound on concepts and diff size, used to trigger splitting.
- **Human review checkpoint:** named important source, failure/test source, invariant, and deferrals.
- **Teach-back:** explaining the design from memory to reveal gaps in understanding.
- **Status honesty:** matching documentation claims to actual evidence.

## Architecture and invariants

```text
roadmap outcome
      -> implementation-ready story + planned lesson
      -> reviewed plan and prediction
      -> bounded implementation and tests
      -> personal review + adversarial review
      -> full gate
      -> verified implementation lesson
```

The invariant is that `Done` and `Verified against implementation` move together. A passing test is
not enough if the review map or lesson still describes only a hypothetical design.

## Practical walkthrough

1. Start at [the story index](../../user-stories/README.md), not the long-range backlog.
2. Read the story exclusions before proposing a plan.
3. Write a prediction: important types, state owner, failure behavior, and first red test.
4. Review only one bounded implementation unit.
5. Use the checkpoint to trace production and failure paths personally.
6. Update the lesson with exact paths, observed surprises, and three teach-back questions.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [Root AGENTS.md](../../AGENTS.md) | Defines repository-wide small-unit and review behavior | What must happen before and after code generation? |
| [Story guidelines](../../user-stories/AGENTS.md) | Defines when an outcome is implementation-ready | What information prevents scope drift? |
| [Lesson template](lesson-template.md) | Connects code evidence to understanding | What must change when a lesson becomes verified? |

## Implementation code samples

This is a documentation unit. No application code exists. Its exact implementation artifacts are
the guidelines, story template, lesson template, and initial one-to-one mappings.

## Failure scenarios to study

- **One story adds service, contracts, streaming, provider, and dashboard:** split it because no
  single invariant can be reviewed deeply.
- **Lesson links pseudocode after code ships:** the story remains In progress until exact source and
  tests replace it.
- **Diff fits a line budget but adds five abstractions:** split by concept; line count is only a
  heuristic.
- **Agent review replaces personal review:** require the user to trace the checkpoint paths and
  answer teach-back questions.

## What changed during implementation

The workflow adapts the strong one-story/one-lesson pattern from Code Assist Harness but avoids
creating a visual deck for every planning stub. Visual companions are required for high-priority
completed units only, after code and written evidence stabilize.

## Production expansion

Teams may add CODEOWNERS, architectural review groups, generated API diffs, change-risk scoring, and
release evidence. Those improve coordination but add process cost. The graduation signal is not
repository size alone; it is repeated missed ownership or compatibility changes across contributors.

## Practical exercises

- Split “build streaming gateway” into six stories, each with one failure path.
- Read ICGT-005 and predict the first three production functions without asking Codex.
- Answer a lesson's teach-back questions a day later without reopening the lesson.

## Key takeaways

- Story size is constrained by human understanding, not agent throughput.
- A completed story and its implementation-backed lesson are one delivery unit.
- Extra review automation supports personal review; it does not replace it.

## Glossary

- **Acceptance criterion:** observable condition required for delivery.
- **Review checkpoint:** focused paths and questions for personal code inspection.
- **Teach-back:** an explanation produced from understanding rather than copied text.

## Teach-back questions

1. What makes a roadmap outcome different from an implementation-ready story?
2. Why can a passing test suite still leave a learning story incomplete?
3. When should the 400-line review heuristic be ignored, and what must be explained first?

## Further reading

- [ICGT-002 delivery contract](../../user-stories/icgt-002-establish-learning-standard.md)
- [Unit lesson index](README.md)
- [Repository roadmap](../roadmap.md)
