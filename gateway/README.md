# FastGate

**Status:** Lifecycle, model-turn v1 contract tooling, the internal provider contract, its basic
deterministic fake, and bounded model-turn admission/execution are implemented. FastGate currently
serves only operational health; no inference endpoint or live provider adapter exists yet.

FastGate is the first implementation target: a small inference gateway that gives clients one
reviewed model-turn contract while keeping provider transport, streaming, routing, rate limits, and
operational telemetry outside application workflows.

## MVP direction

The initial implementation sequence is intentionally smaller than “build a gateway”:

1. select the Go toolchain and module boundary—completed by
   [ADR 0004](../docs/adr/0004-go-toolchain-and-module-strategy.md);
2. materialize that exact root-module scaffold, then bootstrap a service lifecycle and health
   endpoint after the local toolchain preflight—completed by ICGT-005;
3. materialize the FastGate-owned protocol selected in
   [ADR 0003](../docs/adr/0003-fastgate-api-surface.md) as a versioned non-streaming schema and
   fixtures—completed by ICGT-006;
4. define FastGate-owned provider request, result, and failure values for one non-streaming
   invocation—completed by ICGT-007;
5. implement a basic non-streaming deterministic fake upstream—completed by ICGT-008;
6. implement bounded body/semantic admission and one injected provider-neutral execution, tested
   with the deterministic fake and without HTTP binding—completed by ICGT-009;
7. add an injectable HTTP handler that rejects invalid transport requests before dispatch and maps
   every completed or failed provider outcome—planned by ICGT-010;
8. mount that handler only after loopback-listener, runtime-provider, and bounded-concurrency policy
   is reviewed—future ICGT-011;
9. define the FastGate-owned model-turn SSE grammar, then extend the fake with deterministic stream
   gates and cancellation;
10. stream fake output and propagate client cancellation;
11. add deadlines, slow-client bounds, and low-cardinality metrics;
12. add OpenAI as the first opt-in live upstream; and
13. add Groq only after direct and gateway baselines can be compared.

Steps 1 through 6 are complete. The
[ICGT-009 delivery contract](../user-stories/icgt-009-admit-and-execute-model-turn.md),
[implementation](internal/modelturn), and
[verified lesson](../docs/lessons/icgt-009-bounded-model-turn-admission.md) provide bounded admission,
zero/one-dispatch, and provider-return evidence. Step 7 now has a reviewed planned
[ICGT-010 delivery contract](../user-stories/icgt-010-present-model-turn-over-http.md) and
[Markdown lesson](../docs/lessons/icgt-010-model-turn-http-presentation.md), but no handler code exists
yet. Step 8 remains an outcome slice; the executable stays health-only through ICGT-010.

The provider-neutral seam lives in [`internal/provider`](internal/provider). It carries only ordered
conversation, generic instructions, required capabilities, completed text, optional usage, and
normalized provider failures. Wire framing, logical aliases, credentials, endpoints, provider
model IDs, retries, routing, and concrete SDK types remain outside it.

The strict test implementation lives in [`internal/provider/fake`](internal/provider/fake). It owns
one bounded ordered script, matches every provider request field exactly, and returns a scripted
result or direct normalized failure. It performs no transport work and deliberately defers
streaming, cancellation behavior, logical gates, and concurrency to their later stories.

The admission boundary lives in [`internal/modelturn`](internal/modelturn). It consumes but does not
close a bounded `io.Reader`, enforces the strict v1 JSON and request profile, trusts correlation only
after the whole document is safe, rejects `tool_calls` before the fixed `learning-text` alias check,
and invokes the injected provider port at most once. It has no HTTP, live-adapter, retry, logging,
timer, goroutine, queue, or network behavior.

## Run the lifecycle foundation

From the repository root:

```bash
go run ./gateway/cmd/fastgate
```

The process listens on `127.0.0.1:8080`. In another terminal:

```bash
curl --fail --show-error http://127.0.0.1:8080/healthz
```

The bounded response is `ok`. Press Ctrl+C in the service terminal to trigger graceful shutdown.
Use `-listen 127.0.0.1:8081` to select another loopback address. This route proves only that the
local HTTP process is serving; it does not claim that a model provider or tenant system is ready.

## Responsibilities

FastGate owns:

- its versioned client wire schema, language-neutral fixtures, and server-side conformance;
- client transport and validated request admission;
- provider adapter translation;
- streamed response framing and backpressure;
- upstream cancellation and cleanup;
- narrowly scoped provider/network retry policy;
- provider capability-aware selection;
- RPM, TPM, active-stream, and queue protection;
- bounded operational errors and telemetry; and
- provider-reported usage and cache observations.

FastGate does not own:

- agent turns, tool execution, approvals, or coding-task state;
- correctness evaluation;
- tenant identity or budget authority;
- Kubernetes desired state;
- live global fleet scheduling; or
- hosted-provider raw KV tensors.

## Code Assist Harness integration

CAH-023 defines the harness's strict direct OpenAI adapter. After FastGate publishes a stable contract
and conformance bundle, a later Code Assist Harness story can implement a separate `FastGateProvider`
for its existing provider port. This repository does not own that harness client code.

The first handoff is loopback-only, unauthenticated, and no-tool. A versioned FastGate tool extension
plus a compatible CAH contract is required before full coding-agent parity; a separately implemented
authentication/TLS profile is required before non-loopback use.

ICGT-020's earlier infrastructure comparison uses a small repository-owned measurement client for
the same bounded direct-provider and gateway workload. It does not use Code Assist Harness or claim
coding-task correctness parity.

Do not point that direct OpenAI adapter at a FastGate base URL. CAH-023's locked endpoint, model
allowlist, Responses stream grammar, disabled retries, and safety assumptions belong to the direct
baseline. The future CAH-owned FastGate adapter must map the reviewed gateway contract explicitly.

The eventual joint handoff must pin an adapter-ready harness contract snapshot and the exact
FastGate schema/fixture source artifacts. FastGate defines its server guarantees; Code Assist
Harness must separately accept and own the client configuration and mapping. The first profile
defines:

- trusted loopback endpoint selection and the explicit local HTTP boundary;
- no caller authentication, no redirects, no ambient proxy/environment routing, and no explicit
  proxy unless a later profile reviews it;
- logical model aliases and capability requirements;
- instruction and conversation mapping;
- stream grammar and terminal cleanup;
- local stream/resource closure separately from confirmed or unconfirmed upstream cleanup;
- usage and bounded route metadata;
- error normalization;
- cancellation and disconnect behavior;
- request, attempt, and trace correlation; and
- no automatic retry or fallback after the first semantic output is committed.

A later non-loopback profile may add authentication, TLS, redirect, and explicit proxy policy only
after FastGate implements and conformance-tests those behaviors.

ICGT-022 packages and validates the exact handoff artifacts, records a manifest/digest, and does not
change semantics. Upstream cleanup is “confirmed” only after an explicit provider terminal
acknowledgement correlated to the attempt; a returned context or closed local connection/body is
still unconfirmed. No current v1 runtime records that certainty: ICGT-018 must define any bounded
metric, ICGT-019 gathers the first live evidence, and ICGT-021 freezes the handoff meaning.

## Learning focus

FastGate is where this repository studies:

- idiomatic Go service lifecycle;
- HTTP and SSE streaming;
- contexts, cancellation, goroutine ownership, and cleanup;
- backpressure and slow consumers;
- failure normalization and retry safety;
- time to first token and tail latency;
- rate limiting and fairness; and
- provider compatibility without false equivalence.

Read [the scoped guidance](AGENTS.md) before implementing a FastGate story.
