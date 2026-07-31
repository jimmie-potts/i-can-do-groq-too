# ICGT-008 - Build the basic deterministic fake upstream

- **Status:** Planned
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-007
- **Lesson:** [Basic deterministic fake](../docs/lessons/icgt-008-basic-deterministic-fake.md)
- **Review priority:** High

## User story

> As a gateway developer, I want a strict non-streaming fake upstream so that exact requests,
> results, failures, and complete expected interaction can be proven without credentials, cost, or
> network access.

## Scope

- Implement the ICGT-007 provider port in memory.
- Script ordered exact request expectations and one result or enumerated failure per exchange.
- Detect request mismatch, extra request, missing request, unconsumed output, and unfinished
  operation without copying prompt contents into diagnostics.

## Locked behavior

- Mismatch messages identify bounded field paths or ordinals, never request content.
- Fake completion is explicit; test teardown can assert the entire script was consumed.
- The fake performs no filesystem, process, environment, or network I/O.

## Acceptance criteria

1. Exact matching covers every ICGT-007 request field.
2. Success, enumerated failure, mismatch, omitted/extra request, unconsumed result, and unfinished
   script scenarios are deterministic.
3. Diagnostic tests use sentinel prompt/secret text and prove it is absent.
4. No filesystem, environment, process, clock, or network dependency participates.
5. The lesson traces exact matching, result/failure selection, safe diagnostics, and exhaustion
   verification.

## Human review checkpoint

- **Production path:** Exact request match through one scripted successful completion.
- **Failure/test path:** Safe mismatch diagnostics and teardown detection of unconsumed work.
- **Invariant:** A passing fake-backed test proves both expected output and complete expected
  interaction.
- **Deferred:** HTTP mapping, SSE, wall-clock timeouts, live adapters, retries, and routing.

## Validation

- Focused fake contract and exhaustion tests.
- Sentinel-content diagnostic assertions.
- Repeated deterministic runs and `./scripts/check`.

## Documentation impact

Update the FastGate provider package map and lesson with exact production and test excerpts.

## Out of scope

- Emulating an entire OpenAI or Groq API.
- HTTP server integration.
- Streaming events, logical gates, cancellation, malformed streams, concurrency races, real latency,
  network fault injection, or provider credentials.
