# ICGT-008 lesson: A basic deterministic fake upstream

- **Unit:** ICGT-008
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; the provider port has a strict basic deterministic fake
- **Story:** [ICGT-008](../../user-stories/icgt-008-build-basic-fake-upstream.md)
- **Review priority:** High
- **Visual companion:** Not required; this Markdown lesson is the reviewed learning artifact
- **Related architecture:** [ADR 0002](../adr/0002-fake-first-openai-first-live.md) and
  [FastGate agent guidelines](../../gateway/AGENTS.md)

## Quick summary

ICGT-008 adds a small in-memory implementation of FastGate's provider port. A test gives the fake an
ordered script of exact requests and controlled outcomes. Each invocation must match the next request,
and teardown must prove that no expected exchange remains.

The fake is intentionally strict. A request mismatch is broken test setup, not a simulated provider
failure. It panics with a bounded field path, remembers that violation, and never copies prompt or
instruction content into the diagnostic.

## Learning objectives

After this unit, you should be able to:

- distinguish a strict fake from a canned stub and a live provider adapter;
- trace one exact request to one scripted result or normalized failure;
- explain why fake-interaction mistakes do not use the provider error channel;
- preserve absent usage separately from observed-zero usage; and
- use complete-script verification to catch calls that never occurred.

## The mental model

Think of the fake as a short checklist with a cursor:

```text
script: [exchange 1, exchange 2, exchange 3]
                  ^
                next

Invoke(request)
  -> compare only with the next exchange
  -> return that exchange's result or failure
  -> move the cursor exactly once

VerifyComplete()
  -> pass only when the cursor reached the end and no violation was recorded
```

It does not search ahead for a request that happens to match. Searching ahead would hide an ordering
bug in the code under test.

## Stub, fake, and live adapter

| Kind | What it proves | What it does not prove |
| --- | --- | --- |
| Canned stub | The caller can receive one fixed value | Exact input, order, or complete interaction |
| ICGT-008 strict fake | Exact ordered calls and controlled provider-neutral outcomes | HTTP, vendor translation, timing, streaming, or cancellation |
| Live adapter | Translation and transport against a real provider | Fully repeatable rare failures without cost or network uncertainty |

The fake implements the real [`provider.Invoker`](../../gateway/internal/provider/provider.go), but it
does not imitate an OpenAI or Groq HTTP API. Vendor wire and SDK types remain outside the
provider-neutral package.

## The small public scripting surface

The implementation lives in
[`gateway/internal/provider/fake`](../../gateway/internal/provider/fake/fake.go) and exposes only two
main abstractions:

- `Exchange` is one immutable expected request and exactly one terminal outcome. Tests construct it
  with `ExpectResult` or `ExpectFailure`.
- `Provider` owns the copied script, the next-exchange cursor, and any sticky interaction violation.
  Tests construct it with `New`, call the real `Invoke` method, and finish with `VerifyComplete`.

`New` accepts at most `MaxExchanges` (64). It checks that bound before cloning the caller's slice.
An empty script is valid because it is a useful assertion that provider dispatch must happen zero
times.

## Happy path: match, consume, return

The important successful control flow is in
[`Provider.Invoke`](../../gateway/internal/provider/fake/fake.go):

```go
exchange := fake.exchanges[fake.next]
if path := firstRequestMismatch(exchange.expected, actual); path != "" {
	fake.fail(fmt.Sprintf(
		"fake provider request %d differed at %s",
		exchangeNumber,
		path,
	))
}

fake.next++
if exchange.outcome == outcomeResult {
	return exchange.result, nil
}
return provider.Result{}, exchange.failure
```

Read it in four small steps:

1. `fake.exchanges[fake.next]` selects only the next expected exchange.
2. The short variable declaration `path := ...` computes the first mismatch location. The condition
   runs only when that path is nonempty.
3. `fake.next++` consumes the exchange only after an exact match. A mismatch cannot advance state.
4. The final branch preserves the ICGT-007 terminal alternatives: either a result plus `nil`, or a
   zero result plus the same direct `*provider.Failure` that the test scripted.

The ordered two-result test also mutates the caller's original script after `New`. Both expected
results still arrive in order, proving the fake owns a defensive copy. See
[`TestProviderReturnsOrderedResultsAndOwnsScript`](../../gateway/internal/provider/fake/fake_test.go).

## Exact matching without secret-bearing diagnostics

`firstRequestMismatch` compares every current `provider.Request` field explicitly:

| Request part | Exact checks |
| --- | --- |
| Conversation | Length, then each message role and content in order |
| Instructions | Length, then each source and content in order |
| Required capabilities | Length, then each capability in order |

The matcher returns only the first static path, such as `conversation[0].content`. It never formats a
whole request or prints expected and actual values. This keeps the diagnostic bounded and prevents
prompts, instructions, or other secret-bearing content from appearing in test logs.

Nil and empty optional instruction/capability slices both represent zero elements, so they match.
Strings are compared exactly as Go strings. The fake does not normalize Unicode: composed `café` and
an otherwise similar decomposed spelling are different inputs.

[`TestProviderReportsEveryRequestMismatchWithoutContent`](../../gateway/internal/provider/fake/fake_test.go)
uses distinct sentinel secrets and checks the immediate panic, a repeated call, and
`VerifyComplete`. The field-inventory test uses reflection to fail if a future `provider.Request`
field is added without updating the matcher.

## Failure path: an assertion violation stays sticky

A request mismatch, extra invocation, or nil context means the test and the code under test disagree.
It is not a real upstream observation. Returning it as an ordinary error would violate
`provider.Invoker`, whose error channel allows only a direct normalized `*provider.Failure` or an
exact caller-context termination sentinel.

The fake therefore uses an out-of-band test assertion:

```go
func (fake *Provider) fail(diagnostic string) {
	if fake.violation == nil {
		fake.violation = errors.New(diagnostic)
	}
	panic(fake.violation)
}
```

The first violation is stored before the panic. That detail matters because Go code can recover from
a panic. Without sticky state, a test could recover, make later calls, and incorrectly pass final
verification. With sticky state, every later `Invoke` repeats the same safe panic and
`VerifyComplete` returns the same violation.

This panic should be read as “the test program is wrong,” not “the provider failed.” The fake never
disguises an interaction mismatch as `internal_error` or any other billable/provider outcome.

## Missing work: verify the whole script

An output assertion alone cannot prove every expected call happened. `VerifyComplete` separately
checks the interaction:

```go
if fake.violation != nil {
	return fake.violation
}
if fake.next < len(fake.exchanges) {
	remaining := len(fake.exchanges) - fake.next
	return fmt.Errorf(
		"fake provider expected request %d, but %d exchange(s) were never invoked",
		fake.next+1,
		remaining,
	)
}
return nil
```

This catches both “nothing was invoked” and “only a prefix was invoked.” In this synchronous unit,
there is no separate in-flight state: a matching exchange is consumed atomically before its outcome
returns.

## Usage and normalized failures

`ExpectFailure` accepts only a validated direct `*provider.Failure`. `Invoke` returns that exact
pointer unchanged. The tests exercise every ICGT-007 provider failure code and preserve:

- the failure category;
- the provider's retryability observation;
- usage absent;
- usage present with both counts equal to zero; and
- usage present with nonzero or maximum bounded counts.

Retryability remains evidence, not permission for the fake or caller to retry. No retry behavior is
introduced in this story.

## Context, ownership, and deliberate omissions

`Invoke` requires a nonnil `context.Context` because the real port requires it. The basic fake does
not inspect `ctx.Err()`, deadlines, or context values. An already-canceled nonnil context therefore
receives the scripted outcome. This is deliberate: cancellation checkpoints belong to the later
streaming/concurrency fake, where they can be defined against an event lifecycle.

`Provider` has one owner and is not safe for concurrent `Invoke` or `VerifyComplete` calls. There is
no mutex, goroutine, channel, timer, clock, filesystem, process, environment, or network dependency.
Adding synchronization now would imply concurrency semantics that this story cannot yet test.

## Test evidence

The focused suite is
[`gateway/internal/provider/fake/fake_test.go`](../../gateway/internal/provider/fake/fake_test.go).

| Evidence | Important test |
| --- | --- |
| Real-port conformance | Compile-time `provider.Invoker` assertion |
| Ordered success and owned script | `TestProviderReturnsOrderedResultsAndOwnsScript` |
| Wrong order fails immediately | `TestProviderRejectsRequestsOutOfOrder` |
| Success usage presence | `TestProviderPreservesCompletedUsage` |
| Every normalized failure | `TestProviderPreservesEveryFailureOutcome` |
| Every request field and safe diagnostics | `TestProviderReportsEveryRequestMismatchWithoutContent` |
| Nil versus empty and exact Unicode | Matching representation tests |
| Extra, zero-dispatch, and omitted calls | Extra-invocation and completion-verification tests |
| Invalid and exact/one-over bounds | `TestExchangeAndScriptConstructionRejectInvalidValues` |
| Deferred cancellation/deadline behavior | `TestProviderContextBoundaryDoesNotAddCancellationBehavior` |
| Future request-field drift | `TestRequestFieldInventoryLocksExactMatcher` |

Focused tests, 50 repeated runs, `go vet`, the race detector, and the complete offline
`./scripts/check` gate passed. No credential or network access participates.

## Personal code review map

Review these paths in order:

1. Read [`Exchange`, `New`, and `Provider.Invoke`](../../gateway/internal/provider/fake/fake.go) as the
   main production path.
2. Trace `firstRequestMismatch`, `fail`, and `VerifyComplete` in the same file as the meaningful
   failure path.
3. Read `TestProviderReturnsOrderedResultsAndOwnsScript`,
   `TestProviderReportsEveryRequestMismatchWithoutContent`, and
   `TestVerifyCompleteReportsUninvokedExchanges` in
   [`fake_test.go`](../../gateway/internal/provider/fake/fake_test.go).
4. Confirm the invariant: a passing fake-backed test preserves the exact scripted outcome and proves
   the complete ordered interaction without exposing request content.
5. Confirm what is absent: HTTP, provider SDKs, live inference, retries, streaming, timing,
   cancellation checkpoints, and concurrent scripting.

## What changed during implementation and review

- The implementation selected a child `provider/fake` package, keeping scripting state separate from
  the provider-domain values.
- Separate result and failure constructors make “both outcomes” and “no outcome” invalid states.
- The script bound is 64 and is checked before copying; the exact boundary and one-over rejection are
  both tested.
- Empty scripts were retained because later admission tests need strong zero-dispatch evidence.
- Fake-interaction mistakes use a sticky safe panic rather than violating the production provider
  error contract.
- Adversarial review added explicit swapped-entry order, recovered-panic, nested request-field
  inventory, all-string-path privacy, full exact-boundary consumption, and clock-free deadline
  evidence.
- Independent correctness review found no remaining production or test defect. The reusable sticky
  test-double questions were added to the PR review checklist.

The production implementation is 181 nonblank, non-comment lines, below the repository's normal
review heuristic. Tests are intentionally larger because each contract dimension has independent
evidence.

## Production expansion

A production provider adapter will add configuration, credentials, HTTP/SDK translation, response
validation, and safe error normalization. Later fake stories may add an event script, logical gates,
malformed upstream output, cancellation checkpoints, and deterministic concurrency. Those features
need explicit lifecycle ownership and should not be hidden inside this basic synchronous cursor.

## Practical exercises

- Add a two-exchange script on paper and predict `next` after each exact call.
- Change one expected instruction source and identify the only diagnostic path that may appear.
- Explain why an empty script plus `VerifyComplete` is insufficient until the code path that might
  dispatch also runs.
- Compare absent usage with present zero usage and explain why both states matter.

## Key takeaways

- A strict fake verifies inputs and interaction order, not only returned values.
- Fake assertion failures stay outside the production provider-outcome channel.
- Sticky violations prevent recovery from turning a broken interaction into a passing test.
- Content-safe field paths help debugging without leaking prompts or instructions.
- Complete-script verification catches missing calls after the main behavior finishes.

## Glossary

- **Exchange:** one exact expected request and one scripted terminal outcome.
- **Script cursor:** the index of the next expected exchange.
- **Sticky violation:** the first interaction error retained for every later assertion.
- **Exhaustion verification:** proof that the script has no unconsumed exchanges.
- **Out-of-band assertion:** a test-programming failure that is not represented as a production
  provider result or failure.

## Teach-back questions

1. Why does a mismatch panic instead of returning `FailureInternal` from `Invoke`?
2. How do exact next-exchange matching and `VerifyComplete` prove different parts of the interaction?
3. Why does the basic fake return its scripted outcome even when a nonnil context is already canceled?

## Further reading

- [ICGT-008 delivery contract](../../user-stories/icgt-008-build-basic-fake-upstream.md)
- [ICGT-008 implementation note](../../user-stories/notes/2026-08-03-icgt-008-basic-deterministic-fake.md)
- [ICGT-007 provider contract](../../gateway/internal/provider/provider.go)
- [ADR 0002: fake-first, OpenAI-first-live](../adr/0002-fake-first-openai-first-live.md)
- [PR review regression checklist](../pr-review-checklist.md)
