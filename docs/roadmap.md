# Roadmap

**Status:** Planned outcomes. M0 and ICGT-004 through ICGT-009 are implemented. ICGT-010 is the next
implementation-ready unit; HTTP presentation, service binding, and runtime concurrency remain
unimplemented.

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

- ICGT-004 (Done) selected Go 1.26.5 and one future root module without creating source;
- ICGT-005 (Done) materialized the selected root-module scaffold and bootstrapped the service
  lifecycle;
- ICGT-006 (Done) defines the non-streaming v1 schema, mapping, fixtures, parse profile, extension
  versioning rule, and offline validator for the FastGate-owned protocol selected in ADR 0003;
- ICGT-007 (Done) defines only the provider-neutral non-streaming gateway contract required by the
  next fake;
- ICGT-008 (Done) implements the basic non-streaming deterministic fake upstream; and
- ICGT-009 (Done) implements bounded strict admission and exactly one injected provider-neutral
  invocation, with deterministic-fake evidence and no HTTP binding; and
- ICGT-010 (Planned) defines one injectable model-turn HTTP handler, exhaustive outcome
  presentation, transport preflight, and real-loopback integration evidence without mounting the
  handler in the executable.

The remaining units keep service binding, stream grammar, and deterministic concurrency/cancellation
behavior separate so each review has one primary concept. ICGT-010 owns HTTP presentation and proves
that wrong target, method, encoding, or media type causes zero provider calls. ICGT-011 later owns
mounting the route, rejecting a non-loopback actual listener, selecting a safe runtime provider or
demo policy, and bounding concurrent work. The default command remains health-only through
ICGT-010.

### M2 - Streaming reliability

**Outcome:** FastGate streams incrementally while preserving cancellation, bounded timeouts,
backpressure, structured failures, cleanup, and observable latency.

Expected small units include SSE framing, fake streaming, client cancellation, upstream deadline,
bounded FastGate-local reaping after cleanup grace, slow-client bounds, and basic metrics. Each
becomes implementation-ready only when the previous contract is observed in code.

The asynchronous provider-operation, cancellation, and cleanup-barrier contract is introduced only
when the stream grammar can exercise it. Concrete provider-SDK dependency enforcement waits for the
first concrete adapter.

### M3 - Live provider baseline and harness handoff

**Outcome:** OpenAI is the first opt-in live upstream, direct-versus-gateway infrastructure behavior
is measured, and FastGate publishes a versioned handoff and conformance bundle that a later
Code Assist Harness-owned adapter can consume without weakening its direct OpenAI adapter.

Groq follows only after the OpenAI baseline and deterministic contract tests exist. Initial Groq
tasks are narrow and evaluated independently.

The handoff sequence is intentionally explicit:

1. ICGT-020 compares direct-provider and gateway wire semantics, normalized outcomes, retry and
   cancellation behavior, and gateway overhead using a repository-owned measurement client against
   the same bounded workload. It does not depend on a harness adapter or claim coding-task
   correctness parity.
2. ICGT-021 starts only after ICGT-020 and a newly audited adapter-ready harness contract snapshot.
   The handoff pins that immutable harness revision/provider contract plus the exact FastGate
   schema/fixture versions and source artifacts; it does not assume the historical ICGT-006 snapshot
   is still sufficient.
3. ICGT-021 defines a candidate loopback-only, unauthenticated, no-tool profile: FastGate guarantees
   its implemented server contract; Code Assist Harness later accepts and owns endpoint, mapping,
   failure/retry, cancellation, and local-cleanup policy for its adapter. A non-loopback profile waits
   for a separately implemented and conformance-tested FastGate authentication/TLS story.
4. ICGT-022 packages and validates the exact frozen artifacts, records the bundle manifest/digest,
   and may not change semantics without a new contract version and handoff review.
5. A later Code Assist Harness story owns `FastGateProvider`, pins that manifest/digest, and accepts
   the client side of the joint no-tool profile. Full coding-agent parity additionally requires a
   separately reviewed, versioned FastGate tool extension and compatible harness contract, with the
   harness remaining correctness authority.

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

The detailed, linked story list now ends at the planned ICGT-010 HTTP-presentation unit. ICGT-011
remains an outcome slice until its actual-listener, runtime-provider, and bounded-concurrency
decisions are promoted into a reviewed story and lesson. Later rows in
[the backlog](../user-stories/backlog.md) are outcome slices, not promises that their contracts are
ready. Before promoting one, create its story and lesson, lock dependencies and exclusions, and name
the exact human review checkpoint.
