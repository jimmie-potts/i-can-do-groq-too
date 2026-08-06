# I Can Do Groq Too

**Inference Cloud Lab / Agent Infrastructure Kit**

This repository is a learning-first monorepo for building the infrastructure around model
inference: a gateway, a benchmark and chaos lab, a Kubernetes operator, a multi-tenant control
plane, and a fleet simulator. The five projects are intentionally independent, but they can grow
into one small, observable inference platform.

## Current status

The repository foundation, Go toolchain decision,
[ICGT-005 FastGate lifecycle](user-stories/icgt-005-bootstrap-fastgate-service.md),
[ICGT-006 model-turn v1 contract](user-stories/icgt-006-select-fastgate-api.md),
[ICGT-007 provider contract](user-stories/icgt-007-define-provider-contracts.md), and
[ICGT-008 deterministic fake](user-stories/icgt-008-build-basic-fake-upstream.md), and
[ICGT-009 bounded admission](user-stories/icgt-009-admit-and-execute-model-turn.md) are implemented.
The root Go module contains one standard-library-only executable that serves operational health and
shuts down cleanly. FastGate also owns strict language-neutral request, result, and failure schemas;
a harness mapping; 26 fixtures; a dependency-free offline contract validator; and bounded internal
provider request, result, failure, and invocation contracts. A strict in-memory fake implements that
port with ordered exact matching, controlled terminal outcomes, and complete-script verification.
The internal model-turn executor bounds and strictly admits v1 request bytes, rejects unsupported
capabilities and aliases before dispatch, and validates exactly one admitted provider return.

No inference request endpoint or live provider adapter, database, Kubernetes controller, or
simulator has been implemented yet. The admission executor and fake are internal seams, not a
client-reachable inference service. ICGT-010 remains an outcome slice until its loopback-only HTTP
contract, concurrency policy, and review checkpoints are promoted into a reviewed story and lesson.

## What we are building

| Order | Component | Responsibility | Status |
| ---: | --- | --- | --- |
| 1 | [FastGate](gateway/README.md) | Provider transport, streaming, routing, limits, and operational telemetry | Lifecycle, v1 wire contract, provider port/fake, and bounded admission implemented |
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
the contract. FastGate owns its versioned wire schema and language-neutral fixtures; Code Assist
Harness owns any client adapter that consumes them.

FastGate must not block the harness. The intended evolution is:

```text
Code Assist Harness -> direct OpenAI adapter
Code Assist Harness -> CAH-owned FastGate adapter -> FastGate -> OpenAI
Code Assist Harness -> CAH-owned FastGate adapter -> FastGate -> OpenAI or Groq
```

The harness continues to own workflow state even when FastGate selects the provider. Groq support
belongs in FastGate rather than being spread throughout the harness.

The harness MVP currently excludes Git-state mutation. If a future reviewed harness story adds
branch, worktree, commit, or pull-request behavior, that remains a harness concern rather than a
FastGate feature.

See [Architecture](docs/architecture.md) for the detailed boundary and the remaining client-contract
work.

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
6. Apply the [PR review regression checklist](docs/pr-review-checklist.md), run a separate
   correctness review, and run the complete repository gate.
7. Replace planned lesson material with exact repository-backed code samples and a teach-back.

The [user-story index](user-stories/README.md) contains the dependency order. The
[lesson index](docs/lessons/README.md) explains the evidence required before a lesson can be called
verified.

## Repository layout

```text
.
├── gateway/              FastGate lifecycle, contracts, fake, and bounded admission
├── latency-lab/          LatencyLab scope
├── model-operator/       ModelEndpoint Operator scope
├── control-plane/        TenantPlane scope
├── fleet-simulator/      FleetSim scope
├── docs/                 Architecture, ADRs, roadmap, glossary, and lessons
├── user-stories/         Dependency-ordered delivery contracts and notes
├── scripts/              Offline repository checks
├── go.mod                One reviewed Go module and version source of truth
├── AGENTS.md             Repository-wide engineering and learning rules
└── README.md             Current status and project map
```

Paths are introduced only when they contain a current document, test, or implementation. We do not
create empty planned source trees.

## Run the current quality gate

The service uses only the Go standard library. Validation requires Git, Bash, Python 3.11 or later,
Go 1.26.5 or later, cgo, and a working C compiler for race tests:

```bash
./scripts/check
```

The gate is offline. It checks repository hygiene, local Markdown links, story/lesson parity, the
strict model-turn schema and complete fixture corpus, exact Go module and CI policy, formatting,
vet, ordinary tests, race prerequisites, and race tests. Toolchain installation and CI job
preparation occur before the gate; the gate itself does not download tools or modules.

## Provider and framework direction

- [ADR 0003](docs/adr/0003-fastgate-api-surface.md) selects a small FastGate-owned model-turn
  protocol as the first public contract. ICGT-006 materializes its exact non-streaming schema,
  bounds, mapping, fixtures, parse profile, version rule, and offline validator. Chat Completions and
  Responses compatibility facades are not part of v1. ICGT-007 defines the smaller internal
  provider port downstream of that wire contract without exposing wire or vendor types. ICGT-008
  implements the port with a strict test-only deterministic fake.
- The deterministic fake now precedes either live provider. OpenAI will be the first live FastGate
  provider, and Groq will follow after measurable baseline behavior exists.
- Provider differences are explicit capabilities. Unsupported behavior is rejected, emulated, or
  rerouted; it is never silently ignored.
- FastGate, LatencyLab, the operator, TenantPlane, and FleetSim do not use LangChain or LangGraph as
  their foundation. The harness keeps its handwritten loop unless later evidence justifies a
  separate framework ADR.
- Hosted providers own their internal KV tensors. This repository first studies stable-prefix
  prompt caching and cache metrics. Raw KV-cache management is deferred to a future self-hosted
  runtime experiment.

## Open decisions

The following choices remain intentionally unresolved:

- the public product name beyond the repository name;
- the license, database migration tool, optional dashboard stack, and deployment packaging.

Each remaining decision will be resolved by the story that first needs it.

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
