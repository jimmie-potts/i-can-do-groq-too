# FastGate agent guidelines

These instructions apply to work under `gateway/` and extend the repository root `AGENTS.md`.

## Scope

FastGate is an inference data plane, not an agent framework. It translates a reviewed client
contract into one selected upstream operation and returns bounded normalized output. Keep task
orchestration, tools, approvals, tenant authority, and deployment reconciliation outside this
component.

ADR 0003 selects a small FastGate-owned model-turn protocol. No public inference endpoint or
provider-domain contract may be added until ICGT-006 publishes its reviewed non-streaming v1 schema,
mapping, fixtures, and offline validation. ADR 0004 selects Go 1.26.5 and one future root module;
ICGT-005 materializes it and creates lifecycle behavior before that contract work.

## Reviewable package boundaries

Introduce packages only with their owning story. Prefer boundaries that make these responsibilities
independently testable:

- service lifecycle and shutdown;
- FastGate-owned domain contracts;
- upstream provider port;
- deterministic fake upstream;
- external transport adapter;
- concrete provider adapters; and
- metrics or policy projections.

Do not create a generic “utils” package or prebuild abstractions for all five projects.

## Concurrency and streaming

- The inbound request context owns cancellation of all request-scoped work.
- Every goroutine, channel, timer, stream, and response body has a visible owner and stopping rule.
- Do not start unbounded goroutines or use unbounded channels.
- Do not retry after semantic output is committed to the client.
- Test cancellation before upstream output, between outputs, while downstream writing is blocked,
  and against natural completion when the owning story reaches those behaviors.
- Keep terminal selection and cleanup semantics explicit; a successful upstream payload is not a
  completed FastGate request until required cleanup and framing invariants hold.

## Provider boundaries

- Provider SDK values stay inside concrete adapters.
- The first non-streaming provider port accepts the caller context and returns one bounded outcome.
  Do not predesign a lazy asynchronous operation or cleanup barrier before the streaming lifecycle
  story can test it.
- Once a later streaming story introduces `start` or equivalent construction, it is lazy and
  performs no network I/O until the operation is consumed.
- Enforce provider-SDK dependency boundaries in the first concrete-adapter story, where a real SDK
  import makes the negative test meaningful.
- The basic fake verifies exact requests, scripted result/failure, safe mismatch diagnostics, and
  complete script consumption. Add logical stream gates, malformed output, and cancellation
  checkpoints only in the later story that owns the stream grammar and concurrency state.
- Capability absence is explicit. Do not send and hope, discard unknown fields, or infer equivalence
  from similar endpoint names.
- Keep provider SDK retries disabled until a FastGate story owns a bounded retry policy.

## Observability and privacy

Use bounded opaque IDs and low-cardinality labels. Never make prompts, repository paths,
credentials, raw headers, raw provider errors, model output, tenant IDs with unbounded cardinality,
or response bodies into metric labels or default logs.

## FastGate story completion

In addition to root requirements, a completed asynchronous FastGate story proves the resource
cleanup and cancellation behavior in its own scope with deterministic tests. Its lesson must trace
the request owner from admission to terminal cleanup and explain where duplicate cost or output
could occur.
