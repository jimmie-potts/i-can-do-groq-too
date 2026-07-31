# Roadmap

**Status:** Planned outcomes. Only M0 foundation is currently implemented.

## Delivery principles

- Finish dependency-ready units in order.
- Keep one important behavior and one failure mode per code story.
- Add a provider or framework only after a deterministic boundary exists.
- Keep the default gate offline and credential-free.
- Update lessons from planned concepts to exact implementation evidence before marking a unit Done.

## Milestones

### M0 - Repository and learning foundation

**Outcome:** A status-honest monorepo with architecture, ADRs, stories, scoped agent guidance,
lessons, Git hygiene, and an offline gate.

Stories ICGT-001 through ICGT-003 are delivered by the initial bootstrap.

### M1 - FastGate non-streaming walking skeleton

**Outcome:** A small Go service accepts one reviewed request contract, calls a deterministic fake
upstream, returns one normalized non-streaming result, and proves a meaningful failure path.

Reviewed M1 unit sequence:

- ICGT-004 select the Go toolchain and module strategy without creating source;
- ICGT-005 bootstrap the service lifecycle;
- ICGT-006 select the client protocol and accept or replace ADR 0003;
- ICGT-007 define provider-neutral gateway contracts from that client contract; and
- ICGT-008 build the basic non-streaming deterministic fake upstream.

Later units split endpoint mapping, normalized transport failures, stream grammar, and deterministic
concurrency/cancellation behavior so each review has one primary concept.

### M2 - Streaming reliability

**Outcome:** FastGate streams incrementally while preserving cancellation, bounded timeouts,
backpressure, structured failures, cleanup, and observable latency.

Expected small units include SSE framing, fake streaming, client cancellation, upstream deadline,
slow-client bounds, and basic metrics. Each becomes implementation-ready only when the previous
contract is observed in code.

### M3 - Live provider baseline and harness handoff

**Outcome:** OpenAI is the first opt-in live upstream, direct-versus-gateway behavior is measured,
and a separate FastGate adapter can be added to Code Assist Harness without weakening its direct
OpenAI adapter.

Groq follows only after the OpenAI baseline and deterministic contract tests exist. Initial Groq
tasks are narrow and evaluated independently.

### M4 - LatencyLab

**Outcome:** Reproducible workloads measure TTFT, streaming duration, total latency, percentiles,
errors, retries, gateway overhead, and task quality/cost joins. Fault experiments cover saturation,
slow clients, retry storms, and tail-latency incidents.

### M5 - ModelEndpoint Operator

**Outcome:** A Go controller reconciles a versioned endpoint resource into FastGate-facing
deployments and configuration with idempotency, status, finalizers, RBAC, and failure tests.

### M6 - TenantPlane

**Outcome:** Organizations, projects, keys, permissions, quotas, budgets, idempotent usage events,
an append-only ledger, and audit records form a separate control plane. Redis is added only after a
measured hot-path need.

### M7 - FleetSim

**Outcome:** A deterministic discrete-event simulator compares scheduling, reservations, fairness,
failure domains, model placement, and cache-affinity strategies using measured scenario inputs.

## Provider progression

```text
deterministic FastGate fake
        -> OpenAI live baseline
        -> direct versus FastGate comparison
        -> Groq limited workloads
        -> capability-aware routing
        -> optional self-hosted runtime
```

The deterministic fake is the first implementation. OpenAI is the first live provider. Those
statements are complementary, not competing sequences.

## Framework and caching progression

- Do not add LangChain or LangGraph to the infrastructure cores.
- Preserve the handwritten harness loop; evaluate a framework only through a later evidence-backed
  harness ADR.
- Begin with stable prompt prefixes and provider-reported cache metrics.
- Add vLLM prefix caching only with a self-hosted runtime story.
- Add cache-affinity routing only after LatencyLab can measure both benefit and queueing cost.

## Detailed-planning boundary

The detailed, linked planned-story list ends at ICGT-008 on purpose. Only the first dependency-ready
unit may be started. Later rows in
[the backlog](../user-stories/backlog.md) are outcome slices, not promises that their contracts are
ready. Before promoting one, create its story and lesson, lock dependencies and exclusions, and
name the exact human review checkpoint.
