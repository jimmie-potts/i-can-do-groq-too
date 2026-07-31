# ADR 0003: Select FastGate's first API surface

- **Status:** Proposed
- **Date:** 2026-07-31
- **Scope:** Client request and streaming contract

## Context

The initial project proposal names `POST /v1/chat/completions`. The Code Assist Harness roadmap uses
a deliberately strict subset of the OpenAI Responses API. Those APIs have different request,
streaming, tool, state, and error semantics. Calling both “OpenAI compatible” would hide meaningful
differences.

The locked CAH-023 contract for the harness's planned direct OpenAI adapter fixes the official
endpoint, rejects ambient base-URL routing, validates an exact event automaton, disables SDK
retries, and intentionally excludes multi-provider routing. That future adapter must not be
repurposed as the FastGate adapter.

FastGate owns its public wire contract, schema, and conformance fixtures. Code Assist Harness owns
the client adapter that translates that contract into its provider port. This ADR can prove
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

## Decision criteria

ICGT-006 must answer the following through a focused spike and review before this ADR is accepted:

1. How does each candidate map, field by field, to the current harness's ordered conversation,
   ordered repository instructions, text and provider-emitted tool events, optional non-authoritative
   usage, normalized failures, exactly-one terminal behavior, cancellation, and cleanup contract?
2. Is each semantic exact, lossy, explicitly unsupported, or deferred, and does any required loss
   block acceptance or require a named versioned extension?
3. Can cancellation, no-later-event behavior, local resource closure, and confirmed versus
   unconfirmed upstream cleanup be represented without claiming they are implemented?
4. How are future client-declared unsupported capabilities rejected before paid work, while an
   unsolicited unsupported upstream output becomes an honest bounded post-dispatch failure?
5. Can provider/model routing remain gateway-owned without leaking vendor fields into the harness
   request contract?
6. What exact compatibility claim can schema validation and contract tests honestly prove?
7. What canonical non-streaming schema and language-neutral valid/invalid fixtures does FastGate
   own?
8. How will later streaming fixtures and protocol evolution be versioned?
9. How will both repositories pin the same contract release without sharing internal source code?
10. Can the first unit remain small enough for personal review?

## Evidence staging

- ICGT-006 owns the compatibility/gap matrix, selected non-streaming v1 schema, and valid/invalid
  language-neutral fixtures plus their offline gate validation.
- Later SSE, cancellation, and cleanup stories extend the fixture corpus only after they can prove
  those behaviors.
- ICGT-020 pins an adapter-ready harness contract snapshot and turns implemented FastGate behavior,
  transport policy, and exact schema/fixture source artifacts into the jointly reviewed
  cross-repository handoff.
- ICGT-021 packages and validates those frozen artifacts, records a manifest/digest, and cannot
  change semantics without a new contract version.
- A later Code Assist Harness story owns `FastGateProvider`, accepts the client side of the profile,
  pins that manifest/digest, and runs harness-side conformance tests.

## Provisional constraint

Until this ADR is accepted, ICGT-004 may select the toolchain/module boundary and ICGT-005 may build
service lifecycle behavior. ICGT-006 owns this decision. ICGT-007 provider-domain work, ICGT-008
fake work, public inference endpoints, and OpenAI/Groq SDKs must wait for acceptance or replacement.
