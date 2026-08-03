# ICGT-008 - Build the basic deterministic fake upstream

- **Status:** Done
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
- Script ordered exact request expectations and one result or enumerated failure per exchange,
  including failed outcomes with and without bounded usage.
- Detect request mismatch, extra request, missing request, and incomplete expected-exchange
  consumption without copying prompt contents into diagnostics.

## Locked behavior

- Mismatch messages identify bounded field paths or ordinals, never request content.
- Fake completion is explicit; test teardown can assert the entire script was consumed.
- The fake performs no filesystem, process, environment, or network I/O.
- `Exchange` values are built through separate result and failure constructors. `Provider` owns a
  copied ordered script with at most 64 exchanges; an empty script explicitly expects zero calls.
- Matching checks only the next exchange. It treats nil and empty optional lists alike, preserves
  list order, and compares Unicode exactly without normalization.
- A mismatch, extra invocation, or nil context is a test-programming error outside the provider
  outcome contract. It records one content-safe diagnostic and panics; the violation remains visible
  through `VerifyComplete` even if the panic is recovered.
- The basic fake has one owner and is not concurrency-safe. It does not inspect cancellation or
  deadlines, so an already-canceled nonnil context still receives the scripted outcome.

## Acceptance criteria

1. Exact matching covers every ICGT-007 request field.
2. Success, enumerated failure with absent or present usage, mismatch, omitted/extra request, and
   unfinished script scenarios are deterministic.
3. Diagnostic tests use sentinel prompt/secret text and prove it is absent.
4. No filesystem, environment, process, clock, or network dependency participates.
5. The lesson traces exact matching, result/failure selection, safe diagnostics, and exhaustion
   verification.

## Human review checkpoint

- **Production path:** Exact request match through one scripted successful completion.
- **Failure/test path:** Preserve failed outcomes with absent versus observed-zero usage, then trace
  safe mismatch diagnostics and teardown detection of unconsumed work.
- **Invariant:** A passing fake-backed test preserves the exact scripted outcome—including usage
  absence versus zero—and proves complete expected interaction.
- **Deferred:** HTTP mapping, SSE, wall-clock timeouts, live adapters, retries, and routing.

## Validation

- `GOCACHE=/tmp/icgt-go-build go test ./gateway/internal/provider/fake`
- `GOCACHE=/tmp/icgt-go-build go test -count=50 ./gateway/internal/provider/fake`
- `./scripts/check`

## Documentation impact

The FastGate provider package map, repository status documents, implementation note, review
checklist, and lesson contain exact production and test evidence.

## Out of scope

- Emulating an entire OpenAI or Groq API.
- HTTP server integration.
- Streaming events, logical gates, cancellation, malformed streams, concurrency races, real latency,
  network fault injection, or provider credentials.
