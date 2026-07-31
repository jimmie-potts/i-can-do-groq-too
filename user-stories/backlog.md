# Outcome backlog

This file preserves the long-range learning ladder without pretending every outcome is ready for
implementation. The detailed dependency-ready sequence lives in [README.md](README.md).

## M0 - Foundation

- Architecture and cross-repository ownership.
- Story, lesson, and human-review standards.
- Git hygiene and an offline repository gate.

## M1 - FastGate non-streaming walking skeleton

- Go toolchain/module decision, then service lifecycle and health.
- Reviewed external API contract before provider-domain code.
- FastGate-owned provider contracts.
- Basic deterministic fake upstream, followed later by streaming/cancellation controls.
- One non-streaming request and normalized failure path.

## M2 - FastGate streaming reliability

- SSE grammar and fake streaming.
- Client cancellation and upstream cleanup.
- Upstream deadline and cleanup grace.
- Slow-client backpressure bounds.
- TTFT, duration, active request, queue, cancellation, failure, and usage metrics.

## M3 - Live providers and harness integration

- Opt-in OpenAI adapter with automatic SDK retries disabled.
- Direct-versus-gateway parity and overhead measurements.
- Versioned harness-to-FastGate handoff contract.
- Separate harness FastGate adapter; do not reuse the direct OpenAI adapter or `OPENAI_BASE_URL`.
- Groq adapter for limited, evaluated tasks.
- Capability-aware rejection, emulation, or routing.

## M4 - LatencyLab

- Versioned workload and results schemas.
- Concurrency saturation and prompt-length experiments.
- Slow-client, retry-storm, disconnect, and tail-latency fault scenarios.
- Reproducible percentile and gateway-overhead reports.
- Optional visualization only after machine-readable results are stable.

## M5 - ModelEndpoint Operator

- Versioned CRD and status conditions.
- Idempotent Deployment, Service, ConfigMap, and Secret-reference reconciliation.
- Finalizers, RBAC, leader election, restart, and transient API failure evidence.
- FastGate endpoint identity and capability handoff.
- No remote Code Assist Harness worker in the operator MVP.

## M6 - TenantPlane

- Organization, project, role, and key model.
- Hashed keys, explicit authorization, and fast revocation policy.
- Versioned FastGate policy snapshots with expiry and outage behavior.
- Idempotent usage events, partial-stream semantics, append-only ledger, and audit.
- PostgreSQL first; Redis only after a measured enforcement need.

## M7 - FleetSim

- Deterministic scenario and result schemas with units and provenance.
- Independent first-fit, best-fit, utilization, latency, reservation, fairness, and cache-aware
  schedulers.
- Region outage, model launch, long-context saturation, borrowing, and degraded-rack scenarios.
- LatencyLab calibration without prompt or repository content.
- Offline advisory output only.

## Deferred advanced work

- self-hosted vLLM and automatic prefix caching;
- cache-affinity routing across replicas;
- distributed rate limiting after measured need;
- Kubernetes autoscaling based on streams, queue depth, or TTFT;
- raw KV-cache management as a distinct runtime project; and
- framework comparisons that preserve deterministic core ownership.
