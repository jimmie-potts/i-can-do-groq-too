# ICGT-008 lesson: A basic deterministic fake upstream

- **Unit:** ICGT-008
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; no fake upstream exists
- **Story:** [ICGT-008](../../user-stories/icgt-008-build-basic-fake-upstream.md)
- **Review priority:** High
- **Visual companion:** Planned after implementation
- **Related architecture:** [ADR 0002](../adr/0002-fake-first-openai-first-live.md) and
  [FastGate agent guidelines](../../gateway/AGENTS.md)

> The script concepts below are planned. No Go API has been selected.

## Quick summary

This unit will implement a non-streaming programmable upstream that is stricter than a canned mock.
It verifies exact inputs, models one result or enumerated failure, and proves no expected interaction
was skipped.

## Learning objectives

You should be able to distinguish fake, mock, and live adapter roles; explain safe exact matching;
and use exhaustion verification to catch incomplete interactions.

## Why this unit matters

Rare request/failure interactions are expensive to force through a live service. A deterministic
fake makes exact success, failure, mismatch, and omitted-work states ordinary, cheap tests. A later
story adds streaming gates and cancellation only after an event grammar exists.

## Junior engineer foundation

A canned stub returns the same value. A strict fake implements the real port and models behavior. It
can reject an unexpected request, return an expected result or failure, and prove teardown consumed
the full expected script.

A common misconception is that a fake should approximate an entire vendor API. It should implement
the small project-owned port and be strict about the interaction the current unit actually needs.

## Key concepts

- **Exact expectation:** a complete reviewed input match.
- **Exhaustion assertion:** teardown proof that every expected exchange and observation was consumed.
- **Safe mismatch diagnostic:** bounded location information without request content.

## Architecture and invariants

```text
test -> expected exchanges -> Fake upstream invocation -> one controlled result/failure
  |                                                               |
  +--------------------- assert script complete <-----------------+
```

Streaming, single-consumer async state, logical gates, and cancellation are deliberately deferred to
the later event-grammar and concurrency-fake units.

## Practical walkthrough

The implementation should begin with one exact successful exchange, then add enumerated failure,
mismatch, and exhaustion errors. Each state is covered before the fake is used by an HTTP endpoint.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Planned exchange validation | Prevents ambiguous success/failure scenarios | Is every exchange exactly terminal? |
| Planned exhaustion tests | Prevent false-positive tests | Does success prove the whole interaction occurred? |

## Implementation code samples

None. Add exact fake and test excerpts after implementation.

## Failure scenarios to study

- Actual request differs in a secret-bearing field: report only a bounded field path.
- Test forgets to call the expected request: exhaustion fails at teardown.
- Test omits an expected exchange: the remaining script makes verification fail.

## What changed during implementation

No implementation evidence exists yet. The design deliberately borrows the strict-fake lesson from
Code Assist Harness while implementing only the ICGT-007 provider contract.

## Production expansion

Later units add logical streaming gates, cancellation checkpoints, transport fakes, provider sandbox
accounts, recorded conformance traces, and fault proxies. Those test different boundaries at higher
cost or complexity.

## Practical exercises

- Write exact exchanges for one result, one failure, and one mismatch.
- Explain how a fake can verify a request without printing mismatched content.
- Explain why streaming gates should wait for the stream-event contract.

## Key takeaways

- The fake is a strict implementation of the production port, not a hard-coded answer.
- Exact matching and fixed failures make interaction repeatable.
- Exhaustion verification proves expected interaction as well as expected output.

## Glossary

- **Fake:** working test implementation of a real interface.
- **Exhaustion:** proof that no expected work remains.

## Teach-back questions

1. What does a strict fake prove that a canned return value cannot?
2. Why must mismatch diagnostics omit request content?
3. When should a later logical-gate fake be added instead of expanding this unit?

## Further reading

- [ICGT-008 delivery contract](../../user-stories/icgt-008-build-basic-fake-upstream.md)
- [ADR 0002](../adr/0002-fake-first-openai-first-live.md)
