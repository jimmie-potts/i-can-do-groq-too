# ICGT-009 bounded model-turn admission implementation note

## Outcome

ICGT-009 introduces `gateway/internal/modelturn`, the internal boundary from untrusted model-turn v1
bytes to one injected `provider.Invoker`. It consumes but does not close an `io.Reader`, obtains at
most 8 MiB plus one overflow byte, enforces one strict JSON document and the exact request schema,
trusts a request ID only after the whole document is safe, applies capability policy before the fixed
alias, and invokes the provider port exactly once only after admission succeeds.

The unit adds no HTTP endpoint, provider SDK, live provider behavior, credential, retry, timer,
goroutine, queue, log, network call, runtime schema file, `go.sum`, `go.work`, or visual lesson.

## Accepted implementation decisions

- `Executor` and `Outcome` are the only two main exported abstractions. Private helpers own bounded
  reading, strict token walking, exact request decoding, correlation, and mapping.
- `Outcome` has four closed returned alternatives: uncorrelated admission failure, correlated
  admission failure, validated provider outcome, and correlated internal failure. Its constructible
  zero value is invalid and every accessor reports `false`.
- The exact uncorrelated body is the 16-byte `invalid request\n`. Correlated bodies use fixed compact
  field order and fresh bytes on access; the admitted ASCII ID and fixed strings require no JSON
  escaping.
- The reader loop limits each offered buffer to the remaining cap-plus-one capacity, processes every
  positive count before its paired error, rejects invalid Reader counts, and turns 100 consecutive
  `(0, nil)` results into internal `io.ErrNoProgress`.
- Slice capacity growth is explicit and capped. Go's default `append` growth retained a backing array
  larger than the cap on an exact-limit request even though its logical length was bounded.
- Strict syntax uses `utf8.Valid`, a raw escaped-surrogate pairing pass, and a recursive
  `json.Decoder.Token` walk with `UseNumber`, decoded-name sets, one-value framing, and a 16-container
  limit.
- Exact shape decoding uses `json.RawMessage` maps and arrays. Cardinality passes before each
  provider-domain collection allocation. Exact fields avoid `encoding/json`'s case-insensitive
  struct matching.
- Safe correlation occurs only after the bounded complete strict document and root-object check, but
  before the remaining v1 shape. A wrong version may therefore receive a correlated generic failure;
  malformed or ambiguous input cannot.
- A nonempty schema-valid capability list always means `tool_calls`, which is rejected before alias
  selection with the canonical `unsupported_capability` envelope and zero provider calls.
- Only `learning-text` admits the injected invoker. Wire version, kind, request ID, and alias do not
  enter `provider.Request`; the admitted provider capability list is empty.
- Immediately after the only invocation, `provider.ValidateInvocation` accepts only one valid result,
  direct normalized failure, or matching context sentinel. Every invalid alternative becomes the
  fixed correlated internal outcome.

## Important control flow

`Executor.Execute` checks nil programming inputs, completes the bounded read and strict-document
pass, decodes the root object, recovers a safe ID, validates the exact v1 request, rejects capability,
rejects alias, and only then enters `executeAdmitted`.

`executeAdmitted` calls `provider.NewRequest` through the private mapping helper. Mapping failure
returns a fixed correlated internal outcome with zero dispatch. Successful mapping reaches the one
`Invoke` site, followed immediately by provider-return validation. Valid provider identity is
preserved; invalid return data is not exposed.

## Review-driven changes

The first complete model-turn package draft was 718 lines and crossed the story's 650-line mandatory
review threshold. Work stopped. Reusing already-validated provider message and instruction values
after cardinality checks, removing one-use wrappers, and simplifying failure state brought the final
package to 643 production lines without compressing the parser or hiding control flow.

Independent review initially reported a high-severity issue in `provider.ValidateInvocation` and
proposed a dynamic-comparability guard around exact context-sentinel equality. Follow-up review
corrected that finding: Go compares interface dynamic values only when their dynamic types are
identical. A slice-backed raw error differs from either fixed sentinel, while an error with the
sentinel's dynamic type is comparable. The redundant production change was removed. Provider and
executor tests retain a non-comparable raw error whose `Error` method panics as direct evidence that
the existing path rejects it without panic or formatting.

Parser red-team review found no hostile JSON defect, but demonstrated that default append growth gave
an exact-limit slice a backing capacity above the claimed retained bound. Explicit capped growth and a
capacity assertion now make both logical input length and retained backing capacity no greater than
8 MiB plus one byte.

Adversarial test review found that the first mixed-read and no-progress examples did not make ordering
and reset errors observably fail. The final tests add:

- limit-plus-one data paired with a non-EOF error, requiring overflow classification before the raw
  error;
- 99 stalls, progress, another 99 stalls, and EOF, proving the counter resets;
- public no-progress rejection with no ID, code, or dispatch;
- odd/even escaped-backslash surrogate cases; and
- decoded duplicate keys spelled as an escaped surrogate pair and the same literal scalar.

These reusable classes are recorded in `docs/pr-review-checklist.md`: interface-equality review must
analyze both actual dynamic types before proposing a guard, allocation claims must include retained
capacity, and ordering/reset tests must make the wrong implementation observably different.

## Evidence

- Production entry and dispatch: [`gateway/internal/modelturn/executor.go`](../../gateway/internal/modelturn/executor.go)
- Closed outcomes: [`gateway/internal/modelturn/outcome.go`](../../gateway/internal/modelturn/outcome.go)
- Bounded strict syntax: [`gateway/internal/modelturn/parse.go`](../../gateway/internal/modelturn/parse.go)
- V1 shape and mapping: [`gateway/internal/modelturn/request.go`](../../gateway/internal/modelturn/request.go)
- Focused tests: [`gateway/internal/modelturn`](../../gateway/internal/modelturn)
- Provider-return validator: [`gateway/internal/provider/provider.go`](../../gateway/internal/provider/provider.go)
- Learning companion: [`docs/lessons/icgt-009-bounded-model-turn-admission.md`](../../docs/lessons/icgt-009-bounded-model-turn-admission.md)

Validation:

```text
GOCACHE=/tmp/icgt-go-cache TMPDIR=/tmp go test ./gateway/internal/modelturn
GOCACHE=/tmp/icgt-go-cache TMPDIR=/tmp go test -count=20 ./gateway/internal/modelturn
GOCACHE=/tmp/icgt-go-cache TMPDIR=/tmp go vet ./gateway/internal/provider ./gateway/internal/modelturn
GOCACHE=/tmp/icgt-go-cache-race TMPDIR=/tmp go test -race ./gateway/internal/provider ./gateway/internal/modelturn
./scripts/check
```

All commands passed after the review fixes. The complete gate ran outside the restricted tool sandbox
only so the existing loopback lifecycle test could open its local socket; it remained offline and
credential-free. The temporary Go cache is a WSL workspace workaround only and is not repository
configuration.

## Human review checkpoint

Personally trace `Executor.Execute` through `readRequestBody`, `validateStrictDocument`,
`safeRequestID`, `decodeWireRequest`, the capability and alias gates, `mapProviderRequest`, the one
`Invoke`, and `provider.ValidateInvocation`. Then read the exact-limit/one-over test, canonical
`tool_calls` zero-dispatch test, one-exchange admitted test, and invalid-return table.

The reviewer should be able to explain this invariant: no rejected body can start provider work, and
every admitted body starts exactly one provider-neutral invocation whose returned alternative is
either preserved as valid or replaced by fixed safe state.

## Original combined ICGT-010 handoff

At ICGT-009 completion, the next outcome combined binding this executor to one loopback-only HTTP
route with mapping each closed outcome to status, media type, and response bytes. It still needed a
reviewed story and lesson to lock method/media behavior, request-body ownership, loopback startup,
complete provider-outcome mapping, and concurrency policy for the single-owner deterministic fake.

On 2026-08-06, readiness review split that combined handoff. ICGT-010 now owns the injectable HTTP
handler, exact transport preflight, complete outcome presentation, and serial real-loopback test
evidence. ICGT-011 owns mounting the route, actual-listener loopback enforcement, runtime-provider
selection, and bounded concurrency. This preserves the strict fake as a test oracle instead of
silently treating it as a concurrency-safe interactive provider.

Neither story may weaken strict admission, recover IDs from rejected prefixes, reinterpret provider
retryability or usage, add an ambient proxy or redirect path, or expose an unauthenticated route on a
non-loopback listener.
