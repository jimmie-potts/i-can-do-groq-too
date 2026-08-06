# ICGT-008 basic deterministic fake implementation note

## Outcome

ICGT-008 introduces `gateway/internal/provider/fake`, a strict in-memory implementation of the
ICGT-007 synchronous provider port. Tests script ordered exact requests and one validated result or
direct normalized failure per exchange, then explicitly verify complete consumption. It introduces
no endpoint, provider SDK, live inference, retry, stream, timer, goroutine, network, filesystem,
process, environment, or visual lesson.

## Accepted design decisions

- `Exchange` has private immutable state and separate `ExpectResult` and `ExpectFailure`
  constructors, so a script step cannot contain both outcomes or neither outcome.
- `Provider` owns a defensively copied ordered script, one cursor, and the first interaction
  violation. It is a single-owner test value and is not concurrency-safe.
- `MaxExchanges` is 64. `New` rejects one-over before cloning and accepts an empty script so a caller
  can prove zero provider dispatch.
- Matching examines only the next exchange and every current request field. It treats nil and empty
  optional lists as the same zero-element value, preserves list order, and performs exact Go string
  comparison without Unicode normalization.
- A mismatch, extra invocation, or nil context is a test-programming failure outside
  `provider.Invoker`'s production outcome alternatives. The fake records one content-safe diagnostic
  and panics. `VerifyComplete` retains the same violation after recovery.
- A matched failure crosses the port as the same direct `*provider.Failure`, preserving code,
  retryability, and usage absence versus observed zero.
- The basic fake does not inspect cancellation or deadlines. An already-canceled nonnil context gets
  its scripted result or failure; cancellation checkpoints remain deferred.

## Important control flow

`New` checks the script cardinality, copies the accepted slice, and validates every exchange. `Invoke`
rejects a prior sticky violation, nil context, or exhausted script; compares the next expected request;
advances only after an exact match; and returns exactly one scripted terminal alternative.
`VerifyComplete` reports a sticky violation first, then the next expected ordinal and remaining count,
or succeeds when the script is exhausted.

The matcher returns only the first fixed path: conversation length/role/content, instruction
length/source/content, or required-capability length/value. It never formats request values.

## Review-driven changes

Pre-implementation comparison with Code Assist Harness retained the ordered strict-script discipline
but rejected CAH's asynchronous operations, emitted events, delays, gates, and cancellation machinery
because the current FastGate port is synchronous and non-streaming.

Independent implementation review found the production code clean. Adversarial test review found
that several initial tests demonstrated examples without fully locking their invariants:

- the first one-over test used invalid zero-value exchanges, so exchange validation could hide a
  missing cardinality check;
- the first mismatch cases changed only element zero, and only the top-level `Request` field
  inventory was locked;
- extra and missing-interaction diagnostics did not use secret sentinels; and
- substring assertions did not prove the first-path-only diagnostic rule.

The final tests fill exact and one-over scripts with valid exchanges, consume all 64 retained
exchanges, pin the literal bound, swap and mutate second message and instruction elements, lock
`Request`, `Message`, and `Instruction` inventories, use a multi-difference request with one exact
diagnostic, and check all request string fields stay absent across mismatch, repeated, extra,
zero-dispatch, nil-context, and unconsumed-script paths. They also exercise nil/empty equivalence in
both directions, every string-bearing Unicode path, canceled contexts for both results and failures,
and a clock-free context reporting a deadline. These reusable missed-case classes are recorded in
[`docs/pr-review-checklist.md`](../../docs/pr-review-checklist.md).

The current capability constructor admits only `tool_calls` and at most one value, so a capability
value mismatch cannot be built through the public API. Length matching covers every reachable state;
the field-inventory comment requires a value case when that vocabulary expands.

## Evidence

- Production fake: [`gateway/internal/provider/fake/fake.go`](../../gateway/internal/provider/fake/fake.go)
- Focused tests: [`gateway/internal/provider/fake/fake_test.go`](../../gateway/internal/provider/fake/fake_test.go)
- Learning companion: [`docs/lessons/icgt-008-basic-deterministic-fake.md`](../../docs/lessons/icgt-008-basic-deterministic-fake.md)
- Review regression checklist: [`docs/pr-review-checklist.md`](../../docs/pr-review-checklist.md)

The production file is 181 nonblank, non-comment lines. It adds two main exported abstractions and
stays below the repository's normal review heuristic.

Validation:

```text
GOCACHE=/tmp/icgt-go-build go test ./gateway/internal/provider/fake
GOCACHE=/tmp/icgt-go-build go test -count=50 ./gateway/internal/provider/fake
GOCACHE=/tmp/icgt-go-build go vet ./gateway/internal/provider/fake
GOCACHE=/tmp/icgt-go-build go test -race ./gateway/internal/provider/fake
./scripts/check
```

All commands completed successfully after the adversarial review fixes. The temporary Go build cache
is an environment workaround only and is not repository configuration.

## Human review checkpoint

Personally trace `ExpectResult` or `ExpectFailure` through `New`, `Provider.Invoke`,
`firstRequestMismatch`, and `VerifyComplete`. Then read
`TestProviderReturnsOrderedResultsAndOwnsScript`,
`TestProviderReportsEveryRequestMismatchWithoutContent`, and
`TestVerifyCompleteReportsUninvokedExchanges`.

The reviewer should be able to explain this invariant: a passing fake-backed test preserves the exact
scripted outcome and proves the complete ordered interaction without exposing request content.

## ICGT-009 handoff

ICGT-009 may use an empty script to prove zero dispatch for invalid admission and a one-exchange
script to prove exactly one admitted provider-neutral turn. It must not expand the fake with HTTP,
streaming, gates, cancellation checkpoints, live-provider behavior, retries, or routing. Before work
begins, ICGT-009 needs a reviewed implementation-ready story and Markdown lesson. That handoff was a
future requirement when this ICGT-008 note was completed; it is now satisfied by the delivered
[ICGT-009 story](../icgt-009-admit-and-execute-model-turn.md) and
[lesson](../../docs/lessons/icgt-009-bounded-model-turn-admission.md), without changing ICGT-008's
historical implementation evidence.
