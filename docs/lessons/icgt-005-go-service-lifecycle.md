# ICGT-005 lesson: Go service lifecycle

- **Unit:** ICGT-005
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; no Go module or service exists
- **Story:** [ICGT-005](../../user-stories/icgt-005-bootstrap-fastgate-service.md)
- **Review priority:** High
- **Visual companion:** Planned after implementation
- **Related architecture:** [ADR 0004](../adr/0004-go-toolchain-and-module-strategy.md),
  [Local setup](../setup.md), [FastGate README](../../gateway/README.md), and
  [FastGate agent guidelines](../../gateway/AGENTS.md)

> All behavior below is planned. No package names or examples are shipped code.

## Quick summary

This unit will materialize ADR 0004's reviewed root Go module and create the smallest FastGate
process with an honest health response and bounded shutdown. It teaches that build and process
ownership come before provider and streaming complexity.

## Learning objectives

You should be able to explain how a reviewed module decision becomes checked files, how the gate
discovers every module, HTTP server ownership, context cancellation, signal handling, graceful
shutdown, and deterministic lifecycle tests.

## Why this unit matters

Every later request, stream, timer, and goroutine lives inside the process lifecycle. If startup and
shutdown ownership are vague, provider cancellation and resource cleanup will also be vague.

## Junior engineer foundation

A process receives operating-system signals such as interrupt or termination. A context carries a
cancellation signal through ordinary function calls. Graceful shutdown stops new requests and gives
active work a bounded opportunity to finish.

A common misconception is that returning from `main` automatically cleans up every goroutine and
network operation safely. The process exits, but in-flight work may be cut off without flushing or
recording a terminal state.

## Key concepts

- **Composition root:** the place that constructs concrete dependencies and starts the application.
- **Readiness/health:** a bounded observation with a precisely documented meaning.
- **Grace period:** maximum time allowed for cooperative cleanup.
- **Owner:** the code responsible for starting, stopping, and joining a resource.

## Architecture and invariants

Planned flow:

```text
accepted scaffold manifest -> materialize root module -> offline gate enforces exact layout
validated configuration -> construct server -> serve health
signal/context cancel -> stop admission -> bounded shutdown -> return outcome
```

The health endpoint reports local serving state only. It must not claim that OpenAI, Groq, a
database, or TenantPlane is available.

## Practical walkthrough

After the approved local-toolchain and race preflight succeeds, the implementation first creates
exactly ADR 0004's root `go.mod` and updates the offline gate to reject workspaces, nested modules,
manifest drift, or missing source checks. It then creates one executable, one testable server
construction seam, one health handler, and one bounded shutdown path. Provider packages and
inference endpoints remain absent.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Planned root `go.mod` and gate enforcement | Turns ADR 0004 into an enforced manifest | Can a nested module, workspace, or unreviewed dependency escape the gate? |
| Planned service composition root | Owns startup and shutdown | Who joins every started resource? |
| Planned lifecycle tests | Prove health and bounded cleanup | How is time controlled without sleeps? |

## Implementation code samples

None. Replace this section with exact Go excerpts after implementation.

## Failure scenarios to study

- Port binding fails before serving.
- A nested module or workspace exists but the gate tests only the root package pattern.
- The module requests an unavailable toolchain and validation tries to download it.
- Configuration is invalid.
- A shutdown request races a health request.
- Graceful shutdown exceeds its bound.

Each failure needs a bounded safe outcome and deterministic evidence.

## What changed during implementation

No implementation evidence exists yet.

## Production expansion

Production services may add separate liveness/readiness, TLS, connection draining, orchestration
hooks, structured logging, and SLOs. Add them when deployment or incident evidence requires them,
not to make the initial service look complete.

## Practical exercises

- Draw the owner of listener, server, signal context, and shutdown timer.
- Predict what health should return before the server accepts connections.
- Compare injected listener tests with tests that reserve a fixed port.

## Key takeaways

- Lifecycle ownership precedes provider behavior.
- The first code story must implement the exact root-module handoff from the decision story.
- Health claims only what the local process can prove.
- Every started resource needs a bounded stopping and joining rule.

## Glossary

- **Context:** a Go cancellation/deadline propagation value.
- **Graceful shutdown:** bounded stop-admission and drain behavior.
- **Composition root:** dependency construction and application start location.

## Teach-back questions

1. How should the gate prove that the root module is exact and no nested module or workspace exists?
2. Which code should own the HTTP server, shutdown timer, and resource join—and why?
3. When would separate readiness and liveness endpoints become justified?

## Further reading

- [ICGT-005 delivery contract](../../user-stories/icgt-005-bootstrap-fastgate-service.md)
- [Go `net/http` package](https://pkg.go.dev/net/http)
- [Go `context` package](https://pkg.go.dev/context)
