# ICGT-005 - Bootstrap the FastGate service lifecycle

- **Status:** Done
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
   or race prerequisites are incompatible, and gives child stages an explicit environment that
   omits ambient credential variables, uses a gate-owned `HOME`, and disables isolated Go telemetry.
   It uses the complete offline environment selected by ADR 0004. SHA-pinned CI provisioning reads
   its version from `go.mod` and verifies the selected `GOVERSION` instead of repeating a second
   version literal.
8. The lesson traces scaffold ownership, service construction, one request, shutdown, and a failure
   path.

## Human review checkpoint

- **Production path:** [`go.mod`](../go.mod) through
  [`main.run`](../gateway/cmd/fastgate/main.go),
  [`service.New`](../gateway/internal/service/service.go), listener admission, and the bounded health
  response.
- **Failure/test path:**
  [`TestServiceServeFailureRunsBoundedCleanup`](../gateway/internal/service/service_test.go) proves
  that an unexpected server failure still triggers cleanup and preserves its cause;
  [`TestServiceShutdownFailureForcesCloseAndJoins`](../gateway/internal/service/service_test.go)
  proves the deadline, forced close, final join, and safe timeout error without sleeping.
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

Validated with Go 1.26.5 on Linux/amd64 using the complete offline environment in ADR 0004. Focused
tests, vet, ordinary tests, the race preflight, race tests, repository-policy tests, Markdown links,
and Git whitespace checks pass. The rendered and inspected visual companion is linked from the
[lesson](../docs/lessons/icgt-005-go-service-lifecycle.md).

## Documentation impact

Update root/FastGate setup commands, architecture implementation status, and the lesson. Amend the
module ADR only if implementation evidence disproves an accepted assumption.

## Out of scope

- Provider contracts or adapters.
- `/v1/chat/completions`, `/v1/responses`, or another inference endpoint.
- Docker, TLS, authentication, metrics, or deployment.
