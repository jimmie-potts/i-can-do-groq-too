# ModelEndpoint Operator

**Status:** Planned. No CRD, controller, manifest, or cluster dependency exists yet.

ModelEndpoint Operator is a Go Kubernetes controller for reconciling inference-facing services and
configuration. It teaches control-loop behavior rather than merely generating YAML.

## Planned MVP

- Define a versioned `ModelEndpoint` custom resource.
- Reconcile Deployments, Services, ConfigMaps, and Secret references.
- Report `Ready`, `Progressing`, `Degraded`, and configuration status conditions.
- Handle create, update, delete, retry, and finalizer paths idempotently.
- Use least-privilege RBAC and deterministic controller tests.
- Run an opt-in local integration path in `kind` or `k3d` after unit behavior is stable.

## Boundaries

The operator may deploy FastGate, LatencyLab runners, monitoring, and later self-hosted inference
servers. It does not deploy Code Assist Harness workers in the MVP. Turning the local WSL,
keyboard-driven harness into a remote worker would require a separate harness ADR for workspace
transport, authentication, approvals, isolation, secrets, and lifecycle ownership.

The operator reports endpoint identity, health, declared capabilities, configuration revision, and
rollout status. FastGate owns the conformance-tested semantic capability truth used for routing.
Tenant-visible model-alias policy belongs to TenantPlane and is enforced by FastGate.
