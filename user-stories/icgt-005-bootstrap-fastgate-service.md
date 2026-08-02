# ICGT-005 - Bootstrap the FastGate service lifecycle

- **Status:** Planned
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-004
- **Lesson:** [Go service lifecycle](../docs/lessons/icgt-005-go-service-lifecycle.md)
- **Review priority:** High

## User story

> As a learner, I want the smallest FastGate Go process to start, report health, and stop cleanly so
> that later provider work has an understandable lifecycle owner.

## Scope

- Materialize the exact root `go.mod` and module path selected by ADR 0004; add no `go.work`, nested
  module, `toolchain` directive, dependency, or `go.sum`.
- Create one FastGate executable and the minimum package layout needed by this unit.
- Expose a bounded health endpoint with no provider or tenant behavior.
- Use explicit server timeouts and context/signal-driven shutdown.
- Add deterministic lifecycle and handler tests.
- Make `./scripts/check` enforce the selected manifest and run Go formatting, vet, ordinary tests,
  and race tests without network or toolchain downloads.

## Locked behavior

- Startup performs no network call to a provider.
- The materialized root-module layout matches ADR 0004; this story does not add an unreviewed
  module, workspace, toolchain directive, or dependency boundary.
- Health means the local process is serving; it does not claim provider readiness.
- Shutdown stops admission, drains within a bound, and reports a safe failure if cleanup exceeds it.
- No background goroutine lacks an owner and stopping rule.

## Acceptance criteria

1. The root `go.mod` contains only ADR 0004's exact module and `go 1.26.5` directives; no additional
   directive, workspace, nested module, dependency, or `go.sum` exists.
2. One documented command starts the service locally.
3. Health success and method/path failure behavior are tested.
4. Startup configuration is validated before serving.
5. Signal/context cancellation triggers bounded graceful shutdown.
6. Tests use injected listeners or contexts rather than timing-sensitive sleeps.
7. The default gate verifies the exact manifest and source inventory, fails if the local toolchain
   or race prerequisites are incompatible, and uses the complete offline environment selected by
   ADR 0004. SHA-pinned CI provisioning reads its version from `go.mod` and verifies the selected
   `GOVERSION` instead of repeating a second version literal.
8. The lesson traces scaffold ownership, service construction, one request, shutdown, and a failure
   path.

## Human review checkpoint

- **Production path:** Selected root-module scaffold through service construction, listener
  admission, and health response.
- **Failure/test path:** Bounded shutdown when active work or server close does not finish promptly.
- **Invariant:** The checked Go layout is the reviewed ICGT-004 layout, and the owner that starts a
  server is responsible for stopping and joining it.
- **Deferred:** Public inference API, provider port, streaming, metrics, Docker, and Kubernetes.

## Entry condition

ICGT-004 has accepted the toolchain/module ADR. The selected Go toolchain and race prerequisites
must also be available locally after explicit user approval. Do not create `go.mod` until that
environment condition is true, and do not create `go.work` in this story. Once the preflight passes,
this story owns creating the selected root scaffold and teaching the gate how to validate it.

## Validation

- Focused Go lifecycle and handler tests.
- Mandatory race-enabled test stage after the entry-condition preflight.
- `./scripts/check`.

## Documentation impact

Update root/FastGate setup commands, architecture implementation status, and the lesson. Amend the
module ADR only if implementation evidence disproves an accepted assumption.

## Out of scope

- Provider contracts or adapters.
- `/v1/chat/completions`, `/v1/responses`, or another inference endpoint.
- Docker, TLS, authentication, metrics, or deployment.
