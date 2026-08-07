# Roadmap

**Status:** Planned outcomes. M0 and ICGT-004 through ICGT-010 are implemented. ICGT-011 is the
approved, implementation-ready final M1 unit; its local-demo runtime binding remains unimplemented.

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

**Outcome:** A small Go service accepts one reviewed request contract, calls a deterministic
provider-neutral implementation, returns one normalized non-streaming result, and proves a
meaningful failure path. The strict fake remains the test oracle; the runnable profile uses the
stateless demo selected in ADR 0005.

Reviewed M1 unit sequence:

- ICGT-004 (Done) selected Go 1.26.5 and one future root module without creating source;
- ICGT-005 (Done) materialized the selected root-module scaffold and bootstrapped the service
  lifecycle;
- ICGT-006 (Done) defines the non-streaming v1 schema, mapping, fixtures, parse profile, extension
  versioning rule, and offline validator for the FastGate-owned protocol selected in ADR 0003;
- ICGT-007 (Done) defines only the provider-neutral non-streaming gateway contract required by the
  next fake;
- ICGT-008 (Done) implements the basic non-streaming deterministic fake upstream;
- ICGT-009 (Done) implements bounded strict admission and exactly one injected provider-neutral
  invocation, with deterministic-fake evidence and no HTTP binding;
- ICGT-010 (Done) implements one injectable model-turn HTTP handler, exhaustive outcome presentation,
  transport preflight, context-termination abort behavior, and real-loopback integration evidence
  without mounting the handler in the executable; and
- [ICGT-011](../user-stories/icgt-011-bind-local-demo-runtime.md) (Planned) will bind the safe local
  demo runtime, enforce a concrete loopback `*net.TCPListener`, and bound concurrent model-turn work.

ICGT-010 owns HTTP presentation and proves that wrong target, method, encoding, or media type causes
zero provider calls. ICGT-011 is the final M1 service-binding unit: it selects a stateless fixed-output
demo, rejects any listener other than a concrete loopback `*net.TCPListener`, and adds fail-fast
concurrency after transport preflight with default 4, `-max-concurrent-model-turns` range 1 through
16, and no waiting queue.
Health and transport rejections bypass the gate. It adds no Host allowlist, caller authentication,
live provider, or Code Assist Harness change. The default command remains health-only until ICGT-011
is implemented. [ADR 0005](adr/0005-local-demo-runtime-profile.md) records the approved profile.

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

An opt-in live adapter may be implemented and tested in M3, but it may not be mounted in any runnable
profile—alongside or instead of the stateless demo, even on loopback—until a separate review selects
and tests the Host, Origin, CORS, DNS-rebinding, and caller-authentication threat model. Strict JSON
and absent CORS permission are not treated as security guarantees.

The handoff sequence is intentionally explicit:

1. After ICGT-019, a separate small story must select and implement the runnable live-provider
   Host/Origin/CORS/DNS-rebinding/caller-authentication profile. Its identifier is assigned when M3
   becomes dependency-ready; it must not be hidden inside adapter construction or measurement.
2. Only after that profile, ICGT-020 compares direct-provider and gateway wire semantics, normalized outcomes, retry and
   cancellation behavior, and gateway overhead using a repository-owned measurement client against
   the same bounded workload. It does not depend on a harness adapter or claim coding-task
   correctness parity.
3. ICGT-021 starts only after ICGT-020 and a newly audited adapter-ready harness contract snapshot.
   The handoff pins that immutable harness revision/provider contract plus the exact FastGate
   schema/fixture versions and source artifacts; it does not assume the historical ICGT-006 snapshot
   is still sufficient.
4. ICGT-021 defines a candidate loopback-only, unauthenticated, no-tool profile: FastGate guarantees
   its implemented server contract; Code Assist Harness later accepts and owns endpoint, mapping,
   failure/retry, cancellation, and local-cleanup policy for its adapter. A non-loopback profile waits
   for a separately implemented and conformance-tested FastGate authentication/TLS story.
5. ICGT-022 packages and validates the exact frozen artifacts, records the bundle manifest/digest,
   and may not change semantics without a new contract version and handoff review.
6. A later Code Assist Harness story owns `FastGateProvider`, pins that manifest/digest, and accepts
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
        -> stateless local demo runtime
        -> deterministic streaming and cancellation
        -> opt-in OpenAI adapter through non-runnable test seams
        -> reviewed runnable-live security profile
        -> OpenAI live gateway baseline
        -> direct versus FastGate comparison
        -> Groq limited workloads
        -> capability-aware routing
        -> optional self-hosted runtime
```

The deterministic fake is the first provider implementation and remains the strict test oracle. The
stateless demo is the first runnable local provider. OpenAI is the first live provider, but it does
not become runnable until the intervening security profile is implemented. Those statements are
complementary, not competing sequences.

## Framework and caching progression

- Do not add LangChain or LangGraph to the infrastructure cores.
- Preserve the handwritten harness loop; evaluate a framework only through a later evidence-backed
  harness ADR.
- Begin with stable prompt prefixes and provider-reported cache metrics.
- Add vLLM prefix caching only with a self-hosted runtime story.
- Add cache-affinity routing only after LatencyLab can measure both benefit and queueing cost.

## Detailed-planning boundary

The detailed, linked story list now ends at the implementation-ready
[ICGT-011 safe local demo runtime](../user-stories/icgt-011-bind-local-demo-runtime.md), which remains
Planned. ICGT-012 and later rows in [the backlog](../user-stories/backlog.md) are outcome slices, not
promises that their contracts are ready. Before promoting one, create its story and lesson, lock
dependencies and exclusions, and name the exact human review checkpoint.
