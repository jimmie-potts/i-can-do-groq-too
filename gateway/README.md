# FastGate

**Status:** Planned. No Go module, service, endpoint, or provider adapter exists yet.

FastGate is the first implementation target: a small inference gateway that gives clients one
reviewed model-turn contract while keeping provider transport, streaming, routing, rate limits, and
operational telemetry outside application workflows.

## MVP direction

The initial implementation sequence is intentionally smaller than “build a gateway”:

1. select the Go toolchain and module boundary;
2. bootstrap a service lifecycle and health endpoint;
3. resolve the external protocol in [ADR 0003](../docs/adr/0003-fastgate-api-surface.md);
4. define FastGate-owned request, result, capability, and failure values;
5. implement a basic non-streaming deterministic fake upstream;
6. expose one non-streaming endpoint and contract tests;
7. add structured transport failure mapping;
8. define the SSE grammar, then extend the fake with deterministic stream gates and cancellation;
9. stream fake output and propagate client cancellation;
10. add deadlines, slow-client bounds, and low-cardinality metrics;
11. add OpenAI as the first opt-in live upstream; and
12. add Groq only after direct and gateway baselines can be compared.

The reviewed planned sequence currently stops at steps 1 through 5, but only step 1 is presently
dependency-ready. Promote one step at a time as its upstream evidence is accepted. Later steps are
roadmap outcomes until their dependencies produce evidence.

## Responsibilities

FastGate owns:

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

The harness roadmap first calls OpenAI directly through the planned strict CAH-023 adapter. After
FastGate has a stable contract, the harness can gain a separate `FastGateProvider` implementation
of its existing provider port.

Do not point that planned direct OpenAI adapter at a FastGate base URL. CAH-023's locked endpoint,
model allowlist, Responses stream grammar, disabled retries, and safety assumptions belong to the
direct baseline. The FastGate adapter must map the reviewed gateway contract explicitly.

The eventual handoff must define:

- version and authentication;
- logical model aliases and capability requirements;
- instruction and conversation mapping;
- stream grammar and terminal cleanup;
- usage and bounded route metadata;
- error normalization;
- cancellation and disconnect behavior;
- request, attempt, and trace correlation; and
- no automatic retry or fallback after the first semantic output is committed.

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
