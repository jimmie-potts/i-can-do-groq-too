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
- Produce a field-by-field mapping from the current harness request, operation, and event contract
  to each candidate and the selected FastGate contract.
- Define the smallest non-streaming v1 request/result/error contract and versioning rule.
- Describe how later streaming, tool calls, capabilities, cancellation, and terminal cleanup extend
  or constrain the choice.
- Define a canonical, language-neutral FastGate non-streaming schema and valid/invalid fixture
  corpus that later conformance tests in this repository and Code Assist Harness can pin by version.
- Define one offline schema/fixture validation command and add it to `./scripts/check`.
- Define the exact compatibility claim contract tests may make.
- Accept, replace, or split [ADR 0003](../docs/adr/0003-fastgate-api-surface.md).

## Acceptance criteria

1. The comparison includes concrete request/result and future stream examples for all three options.
2. The accepted ADR names one initial protocol and explains rejected alternatives.
3. A versioned client contract defines required/optional fields, bounds, authentication placeholder,
   model alias, normalized errors, request correlation, and unknown-field behavior.
4. The mapping covers ordered conversation roles/content; ordered repository-instruction
   source/content; text observations; provider-emitted tool-call requests; optional usage;
   normalized failure code, bounded control-safe message, and retryability; completed/failed
   terminal behavior; cancellation; no-later-event behavior after terminal/local cancellation;
   repeatable local cleanup confirmation; and confirmed versus unconfirmed upstream cleanup.
5. Every mapped semantic is marked exact, lossy, explicitly unsupported, or deferred to a named
   versioned extension and owning story. Required semantic loss blocks ADR acceptance until the
   contract preserves it.
6. Unsupported client-declared capabilities are rejected explicitly before provider work. An
   unsolicited unsupported upstream output becomes a bounded failure after dispatch and is never
   silently discarded or mislabeled as a preflight rejection.
7. FastGate owns the canonical versioned non-streaming schema and valid/invalid fixtures. Streaming,
   cancellation, and cleanup fixtures are added only by the later stories that implement those
   behaviors. Later FastGate tests validate its implementation against the applicable corpus; a
   future harness-owned adapter pins the same contract version and runs applicable fixtures in the
   harness repository.
8. A committed offline, credential-free contract test accepts every valid fixture, rejects every
   invalid fixture, validates schema syntax, and runs from `./scripts/check`.
9. The mapping distinguishes FastGate adapter configuration—trusted endpoint, authentication, and
   logical model alias—from fields in the current harness provider request.
10. The mapping assigns adapter implementation to Code Assist Harness and does not weaken or
   repurpose planned CAH-023.
11. No HTTP endpoint, provider port, SDK, live call, or harness adapter is implemented by the spike;
    the fixture validator is contract tooling, not service behavior.

## Human review checkpoint

- **Production path:** Trace the offline contract validator from the selected schema through every
  accepted valid fixture; there is no service request path in this unit.
- **Failure/test path:** Trace one invalid fixture rejected for its intended rule and one required
  semantic that a candidate cannot represent, plus one unsupported capability, unknown field, and
  disconnect, to bounded explicit decisions.
- **Invariant:** FastGate's internal provider seam is downstream of a reviewed client contract.
- **Deferred:** Go provider types, fake upstream, HTTP handler, SSE implementation, and adapters.

## Validation

- Reconcile the mapping against the current harness request, operation, event, cancellation, and
  cleanup contract without reusing CAH-023.
- Run the offline contract test and prove that it accepts every valid fixture, rejects every invalid
  fixture, and fails on invalid schema syntax.
- Review compatibility claims against primary provider API documentation current at implementation.
- Run `./scripts/check`.

## Documentation impact

Accept or replace ADR 0003 and add the first client-contract schema, harness field mapping, and
language-neutral fixtures under a non-empty contract path introduced by this story.

## Out of scope

- Implementing the endpoint or provider port.
- Implementing or changing the Code Assist Harness adapter or provider port.
- Supporting both APIs merely to defer the decision.
- Provider routing, retries, quotas, streaming code, or live-provider integration.
