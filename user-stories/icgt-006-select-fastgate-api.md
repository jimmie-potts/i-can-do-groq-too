# ICGT-006 - Select FastGate's first client protocol

- **Status:** Planned
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-005
- **Lesson:** [Selecting a client protocol](../docs/lessons/icgt-006-selecting-client-protocol.md)
- **Review priority:** High

## User story

> As the platform and harness owner, I want FastGate's first request, result, error, and later stream
> semantics selected before provider-domain code so that northbound client needs drive the internal
> seam rather than being forced into a premature provider model.

## Scope

- Compare a tested Chat Completions subset, Responses subset, and small FastGate-owned model-turn
  protocol.
- Use the current harness provider events and locked CAH-023 plan as one client requirement without
  making the harness the only client.
- Define the smallest non-streaming v1 request/result/error contract and versioning rule.
- Describe how later streaming, tool calls, capabilities, cancellation, and terminal cleanup extend
  or constrain the choice.
- Define the exact compatibility claim contract tests may make.
- Accept, replace, or split [ADR 0003](../docs/adr/0003-fastgate-api-surface.md).

## Acceptance criteria

1. The comparison includes concrete request/result and future stream examples for all three options.
2. The accepted ADR names one initial protocol and explains rejected alternatives.
3. A versioned client contract defines required/optional fields, bounds, authentication placeholder,
   model alias, normalized errors, request correlation, and unknown-field behavior.
4. Unsupported capabilities are rejected explicitly before provider work.
5. The contract preserves cancellation and terminal-cleanup meaning without claiming those behaviors
   are implemented.
6. The harness mapping uses a future separate FastGate adapter and does not weaken planned CAH-023.
7. No HTTP endpoint, provider port, SDK, or live call is implemented by the spike.

## Human review checkpoint

- **Production path:** None; review the accepted ADR and versioned client-contract examples.
- **Failure/test path:** Trace one unsupported capability, unknown field, and disconnect through the
  selected semantics to a bounded explicit outcome.
- **Invariant:** FastGate's internal provider seam is downstream of a reviewed client contract.
- **Deferred:** Go provider types, fake upstream, HTTP handler, SSE implementation, and adapters.

## Validation

- Map the selected contract to the existing harness event vocabulary without reusing CAH-023.
- Review compatibility claims against primary provider API documentation current at implementation.
- Run `./scripts/check`.

## Documentation impact

Accept or replace ADR 0003 and add the first client-contract document under a non-empty contract
path introduced by this story.

## Out of scope

- Implementing the endpoint or provider port.
- Supporting both APIs merely to defer the decision.
- Provider routing, retries, quotas, streaming code, or live-provider integration.
