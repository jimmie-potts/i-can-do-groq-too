# Project brief

- **Source:** [Public ChatGPT conversation - Groq Cloud Overview](https://chatgpt.com/share/6a6d095c-6a34-83ea-af53-5f146b2c8acb)
- **Recovered:** 2026-07-31
- **Status:** Context summary; accepted choices are recorded separately in ADRs

## Goal

Build a learning-oriented set of inference infrastructure projects with Codex as a coding partner,
while keeping the user responsible for architecture and able to explain the important code. The
projects should be individually useful, connect into a small inference platform, and build skills
relevant to GroqCloud and similar high-performance inference systems.

## Proposed system

```text
Code Assist Harness and other clients
                  |
                  v
              FastGate <------ LatencyLab

TenantPlane -------- policy projection --------> FastGate
TenantPlane <------- idempotent usage events ---- FastGate

ModelEndpoint Operator -- reconciles FastGate, benchmark runners,
                          and later self-hosted endpoints

FleetSim consumes measured scenarios and remains offline.
```

The conversation proposed five projects:

1. **FastGate:** a production-style inference gateway.
2. **LatencyLab:** an inference-aware benchmark and chaos platform.
3. **ModelEndpoint Operator:** a Kubernetes operator for endpoint configuration and lifecycle.
4. **TenantPlane:** a multi-tenant inference control plane.
5. **FleetSim:** a deterministic global capacity and scheduling simulator.

Go is the default backend and infrastructure language. TypeScript is optional for a visualization
UI, not for the core control or data planes.

## Learning contract

The repository must favor active learning over one-shot generation:

- one approved, dependency-ready story at a time;
- a reviewed plan before implementation;
- small explainable changes;
- explicit failure modes and tests;
- a personal diff-review checkpoint;
- a separate adversarial review; and
- a written lesson updated from planned concepts to exact implementation evidence.

The user specifically wants to review the most important code. Stories and lessons therefore name
the production path, failure/test path, invariant, and deliberate deferrals that deserve personal
attention.

## Relationship to Code Assist Harness

The later part of the conversation clarified that `code-assist-harness` starts with a handwritten
OpenAI Responses loop and expands to FastGate and Groq later. This does not merge the repositories.

- The harness owns task history, model-turn orchestration, tools, approvals, audit, and evaluation.
- FastGate owns model transport, provider capabilities, provider/network retries, routing, and
  operational telemetry.
- The harness calls OpenAI directly first, then gains a separate FastGate adapter.
- OpenAI is the first live FastGate baseline. Groq follows for limited, measured task categories.
- Provider state such as `previous_response_id` is optional adapter metadata, never the harness's
  durable source of truth.
- Core coding tools remain harness-controlled rather than provider-hosted.

## Caching direction

The project does not attempt to manage Groq's internal KV tensors. The learning progression is:

1. stable prompt prefixes and provider-managed cache metrics;
2. self-hosted vLLM automatic prefix caching;
3. cache-aware routing that balances affinity against queue depth and load; and
4. a raw KV-cache manager only as a separate advanced runtime project, if still valuable.

Cache isolation must eventually consider tenant, model, tokenizer, adapter/version, and security
domain. Cache state never replaces task or audit state.

## Framework direction

LangChain and LangGraph are not foundations for the five infrastructure projects. FastGate,
LatencyLab, the operator, TenantPlane, and FleetSim require deterministic systems behavior that is
clearer in ordinary Go.

The harness keeps its handwritten loop. LangGraph may be evaluated later only after the explicit
state machine exists and real traces demonstrate a benefit. Such a change requires its own ADR and
must preserve harness-owned state, tools, approval, audit, and evaluation boundaries.

## Decisions intentionally left open

The source conversation did not settle:

- the public umbrella name;
- FastGate's first external request/stream contract;
- the Go module/workspace strategy;
- license and release model;
- migration, dashboard, and deployment tooling; or
- which stories require visual lesson decks beyond written lessons.

This repository resolves only the decisions required by each small implementation unit.
