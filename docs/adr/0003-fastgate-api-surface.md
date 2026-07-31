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

1. Which harness events and tool semantics must survive the boundary?
2. Can cancellation and terminal cleanup be expressed unambiguously?
3. How are unsupported capabilities rejected before paid work?
4. Can provider/model routing remain gateway-owned without leaking vendor fields?
5. What compatibility claim can contract tests honestly prove?
6. Can the first unit remain small enough for personal code review?
7. How will protocol evolution be versioned?

## Provisional constraint

Until this ADR is accepted, ICGT-004 may select the toolchain/module boundary and ICGT-005 may build
service lifecycle behavior. ICGT-006 owns this decision. ICGT-007 provider-domain work, ICGT-008
fake work, public inference endpoints, and OpenAI/Groq SDKs must wait for acceptance or replacement.
