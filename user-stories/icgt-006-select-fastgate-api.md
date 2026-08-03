# ICGT-006 - Specify and version FastGate's model-turn v1 contract

- **Status:** Done
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-005
- **Lesson:** [Defining FastGate model-turn v1](../docs/lessons/icgt-006-selecting-client-protocol.md)
- **Review priority:** High

## User story

> As the platform and harness owner, I want the selected FastGate-owned model-turn protocol specified
> and validated before provider-domain code so that northbound client needs drive the internal seam
> rather than being forced into a premature provider model.

## Scope

- Preserve the ADR's concise comparison with the rejected Chat Completions and Responses subsets
  without designing or validating either compatibility facade.
- Use the current harness provider events and CAH-023 direct OpenAI contract as one client
  requirement without making the harness the only client.
- Produce a field-by-field mapping from the current harness request, operation, and event contract
  to the selected FastGate contract, with rejected-option gaps retained as decision evidence.
- Define the smallest client-neutral non-streaming v1 request/result/error contract and versioning
  rule, including generic ordered instruction blocks without importing CAH-specific field names.
- Describe how later streaming, tool calls, capabilities, cancellation, and terminal cleanup extend
  or constrain the choice.
- Define a canonical, language-neutral FastGate non-streaming schema and valid/invalid fixture
  corpus that later conformance tests in this repository and Code Assist Harness can pin by version.
- Define one offline schema/fixture validation command and add it to `./scripts/check`.
- Define the exact compatibility claim contract tests may make.
- Materialize the direction accepted in [ADR 0003](../docs/adr/0003-fastgate-api-surface.md), or
  propose a superseding ADR if required semantics prove impossible within a bounded project-owned
  protocol.

## Acceptance criteria

1. One concise, non-normative comparison explains why Option C remains the v1 choice and why the
   contract does not claim compatibility with either rejected option. Only the selected protocol
   receives a schema and fixtures.
2. The client contract is FastGate-owned and provider-neutral. It does not claim Chat Completions or
   Responses compatibility or expose either as a v1 alias.
3. A versioned client contract defines required/optional fields, bounds, authentication placeholder,
   model alias, normalized errors, request correlation, and unknown-field behavior.
4. The mapping covers ordered conversation roles/content; CAH repository guidance translated into
   generic ordered instruction-block source/content; text observations; provider-emitted tool-call
   requests; optional usage;
   normalized failure code, bounded control-safe message, and retryability; completed/failed
   terminal behavior; lazy operation start; single-consumer event claim; cancellation and its
   `cancelled`/`already_closed` outcomes; no-later-event behavior after terminal/local cancellation;
   repeatable local cleanup confirmation; idempotent forced local task reaping; and confirmed versus
   unconfirmed upstream cleanup.
5. Every mapped semantic is marked exact, lossy, explicitly unsupported, or deferred to a named
   versioned extension and owning story. Required semantic loss blocks ICGT-006 completion until the
   contract preserves it or a reviewed ADR amendment or supersession changes the decision.
6. The schema, mapping, and fixtures distinguish a client-declared unsupported capability that later
   runtime must reject before provider work from unsolicited unsupported upstream output that later
   runtime must map to a bounded post-dispatch failure. This unit does not claim to enforce either
   behavior at runtime.
7. FastGate owns the canonical versioned non-streaming schema and valid/invalid fixtures. Streaming,
   cancellation, and cleanup fixtures are added only by the later stories that implement those
   behaviors. Later FastGate tests validate its implementation against the applicable corpus; a
   future harness-owned adapter pins the same contract version and runs applicable fixtures in the
   harness repository.
8. A committed offline, credential-free contract test accepts every valid fixture, rejects every
   invalid fixture, validates schema syntax, rejects any validation-affecting drift from the frozen
   canonical v1 schemas, and runs from `./scripts/check`.
9. The mapping distinguishes FastGate adapter configuration—trusted endpoint, authentication, and
   logical model alias—from fields in the current harness provider request.
10. The mapping assigns adapter implementation to Code Assist Harness and does not weaken or
   repurpose the CAH-023 direct OpenAI adapter.
11. No HTTP endpoint, provider port, SDK, live call, or harness adapter is implemented by the spike;
    the fixture validator is contract tooling, not service behavior.

## Human review checkpoint

- **Production path:** Trace the [minimal request fixture](../gateway/contracts/model-turn/v1/fixtures/valid/minimal-request.json)
  through the [request schema](../gateway/contracts/model-turn/v1/schema/request.schema.json) and
  [offline validator](../scripts/check_contract.py). There is no service request path in this unit.
- **Failure/test path:** Trace the [unknown-field fixture](../gateway/contracts/model-turn/v1/fixtures/invalid/unknown-request-field.json)
  through its manifest expectation, then personally review the nested-schema fingerprint mutations,
  bounded-read probe, and artifact-guard-versus-`json` regressions in
  [the checker suite](../tests/test_check_contract.py).
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

Add the first client-contract schema, harness field mapping, language-neutral fixtures, and exact
compatibility statement under a non-empty contract path introduced by this story. Amend or supersede
ADR 0003 only if the evidence invalidates its accepted constraints.

## Out of scope

- Implementing the endpoint or provider port.
- Implementing or changing the Code Assist Harness adapter or provider port.
- Implementing Chat Completions or Responses compatibility facades.
- Provider routing, retries, quotas, streaming code, or live-provider integration.
