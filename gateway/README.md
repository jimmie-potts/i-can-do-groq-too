# FastGate

**Status:** Lifecycle and model-turn v1 contract tooling implemented. FastGate currently serves only
operational health; no inference endpoint, provider contract, or provider adapter exists yet.

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
   invocation;
5. implement a basic non-streaming deterministic fake upstream;
6. expose one non-streaming endpoint and contract tests;
7. add structured transport failure mapping;
8. define the FastGate-owned model-turn SSE grammar, then extend the fake with deterministic stream
   gates and cancellation;
9. stream fake output and propagate client cancellation;
10. add deadlines, slow-client bounds, and low-cardinality metrics;
11. add OpenAI as the first opt-in live upstream; and
12. add Groq only after direct and gateway baselines can be compared.

The reviewed planned sequence currently stops at steps 1 through 5. Steps 1 through 3 are complete;
step 4 is next in dependency order. Promote one step at a time as its upstream evidence and start
conditions are accepted. Later steps are roadmap outcomes until their dependencies produce evidence.

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

ICGT-019's earlier infrastructure comparison uses a small repository-owned measurement client for
the same bounded direct-provider and gateway workload. It does not use Code Assist Harness or claim
coding-task correctness parity.

Do not point that direct OpenAI adapter at a FastGate base URL. CAH-023's locked endpoint, model
allowlist, Responses stream grammar, disabled retries, and safety assumptions belong to the direct
baseline. The future CAH-owned FastGate adapter must map the reviewed gateway contract explicitly.

The eventual joint handoff must pin an adapter-ready harness contract snapshot and the exact
FastGate schema/fixture source artifacts. FastGate defines its server guarantees; Code Assist
Harness must separately accept and own the client configuration and mapping. Together they define:

- trusted endpoint selection and authentication source, scope, rotation, and redaction;
- HTTPS verification, any loopback-only HTTP exception, redirect policy, and explicit versus
  ambient proxy/environment trust;
- logical model aliases and capability requirements;
- instruction and conversation mapping;
- stream grammar and terminal cleanup;
- local stream/resource closure separately from confirmed or unconfirmed upstream cleanup;
- usage and bounded route metadata;
- error normalization;
- cancellation and disconnect behavior;
- request, attempt, and trace correlation; and
- no automatic retry or fallback after the first semantic output is committed.

ICGT-021 packages and validates the exact handoff artifacts, records a manifest/digest, and does not
change semantics. Upstream cleanup is “confirmed” only after an explicit provider terminal
acknowledgement correlated to the attempt; a returned context or closed local connection/body is
still unconfirmed. FastGate records that certainty as bounded operational telemetry in v1.

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
