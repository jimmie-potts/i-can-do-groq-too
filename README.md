# I Can Do Groq Too

**Inference Cloud Lab / Agent Infrastructure Kit**

This repository is a learning-first monorepo for building the infrastructure around model
inference: a gateway, a benchmark and chaos lab, a Kubernetes operator, a multi-tenant control
plane, and a fleet simulator. The five projects are intentionally independent, but they can grow
into one small, observable inference platform.

## Current status

The repository foundation is implemented: architecture boundaries, delivery stories, lesson
standards, component briefs, Git hygiene, and an offline documentation quality gate exist. No
service, provider adapter, database, Kubernetes controller, or simulator has been implemented yet.

[ICGT-004](user-stories/icgt-004-select-go-toolchain.md) is the next dependency-ready decision
unit. It selects the Go toolchain and module boundary without installing software or creating source.
[ICGT-005](user-stories/icgt-005-bootstrap-fastgate-service.md) is the first planned code-bearing
unit and starts only after that decision and local toolchain availability are reviewed.

## What we are building

| Order | Component | Responsibility | Status |
| ---: | --- | --- | --- |
| 1 | [FastGate](gateway/README.md) | Provider transport, streaming, routing, limits, and operational telemetry | Planned |
| 2 | [LatencyLab](latency-lab/README.md) | Inference-aware load, latency, and failure experiments | Planned |
| 3 | [ModelEndpoint Operator](model-operator/README.md) | Kubernetes reconciliation for inference-facing services | Planned |
| 4 | [TenantPlane](control-plane/README.md) | Identity, authorization, quota, budget, usage, and audit control plane | Planned |
| 5 | [FleetSim](fleet-simulator/README.md) | Deterministic capacity and scheduling simulation | Planned |

The dependency order is deliberate. FastGate gives us something real to operate. LatencyLab
measures and breaks it. The operator deploys and reconciles it. TenantPlane makes it safe for
multiple customers. FleetSim explores capacity decisions that would be too expensive to test on a
real accelerator fleet.

## Relationship to Code Assist Harness

[`code-assist-harness`](https://github.com/jimmie-potts/code-assist-harness) remains a separate client application and
learning project. It owns coding-agent behavior; this repository owns inference-platform behavior.

| Concern | Owner |
| --- | --- |
| Task state, agent turns, tools, approvals, transcripts, and correctness evaluation | Code Assist Harness |
| Provider transport, streaming, network retries, routing, rate limits, and provider telemetry | FastGate |
| Performance, saturation, and fault experiments | LatencyLab |
| Identity, permissions, quotas, budgets, usage ledger, and audit | TenantPlane |
| Deployment and reconciliation | ModelEndpoint Operator |
| Offline scheduling and capacity policy experiments | FleetSim |

At this repository's bootstrap, the harness already has a provider-neutral asynchronous port and a
strict deterministic fake. Its next planned units prove one provider-neutral turn and hard limits
before activating its direct OpenAI Responses adapter. We reuse those lessons here: define a small
port, prove it with a fake, keep vendor types at the edge, and make cleanup and cancellation part of
the contract.

FastGate must not block the harness. The intended evolution is:

```text
Code Assist Harness -> direct OpenAI adapter
Code Assist Harness -> FastGate adapter -> OpenAI
Code Assist Harness -> FastGate adapter -> OpenAI or Groq
```

The harness continues to own workflow state even when FastGate selects the provider. Groq support
belongs in FastGate rather than being spread throughout the harness.

The harness MVP currently excludes Git-state mutation. If a future reviewed harness story adds
branch, worktree, commit, or pull-request behavior, that remains a harness concern rather than a
FastGate feature.

See [Architecture](docs/architecture.md) for the detailed boundary and the open API-contract
question.

## Learning and review workflow

Every implementation-ready unit has one delivery story and one learning companion. Units are kept
small enough for a human to review the important production path and at least one failure or test
path.

For each unit:

1. Read the story and its planned lesson.
2. Review a code-free implementation plan.
3. Predict the important types, state, failure behavior, and tests.
4. Implement one bounded behavior.
5. Review the diff personally, with special attention to the story's review checkpoint.
6. Run a separate correctness review and the complete repository gate.
7. Replace planned lesson material with exact repository-backed code samples and a teach-back.

The [user-story index](user-stories/README.md) contains the dependency order. The
[lesson index](docs/lessons/README.md) explains the evidence required before a lesson can be called
verified.

## Repository layout

```text
.
├── gateway/              FastGate scope and component-specific guidance
├── latency-lab/          LatencyLab scope
├── model-operator/       ModelEndpoint Operator scope
├── control-plane/        TenantPlane scope
├── fleet-simulator/      FleetSim scope
├── docs/                 Architecture, ADRs, roadmap, glossary, and lessons
├── user-stories/         Dependency-ordered delivery contracts and notes
├── scripts/              Offline repository checks
├── AGENTS.md             Repository-wide engineering and learning rules
└── README.md             Current status and project map
```

Paths are introduced only when they contain a current document, test, or implementation. We do not
create empty planned source trees.

## Run the current quality gate

The foundation has no application dependencies. It requires Git, Bash, and Python 3.11 or later:

```bash
./scripts/check
```

The gate is offline. It checks repository text hygiene, required foundation files, local Markdown
links, story/lesson parity, review metadata, checker regression tests, and Git whitespace rules. Go
formatting and tests become mandatory when the reviewed root module or workspace exists; further Go
lint/race stages arrive with the story that can validate them honestly.

## Provider and framework direction

- A deterministic fake will precede either live provider, while OpenAI will be the first live
  FastGate provider and Groq will follow after measurable baseline behavior exists.
- Provider differences are explicit capabilities. Unsupported behavior is rejected, emulated, or
  rerouted; it is never silently ignored.
- FastGate, LatencyLab, the operator, TenantPlane, and FleetSim do not use LangChain or LangGraph as
  their foundation. The harness keeps its handwritten loop unless later evidence justifies a
  separate framework ADR.
- Hosted providers own their internal KV tensors. This repository first studies stable-prefix
  prompt caching and cache metrics. Raw KV-cache management is deferred to a future self-hosted
  runtime experiment.

## Open decisions

The following choices are intentionally unresolved:

- whether FastGate's first public contract is a Responses-compatible subset, a Chat Completions
  subset, or a small project-owned model-turn protocol with compatibility facades;
- one root Go module versus a `go.work` workspace with component modules;
- the public product name beyond the repository name;
- the license, database migration tool, optional dashboard stack, and deployment packaging.

Each decision will be resolved by the story that first needs it. See
[ADR 0003](docs/adr/0003-fastgate-api-surface.md) for the API decision criteria.

## Configuration and secrets

There is no live-provider configuration yet. Do not create or commit an `.env` file. When a story
introduces configuration, document variable names in a reviewed `.env.example` with blank or
unmistakably fake values. Tests and the default quality gate must remain credential-free and
network-free.

## Project context

The originating public conversation is summarized in [Project brief](docs/project-brief.md). A
public share link is sufficient context, but an attached Markdown or text export is more reliable
for future long conversations because it avoids browser-rendering dependencies.

If Codex browser control fails from this WSL workspace with `sandboxCwd is not a local file URI`,
see [Troubleshooting](docs/troubleshooting.md). That is a Codex Desktop path-translation issue, not a
repository setup failure.
