# Architecture decision records

ADRs record decisions that constrain multiple stories or components. They preserve alternatives and
supersession instead of letting current code become the only explanation.

## Status vocabulary

| Status | Meaning |
| --- | --- |
| Proposed | The decision and criteria are open for review. |
| Accepted | New work must follow the decision. |
| Superseded | A later ADR replaces the decision; retain it as history. |

## Index

| ADR | Status | Decision |
| --- | --- | --- |
| [0001](0001-separate-learning-monorepo.md) | Accepted | Keep the inference platform separate from Code Assist Harness. |
| [0002](0002-fake-first-openai-first-live.md) | Accepted | Prove provider contracts with a fake; use OpenAI as the first live baseline. |
| [0003](0003-fastgate-api-surface.md) | Accepted | Use a small FastGate-owned model-turn protocol as the first client contract. |
| [0004](0004-go-toolchain-and-module-strategy.md) | Accepted | Use Go 1.26.5 and one root module with offline local-toolchain validation. |
