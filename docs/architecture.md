# Architecture

**Status:** Accepted component boundaries with an incrementally implemented target.

This document describes ownership and integration rules. It does not claim that the planned
components exist. See the [README](../README.md) and [story index](../user-stories/README.md) for
current implementation status.

## System context

```text
                          separate client repository
                 +--------------------------------------+
                 | Code Assist Harness                  |
                 | workflow | tools | approvals | audit |
                 +------------------+-------------------+
                                    |
                      provider-neutral model turn
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

That port should not be redesigned for FastGate now. The planned direct OpenAI adapter remains the
first baseline in the locked CAH-023 sequence. FastGate later receives its own harness adapter
rather than masquerading as the official OpenAI endpoint.

```text
Harness Provider implementations
  - FakeProvider                  implemented in the harness
  - OpenAIProvider               planned direct baseline
  - FastGateProvider             future, separate adapter

FastGate upstream implementations
  - Deterministic fake upstream  first local infrastructure proof
  - OpenAI upstream              first live baseline
  - Groq upstream                later measured expansion
  - Self-hosted upstream         later cache/runtime experiments
```

### Changes the harness may need later

These are future stories, not current prerequisites:

1. **A separate FastGate provider adapter.** It implements the existing harness port and has its own
   endpoint and logical-model-alias configuration. It must not weaken the fixed official endpoint
   and exact model allowlist in the planned CAH-023 direct OpenAI contract.
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

## The real integration tension: API semantics

The original FastGate idea used `POST /v1/chat/completions`. The locked CAH-023 plan makes the
harness's first live adapter a strict subset of OpenAI Responses. Pointing that future adapter at
FastGate would conflict with the planned safety contract: CAH-023 fixes the official OpenAI
endpoint, rejects ambient base-URL routing, uses a strict Responses event automaton, and
intentionally excludes routing and retries.

Therefore:

- do not repoint the harness's OpenAI adapter at FastGate;
- do not claim that Chat Completions and Responses are interchangeable;
- preserve direct OpenAI as the measurable baseline; and
- resolve FastGate's external protocol in [ADR 0003](adr/0003-fastgate-api-surface.md) before its
  first endpoint story.

A separate FastGate adapter can translate whichever reviewed protocol is chosen into the existing
harness event model.

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
