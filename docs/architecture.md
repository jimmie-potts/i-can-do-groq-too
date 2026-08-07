# Architecture

**Status:** Accepted component boundaries with an incrementally implemented target.

This document describes ownership and integration rules. It does not claim that the planned
components exist. See the [README](../README.md) and [story index](../user-stories/README.md) for
current implementation status.

## Current implementation slice

ICGT-005 implements the repository-root Go module and the first FastGate process lifecycle. ICGT-006
implements the strict, non-streaming model-turn v1 schemas, language-neutral fixture corpus,
harness-semantic mapping, and offline artifact validator. ICGT-007 implements the bounded internal
provider request, result, normalized failure, and synchronous context-aware invocation contract.
ICGT-008 implements a strict, ordered, in-memory fake for that port. ICGT-009 implements bounded
model-turn v1 admission, semantic gates, exact mapping, and one validated invocation through the
injected provider port. ICGT-010 implements and tests an injectable HTTP handler with exact transport
preflight, exhaustive non-streaming outcome presentation, response abort for matching context
termination, and serial real-loopback evidence. The process still serves only the operational
`/healthz` surface (`GET`, with implicit `HEAD`); the handler is not mounted. The approved,
implementation-ready ICGT-011 story is the final M1 unit. It
plans service binding with a stateless fixed-output demo, concrete loopback `*net.TCPListener`
enforcement, and fail-fast bounded concurrency after transport preflight: default 4, CLI range 1
through 16, and no waiting queue. Health and transport rejections bypass that gate. The profile adds
no Host allowlist or caller authentication and makes no non-loopback safety claim. A live or billable
provider cannot be mounted in any runnable profile—even alongside the demo—until browser-origin and
authentication risk has a separately reviewed policy. Every client-reachable inference route and live
provider remains unimplemented.
[ADR 0005](adr/0005-local-demo-runtime-profile.md) records the accepted local-runtime profile and its
deferred security and capacity policies.

The northbound model-turn protocol and southbound provider port deliberately are not the same type:

```text
client wire document -> bounded admission/mapping -> internal provider Request
                                                       -> strict test fake
                                                       -> planned local demo
                                                       -> future live adapter
```

The internal request retains ordered conversation, generic instructions, and required capabilities.
It excludes wire framing and correlation (`version`, `kind`, `request_id`), routing
(`model_alias`, provider model IDs), credentials, endpoints, and vendor objects. Adapters return
only a bounded result, a direct normalized provider failure, or the exact termination sentinel from
the caller context. Admission-owned `invalid_request` and `unsupported_capability` failures never
cross back from an invoked adapter.

## System context

```text
                          separate client repository
                 +--------------------------------------+
                 | Code Assist Harness                  |
                 | workflow | tools | approvals | audit |
                 +------------------+-------------------+
                                    |
                  versioned FastGate model-turn protocol
                                    |
                                    v
+---------------- inference platform monorepo ----------------+
|                                                             |
|  +-------------------+       +---------------------------+   |
|  | FastGate          |<------| LatencyLab                |   |
|  | inference data    |       | workload and fault lab    |   |
|  | plane             |       +---------------------------+   |
|  +---------+---------+                                       |
|            ^ policy projection                               |
|            |                                                 |
|  +---------+---------+       +---------------------------+   |
|  | TenantPlane       |       | ModelEndpoint Operator    |   |
|  | control plane     |       | reconciliation side-plane |   |
|  +---------^---------+       +-------------+-------------+   |
|            | usage events                  | reconciles       |
|            +--------------- FastGate <-----+ and endpoints    |
|                                                             |
|  +-------------------------------------------------------+  |
|  | FleetSim - offline scheduling and capacity simulator  |  |
|  +-------------------------------------------------------+  |
+-------------------------------------------------------------+
```

## Ownership matrix

| Concern | Owner | Boundary rule |
| --- | --- | --- |
| Coding task lifecycle and durable workflow state | Code Assist Harness | Provider state may optimize a turn but is not the source of truth. |
| Tool validation, approval, and execution | Code Assist Harness | FastGate never executes repository or shell tools. |
| Correctness and task-quality evaluation | Code Assist Harness | LatencyLab may combine these results but does not redefine correctness. |
| Model HTTP/SSE transport and provider translation | FastGate | SDK and provider wire types remain adapter-local. |
| FastGate wire schema, fixtures, and server-side conformance | FastGate | Publish versioned language-neutral artifacts; do not export internal packages as a client SDK. |
| Harness-side `FastGateProvider` implementation and configuration | Code Assist Harness | Translate the pinned FastGate contract into the existing harness port without repurposing its direct OpenAI adapter. |
| Provider selection and capability-aware routing | FastGate | Unsupported capabilities are explicit; no silent field dropping. |
| Provider/network retry and circuit policy | FastGate | Avoid hidden retries in the harness and provider SDK. |
| Client cancellation intent | Code Assist Harness | FastGate must propagate the resulting disconnect/cancel upstream. |
| Local task budgets | Code Assist Harness | A local safety limit can be stricter than platform quota. |
| RPM, TPM, connection, budget, and provider-quota policy | TenantPlane | It publishes versioned policy and remains authoritative. |
| Hot-path quota, connection, and fairness enforcement | FastGate | It enforces TenantPlane's projection and reports bounded observations. |
| Identity, authorization, budget, ledger, and audit | TenantPlane | Usage ingestion is idempotent and control-plane outage behavior is explicit. |
| Deployment desired state and reconciliation | ModelEndpoint Operator | Reconciliation is idempotent; it does not choose task intent. |
| Performance and failure experiments | LatencyLab | Experiments are reproducible and distinguish gateway overhead from provider time. |
| Global scheduling experiments | FleetSim | Simulation does not become a live router without a later reviewed boundary. |

## Harness integration

The current harness provider port already expresses the essential seam:

- one immutable provider-neutral request;
- one lazy asynchronous operation;
- a single-consumer event stream;
- explicit awaited cancellation and cleanup;
- text, tool-call, usage, completion, and normalized failure events; and
- no provider SDK values in harness state.

That port should not be redesigned for FastGate now. The CAH-023 direct OpenAI adapter remains the
direct baseline. ICGT-006 commits the model-turn v1 schema and fixture source; ICGT-021 and ICGT-022
later freeze and package the adapter-ready conformance bundle. A separate Code Assist Harness story
owns the client adapter rather than this repository writing harness code or masquerading as the
official OpenAI endpoint.

```text
Harness Provider implementations
  - FakeProvider                  implemented in the harness
  - OpenAIProvider               CAH-023 direct baseline
  - FastGateProvider             future, CAH-owned separate adapter

FastGate upstream implementations
  - Deterministic fake upstream  implemented local infrastructure proof
  - Stateless local demo         planned final M1 runnable profile
  - OpenAI upstream              first live baseline
  - Groq upstream                later measured expansion
  - Self-hosted upstream         later cache/runtime experiments
```

### Changes Code Assist Harness may own later

These are future stories, not current prerequisites:

1. **A separate FastGate provider adapter.** A future harness story implements the existing harness
   port and has its own endpoint and logical-model-alias configuration. This repository supplies
   the pinned wire schema, fixtures, and integration guidance. The adapter must not weaken the fixed
   official endpoint and exact model allowlist in the CAH-023 direct OpenAI contract.
2. **Capability negotiation.** Add only when the harness requests a feature that providers differ
   on, such as tools, structured output, provider-managed state, or cache controls. Capability
   absence must fail before paid work begins.
3. **Bounded route metadata.** A provider/model route label and trace correlation may support
   evaluation, but raw gateway/provider payloads must not enter transcripts.
4. **Provider-neutral usage extensions.** Cached-token or cost observations may be added when an
   evaluation story consumes them. Platform metrics remain in FastGate/LatencyLab.
5. **Tool-result conversation values.** The harness will need these for its own multi-turn tool loop
   regardless of FastGate; this is not a gateway-driven redesign.
6. **Platform failure distinctions.** Add access-denied or budget-exhausted categories only when a
   real FastGate handoff needs the harness to distinguish them from generic rejection or rate
   limiting.
7. **A sanitized evaluation export.** Join harness correctness outcomes with LatencyLab timing by a
   bounded identifier/schema rather than importing either repository's internal library.

No current harness change is required merely to create this repository.

### Cross-repository contract and handoff

The repositories share protocol artifacts, not internal source packages:

| Artifact or behavior | Owner | Evidence stage |
| --- | --- | --- |
| FastGate non-streaming v1 schema and valid/invalid fixtures | This repository | ICGT-006 defines and versions them under accepted ADR 0003. |
| Future stream, cancellation, and cleanup fixtures | This repository | Added by the stories that implement and test those behaviors. |
| Joint harness-to-FastGate profile | Both repositories, each for its owned side | ICGT-021 freezes FastGate guarantees and candidate client requirements against a pinned harness snapshot; a future CAH story must accept and implement the client side. |
| `FastGateProvider` mapping and harness-port compliance | Code Assist Harness | A future CAH-owned story pins the FastGate contract and runs harness-side conformance tests. |

ICGT-006 must classify every current harness request, operation, and event semantic as exact, lossy,
explicitly unsupported, or deferred. The matrix covers ordered conversation roles/content; CAH
repository guidance mapped into generic ordered instruction-block source/content; text
delta/completion; provider-emitted tool-call identity,
name, and arguments; optional, non-authoritative, non-negative usage; normalized failure code,
bounded control-safe message, and retryability; lazy operation start; single-consumer event claim;
exactly-one terminal behavior; cancellation with `cancelled` versus `already_closed` outcomes;
no-later-event behavior; repeatable local cleanup; idempotent forced local task reaping; and confirmed
versus unconfirmed upstream cleanup. The committed harness contract bounds both provider-reported
token counts through `MAX_MODEL_USAGE_TOKENS`. ICGT-006 maps that existing bound; ICGT-021 later pins
its exact harness revision and value. Fixed per-code messages are CAH-023 adapter policy, not a
promise of the provider-neutral port; ICGT-010 separately owns FastGate's fixed client-facing
presentation messages.

FastGate's generic completed text is intentionally broader than CAH's terminal-text policy. A future
CAH adapter rejects an inadmissible response as one fixed safe `invalid_response`; it does not
sanitize, truncate, emit, or log the text. Likewise, no-text failure usage is preserved on the
FastGate wire but is currently unrepresentable by CAH. Only failure usage after observed text can be
deferred to the ICGT-012 stream grammar; ICGT-021 may publish the no-text omission only as explicitly
lossy, while exactness requires a later CAH contract change.

Required semantic loss blocks the ICGT-006 contract from proceeding unless the FastGate-owned schema
preserves it, a named versioned extension owns it, or a later ADR supersedes ADR 0003. The current
harness request has no tool declaration. ICGT-009 owns the rejection decision/envelope and proves the
fake is not invoked; ICGT-010 repeats the no-dispatch proof at the HTTP boundary. An
unsolicited provider-emitted tool event can only become a bounded post-dispatch failure. Neither case
may be silently discarded or described as the other.

Endpoint, authentication, and logical model alias are adapter configuration, not fields silently
invented in the current harness `ProviderRequest`. The joint ICGT-021 profile pins an immutable
harness contract snapshot and exact FastGate schema/fixture versions, then separates ownership:

- FastGate publishes only implemented, conformance-tested server behavior. The first candidate
  handoff is loopback-only and unauthenticated only after a reviewed Host/Origin/CORS/DNS-rebinding/
  caller-authentication decision for its live-provider runtime; it includes logical aliases,
  capability admission, normalized wire failures, bounded identifiers, cancellation observation, and
  any upstream-cleanup metric later defined by ICGT-018 and evidenced by ICGT-019. It advertises no
  authentication/TLS scheme.
- Code Assist Harness owns trusted endpoint selection, credential source/scope/rotation/redaction,
  TLS verification and any explicit loopback-only HTTP exception, redirect following, ambient versus
  explicit proxy/environment trust, request/event/failure mapping, retry behavior, and local
  stream/resource cleanup. It never reuses ambient `OPENAI_BASE_URL` for this adapter.

ICGT-021 freezes the versioned source artifacts. ICGT-022 packages and validates that exact content,
records the bundle manifest and digest, and may not change its semantics. A semantic change returns
to the handoff review under a new contract version. The future CAH-owned adapter pins the published
manifest/digest before the joint profile is called accepted in both repositories.

ICGT-021 re-audits the then-current CAH provider contract instead of treating the ICGT-006 snapshot
as permanently adapter-ready. The first candidate handoff is loopback-only, unauthenticated, and
no-tool after the local live-provider threat-model review. A later non-loopback profile requires
implemented FastGate authentication/TLS conformance; full coding-agent parity also requires a
separately reviewed FastGate tool extension and compatible
harness contract.

Local cleanup is not proof that a remote provider stopped work. “Confirmed” requires an explicit
provider terminal cancellation/termination acknowledgement correlated to the active attempt;
context return, local connection/body closure, or absence of later harness events remains
unconfirmed. No current v1 runtime records that certainty. ICGT-018 must define any bounded
operational metric, ICGT-019 owns the first live-provider evidence, and ICGT-021 freezes the handoff.
A later client-visible cleanup result requires a versioned contract extension.

## The real integration tension: API semantics

The original FastGate idea used `POST /v1/chat/completions`. The CAH-023 contract makes the harness's
direct live adapter a strict subset of OpenAI Responses. Pointing that adapter at FastGate would
conflict with its safety contract: CAH-023 fixes the official OpenAI endpoint, rejects ambient
base-URL routing, uses a strict Responses event automaton, and intentionally excludes routing and
retries.

Therefore:

- do not repoint the harness's OpenAI adapter at FastGate;
- do not claim that Chat Completions and Responses are interchangeable;
- preserve direct OpenAI as the measurable baseline; and
- materialize the FastGate-owned model-turn protocol selected in
  [ADR 0003](adr/0003-fastgate-api-surface.md) before its HTTP presentation and service-binding
  stories.

A later CAH-owned FastGate adapter can translate the reviewed FastGate model-turn protocol into the
existing harness event model using the pinned schema and fixtures published here. Chat Completions or
Responses compatibility, if later justified by another client, remains a separate facade rather than
the v1 contract.

## Provider capabilities

FastGate eventually needs an explicit capability registry. The exact schema is deferred, but the
behavior is not:

```text
request requires capability
          |
          v
provider supports it? -- yes --> send and validate
          |
          no
          v
explicitly emulate, reroute, or reject
```

Silently ignoring a request field is forbidden. Similar APIs do not imply identical semantics,
tool reliability, statefulness, caching, rate limits, errors, or model behavior.

FastGate owns the conformance-tested semantic capability truth used for request admission and
routing. The operator may report an endpoint's declared capabilities, deployment identity, health,
and revision, but those declarations become routable only after FastGate's adapter/conformance
contract accepts them.

## Retry and billing boundary

Retries can duplicate cost and side effects. The layers must make ownership visible:

- provider SDK automatic retries are disabled unless one adapter story explicitly justifies them;
- FastGate may retry a provider/network failure only before response commitment and within a
  bounded reviewed policy;
- once streaming output is committed to the client, an automatic replay is normally unsafe;
- the harness does not add an invisible retry around FastGate; and
- usage and request identifiers must make duplicate attempts auditable without logging prompts.

## Caching boundary

Three different mechanisms must remain distinct:

| Layer | Authority | Enforcer/integrator | Observer |
| --- | --- | --- | --- |
| Provider prompt/prefix caching | Hosted provider or self-hosted runtime | Provider adapter and FastGate arrange cache-friendly requests | FastGate records bounded metrics; LatencyLab measures effects |
| Application response caching | Future explicit product policy | Future reviewed FastGate/application story | Correctness and privacy evaluation |
| Raw KV-cache management | Self-hosted inference runtime | Future runtime-specific project | LatencyLab and FleetSim experiments |

Cache keys and affinity eventually include tenant, model, tokenizer, model/adaptor version, and
security domain. A cache hit never substitutes for harness audit or workflow state.

## Control plane and data plane

FastGate's request path must continue safely under a defined, bounded TenantPlane outage policy. A
cached policy projection needs a version, expiry, and fail-safe behavior. TenantPlane remains the
authority for identities, permissions, quotas, and budgets; FastGate owns low-latency enforcement.

Usage events are append-only and potentially duplicated. TenantPlane deduplicates them using a
stable event identity. Provider-observed usage is not automatically equivalent to billable usage;
partial streams and client disconnects require an explicit accounting decision.

Model-alias authority also evolves explicitly:

- before TenantPlane exists, FastGate owns a reviewed static alias map;
- once TenantPlane is authoritative, it publishes tenant-visible alias and policy revisions;
- FastGate caches and enforces a versioned projection;
- ModelEndpoint resources advertise deployable endpoint identity, capabilities, health, and rollout,
  not tenant-facing alias policy; and
- FleetSim may recommend mappings offline but never publishes live authority.

## Observability and privacy

Telemetry must separate:

- time to first token;
- inter-token or streaming duration;
- total latency;
- queue and gateway overhead;
- upstream provider time;
- cancellation and failure category;
- provider/model route label; and
- bounded token and cache counts when available.

Prompt content, credentials, authorization headers, raw response bodies, and unbounded exception
messages are excluded from logs and metric labels by default. Cross-repository correlation uses
bounded opaque identifiers and standard trace context, not content.
