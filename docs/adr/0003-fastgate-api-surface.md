# ADR 0003: Use a FastGate-owned model-turn protocol

- **Status:** Accepted
- **Date:** 2026-07-31
- **Accepted:** 2026-08-01
- **Scope:** Initial client protocol direction and ownership

## Context

The initial project proposal names `POST /v1/chat/completions`. The Code Assist Harness roadmap uses
a deliberately strict subset of the OpenAI Responses API. Those APIs have different request,
streaming, tool, state, and error semantics. Calling both “OpenAI compatible” would hide meaningful
differences.

The CAH-023 direct OpenAI contract fixes the official endpoint, rejects ambient base-URL routing,
validates an exact event automaton, disables SDK retries, and intentionally excludes multi-provider
routing. That adapter must not be repurposed as the FastGate adapter.

FastGate owns its public wire contract, schema, and conformance fixtures. Code Assist Harness owns
the client adapter that translates that contract into its provider port. ICGT-006 can prove
representability and define non-streaming fixtures; it cannot claim that later streaming,
cancellation, or cleanup behavior has passed runtime conformance before those behaviors exist.

## Options

### A. Chat Completions-compatible subset

Advantages:

- familiar and widely implemented by inference providers;
- smaller initial request and SSE surface; and
- useful for non-agent clients.

Costs:

- requires a distinct harness translation path;
- may not preserve Responses concepts the harness later needs; and
- risks compatibility claims that exceed the tested subset.

### B. Responses-compatible subset

Advantages:

- closer to the harness's first live provider flow;
- clearer path for typed streamed items and tool calls; and
- can preserve newer request semantics.

Costs:

- provider support differs substantially;
- larger event automaton and translation burden; and
- may bias the gateway architecture toward one provider's API.

### C. Small FastGate-owned model-turn protocol

Advantages:

- names only the stable behavior this platform owns;
- makes provider capabilities and normalized failures explicit; and
- compatibility facades can remain separate adapters.

Costs:

- every client needs an adapter;
- less immediate ecosystem compatibility; and
- requires careful versioning from the first endpoint.

## Decision

Select option C: FastGate will expose a small, versioned, FastGate-owned model-turn protocol as its
first client contract.

The protocol will name only behavior FastGate owns and can validate across providers. It will not
claim Chat Completions or Responses compatibility, and the first endpoint will not expose both
formats as aliases. A future compatibility facade requires its own client need, contract fixtures,
and reviewed story or ADR.

ICGT-006 materializes the exact v1 fields, bounds, normalized-error meanings, strict JSON parse
profile, unknown-field policy, base/extension versioning mechanics, mapping, fixtures, and offline
artifact validation. Bounded transport/body admission and provider-neutral execution belong to
ICGT-009; endpoint path, binding, and complete provider-outcome mapping belong to ICGT-010. The accepted
direction and committed schema still do not claim that an endpoint, streaming implementation,
cancellation behavior, or runtime conformance exists.

## Rationale

- A project-owned protocol keeps FastGate's public promise provider-neutral instead of making one
  provider's current wire format the platform domain model.
- Explicit capabilities and normalized failures make unsupported behavior reviewable rather than
  silently flattening differences between providers.
- Code Assist Harness remains one client rather than becoming the shape of the gateway. Its future
  `FastGateProvider` can map the reviewed protocol into the harness provider port without weakening
  or repurposing its direct OpenAI adapter.
- A small contract is appropriate for a learning project: its request, result, failure, and later
  stream lifecycle can be inspected and tested directly.
- Compatibility facades remain possible later, but their adoption and maintenance cost will be
  visible rather than hidden inside the first endpoint.

## Rejected alternatives

Option A is not the first contract because immediate SDK familiarity does not compensate for a
weaker fit with the harness's typed events and lifecycle semantics. If a real client later requires
Chat Completions compatibility, FastGate can evaluate a separate facade against explicit fixtures.

Option B is not the first contract because its richer event model still couples FastGate's public
surface to one provider's evolving API and is unevenly supported by other providers. The harness's
direct Responses adapter remains the direct-provider baseline; it is not the FastGate contract.

## Implementation requirements

ICGT-006 must turn this accepted direction into a reviewable non-streaming v1 contract before any
public inference endpoint or provider-domain contract depends on it:

1. Map the selected protocol, field by field, to the current harness's ordered conversation,
   repository guidance translated into generic ordered instruction blocks, text and provider-emitted
   tool events, optional non-authoritative usage, normalized failures, lazy operation start,
   single-consumer event claim, exactly-one terminal behavior, cancellation with its
   `cancelled`/`already_closed` outcomes, ordinary cleanup, and forced local cleanup contract.
2. Classify each semantic as exact, lossy, explicitly unsupported, or deferred. Any required loss
   blocks ICGT-006 completion unless the schema preserves it, a named versioned extension owns it,
   or a later ADR changes the direction.
3. Represent cancellation, no-later-event behavior, local resource closure, idempotent forced local
   reaping, and confirmed versus unconfirmed upstream cleanup without claiming they are implemented
   or that local closure proves remote termination.
4. Assign client-declared unsupported-capability rejection to the pre-dispatch admission story before
   paid work, with deterministic no-dispatch evidence, while unsolicited unsupported upstream output
   becomes an honest bounded post-dispatch failure in the endpoint story.
5. Keep provider/model routing gateway-owned without leaking vendor fields into the harness request
   contract.
6. State the exact FastGate protocol compatibility claim that schema validation and contract tests
   can honestly prove; do not broaden it into an OpenAI compatibility claim.
7. Publish the canonical non-streaming schema and language-neutral valid/invalid fixtures FastGate
   owns.
8. Define how later streaming fixtures and protocol evolution will be versioned.
9. Define how both repositories will pin the same contract release without sharing internal source
   code.
10. Keep the first unit small enough for personal review.

The mapping must remain honest when one client is narrower than the provider-neutral base contract.
In particular, it records current CAH terminal-text restrictions as a lossy client mapping rather
than narrowing FastGate for every client. It also separates no-text failure usage, which current CAH
cannot represent, from text-first failure usage that the later stream profile may preserve.

## Evidence staging

- ICGT-006 owns the compatibility/gap matrix, selected non-streaming v1 schema, and valid/invalid
  language-neutral fixtures plus their offline gate validation.
- Later SSE, cancellation, and cleanup stories extend the fixture corpus only after they can prove
  those behaviors.
- ICGT-020 pins an adapter-ready harness contract snapshot and turns implemented FastGate behavior,
  transport policy, and exact schema/fixture source artifacts into a candidate cross-repository
  handoff profile.
- ICGT-021 packages and validates those frozen artifacts, records a manifest/digest, and cannot
  change semantics without a new contract version.
- A later Code Assist Harness story owns `FastGateProvider`, accepts the client side of the profile,
  pins that manifest/digest, and runs harness-side conformance tests before the handoff is jointly
  accepted.

## Consequences and sequencing

ICGT-004 may select the toolchain/module boundary and ICGT-005 may build service lifecycle behavior.
ICGT-006 owns materializing this decision as the versioned non-streaming schema, mapping, fixtures,
and offline validator. ICGT-007 provider-domain work, ICGT-008 fake work, public inference endpoints,
and OpenAI/Groq SDKs must wait for that evidence.

Every client needs an adapter for the FastGate protocol. Code Assist Harness owns its adapter; this
repository owns the wire schema, fixtures, server conformance, and provider translation. Existing
OpenAI-compatible clients cannot treat FastGate as a base-URL replacement unless a later facade is
implemented and tested.

This decision does not set model output length or provider inference speed. The custom JSON and
stream framing may add or remove small amounts of translation and transport overhead; ICGT-019 will
measure gateway overhead rather than inferring it from the protocol name.
