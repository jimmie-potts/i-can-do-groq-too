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

- Create one FastGate executable and the minimum package layout needed by this unit.
- Expose a bounded health endpoint with no provider or tenant behavior.
- Use explicit server timeouts and context/signal-driven shutdown.
- Add deterministic lifecycle and handler tests.
- Extend `./scripts/check` with reviewed Go formatting, vet/lint if chosen, tests, and race behavior.

## Locked behavior

- Startup performs no network call to a provider.
- Health means the local process is serving; it does not claim provider readiness.
- Shutdown stops admission, drains within a bound, and reports a safe failure if cleanup exceeds it.
- No background goroutine lacks an owner and stopping rule.

## Acceptance criteria

1. One documented command starts the service locally.
2. Health success and method/path failure behavior are tested.
3. Startup configuration is validated before serving.
4. Signal/context cancellation triggers bounded graceful shutdown.
5. Tests use injected listeners or contexts rather than timing-sensitive sleeps.
6. The default gate runs all Go stages without network or credentials.
7. The lesson traces service construction, one request, shutdown, and a failure path.

## Human review checkpoint

- **Production path:** Service construction through listener admission and health response.
- **Failure/test path:** Bounded shutdown when active work or server close does not finish promptly.
- **Invariant:** The owner that starts a server is responsible for stopping and joining it.
- **Deferred:** Public inference API, provider port, streaming, metrics, Docker, and Kubernetes.

## Entry condition

ICGT-004 has accepted the toolchain/module ADR and the selected Go toolchain is available locally.
Do not create `go.mod` until both conditions are true.

## Validation

- Focused Go lifecycle and handler tests.
- Race-enabled test stage if supported by the chosen toolchain and environment.
- `./scripts/check`.

## Documentation impact

Update root/FastGate setup commands, module ADR, architecture implementation status, and lesson.

## Out of scope

- Provider contracts or adapters.
- `/v1/chat/completions`, `/v1/responses`, or another inference endpoint.
- Docker, TLS, authentication, metrics, or deployment.
