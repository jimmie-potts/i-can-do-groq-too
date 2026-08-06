# Outcome backlog

This file preserves the long-range learning ladder without pretending every outcome is ready for
implementation. The detailed dependency-ready sequence lives in [README.md](README.md).

## M0 - Foundation

- Architecture and cross-repository ownership.
- Story, lesson, and human-review standards.
- Git hygiene and an offline repository gate.

## M1 - FastGate non-streaming walking skeleton

- Go toolchain/module decision, then service lifecycle and health.
- Reviewed FastGate-owned model-turn v1 schema and fixtures before provider-domain code.
- FastGate-owned provider contracts.
- Basic deterministic fake upstream, followed later by streaming/cancellation controls.
- Completed bounded pre-dispatch model-turn admission includes schema-valid `tool_calls` rejection
  with deterministic zero-dispatch evidence. The next outcome is one loopback-only fake-backed
  endpoint that exhaustively maps completed and failed provider outcomes.

## M2 - FastGate streaming reliability

- FastGate-owned model-turn SSE grammar and fake streaming.
- Client cancellation and upstream cleanup.
- Upstream deadline, cleanup grace, and bounded FastGate-local reaping that never proves remote
  termination.
- Slow-client backpressure bounds.
- TTFT, duration, active request, queue, cancellation, failure, and usage metrics.

## M3 - Live providers and harness integration

- Opt-in OpenAI adapter with automatic SDK retries disabled.
- Direct-versus-gateway wire, failure, retry, cancellation, and overhead measurements; no coding-task
  parity claim before a harness adapter exists. Use a repository-owned measurement client rather
  than depending on Code Assist Harness.
- Versioned harness-to-FastGate handoff pinned to an adapter-ready harness contract snapshot.
- FastGate-owned schema, fixtures, conformance artifacts, and client integration guidance.
- ICGT-021 packaging that preserves ICGT-020's frozen artifacts and records a manifest/digest;
  semantic changes require a new version and handoff review.
- A separate Code Assist Harness-owned FastGate adapter; do not reuse the direct OpenAI adapter or
  `OPENAI_BASE_URL`.
- An explicit loopback-only, unauthenticated, no-TLS first handoff with endpoint/proxy/redirect and
  confirmed-versus-unconfirmed cleanup policy; authentication/TLS waits for a later implemented and
  reviewed non-loopback profile.
- Groq adapter for limited, evaluated tasks.
- Capability discovery, support, emulation, and routing beyond the mandatory v1 rejection.

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
