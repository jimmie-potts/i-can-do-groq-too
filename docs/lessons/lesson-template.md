# ICGT-XXX lesson: Unit title

- **Unit:** ICGT-XXX
- **Milestone:** M0, M1, or later
- **Lesson status:** Planned, Implementation companion, Implementation companion - blocked, or
  Verified against implementation
- **Implementation status:** Match the linked story
- **Story:** Link the delivery contract
- **Review priority:** High or Normal
- **Visual companion:** Default to Not required; link one only when explicitly requested or separately justified
- **Related architecture:** Link the relevant ADR and conceptual document

> State whether this lesson describes accepted design, planned behavior, or observed implementation.

## Quick summary

Explain what the unit builds, what it teaches, and the boundary it creates for later work.

## Learning objectives

After completing this unit, you should be able to:

- explain the primary concept in plain language;
- identify the component that owns the decision;
- trace the important production and failure paths; and
- compare the local design with a justified production expansion.

## Why this unit matters

Explain which later work depends on this unit and which ambiguity or failure it prevents.

## Junior engineer foundation

Define prerequisite ideas, show one tiny concrete example, and correct at least one common beginner
misconception.

## Key concepts

Define the important concepts and connect each one to this repository.

## Architecture and invariants

Describe ownership, data flow, state, bounds, cleanup, and deliberately deferred work. Use a small
diagram or table only when it clarifies a relationship.

## Practical walkthrough

Describe the intended implementation sequence and observable evidence without duplicating every
acceptance criterion.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Important production path | Replace with exact file links after code exists | What owns this behavior? |
| Failure or test path | Replace with exact file links after code exists | What safe outcome is proved? |

## Implementation code samples

After code exists, include focused exact excerpts for the production path and one failure/test path.
Link each excerpt and explain it line-by-line or in small logical chunks. Planned lessons may use
clearly labeled pseudocode only.

## Failure scenarios to study

Explain the symptom, responsible boundary, safe outcome, and deterministic test evidence.

## What changed during implementation

Record failed assumptions, observed constraints, design changes, and why the final approach won.
Planned lessons state that implementation evidence does not exist yet.

## Production expansion

### Example production scenario

Describe a realistic scale, reliability, security, or organizational requirement beyond this unit.

### Representative capabilities and tools

Name three to five official, relevant examples without implying they are dependencies.

### Local versus production

| Dimension | This repository | Production expansion |
| --- | --- | --- |
| Scope | One small learning behavior | Multi-team or high-scale behavior |
| Reliability | Deterministic local evidence | Redundancy, recovery, and operations |
| Cost | Low cognitive and service cost | Additional systems and ownership |

### Trade-offs and graduation signals

Explain the benefit, operational cost, and measurable signal that would justify expansion.

## Practical exercises

- Add one focused experiment.
- Predict one failure before running its test.
- Compare one alternative without implementing it.

## Key takeaways

- State the ownership rule.
- State the important invariant.
- State the production trade-off.

## Glossary

Define local lesson terms and link the shared [glossary](../glossary.md).

## Teach-back questions

1. Ask one ownership question.
2. Ask one failure/invariant question.
3. Ask one trade-off or production-expansion question.

## Further reading

Link the story, architecture/ADR, and official references for named tools or protocols.
