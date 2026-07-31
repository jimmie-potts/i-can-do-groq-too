# ADR 0001: Keep the inference platform separate from Code Assist Harness

- **Status:** Accepted
- **Date:** 2026-07-31
- **Scope:** Repository and component ownership

## Context

The planned FastGate, LatencyLab, ModelEndpoint Operator, TenantPlane, and FleetSim projects can
support `code-assist-harness`, but they teach different systems concerns. Combining them into the
harness would couple coding-agent safety and workflow changes to networking, Kubernetes, tenancy,
and scheduling experiments.

The harness already defines an explicit-loop architecture and implements its provider-neutral port,
deterministic fake, cancellation semantics, and transcript precursor. The provider-backed turn and
hard limits remain planned. Those boundaries are designed to accept another provider adapter without
importing a gateway or provider SDK into its domain.

## Decision

Maintain this repository as a separate learning monorepo. Code Assist Harness is an external client.

- The harness owns task state, model-turn orchestration, tools, approvals, audit, and correctness.
- FastGate owns inference transport, provider adaptation, capability-aware routing, and operational
  request policy.
- LatencyLab owns performance and fault experiments.
- TenantPlane owns platform identity, permission, quota, budget, ledger, and audit authority.
- ModelEndpoint Operator owns deployment reconciliation.
- FleetSim owns offline scheduling experiments.

Integration occurs through an explicit FastGate provider adapter in the harness after both sides
have stable contracts. The direct OpenAI adapter remains a baseline and is not repointed to a custom
base URL.

## Consequences

- Each repository can progress and validate independently.
- Cross-repository behavior requires a versioned handoff contract and parity tests rather than
  source imports.
- Similar concepts such as cancellation, limits, usage, and errors must have named ownership to
  avoid duplicate or conflicting behavior.
- Some setup and lesson machinery is intentionally repeated because it serves separate learning
  histories.

## Rejected alternatives

### Add the five projects inside Code Assist Harness

Rejected because it makes the harness responsible for unrelated platform infrastructure and turns
an application learning path into a distributed monolith.

### Put model routing directly in the harness

Rejected because it spreads provider/network policy through the coding workflow and makes other
clients unable to reuse the gateway.
