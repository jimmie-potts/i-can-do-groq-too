# Code Assist Harness mapping for FastGate model-turn v1

## Snapshot and ownership

ICGT-006 reviewed merged Code Assist Harness commit
`ce76b4f9a3be5ea49f252616db0ced6ec4e8cdd7` (`Merge pull request #24 from
jimmie-potts/codex/implement-cah-023`). That commit is evidence for this mapping, not an immutable or
jointly published handoff. ICGT-021 owns an adapter-ready snapshot; a later harness story owns a
separate `FastGateProvider` and pins the packaged contract.

The reviewed harness sources are:

- `src/code_assist_harness/provider/models.py` for provider-neutral request and observation values;
- `src/code_assist_harness/model_evidence.py` for the authoritative JavaScript-safe usage bound;
- `src/code_assist_harness/provider/port.py` for operation, cancellation, and cleanup semantics;
- `src/code_assist_harness/provider_session.py` for one-turn grammar and terminal ownership;
- `src/code_assist_harness/provider/openai_config.py` and `openai_responses.py` for the direct OpenAI
  adapter; and
- `tests/provider/test_provider_models.py`, `tests/provider/test_fake.py`,
  `tests/test_provider_session.py`, and `tests/provider/test_openai_responses.py` for deterministic
  evidence.

This repository owns the FastGate wire schema and later server/provider translation. Code Assist
Harness owns its client adapter, session lifecycle, workflow limits, cancellation intent,
transcripts, and terminal truth. The CAH-023 direct OpenAI adapter remains a separate adapter and is
neither reused nor weakened by this contract.

## Classification terms

- **Exact:** the meaning can cross the boundary without reinterpretation. Adapter-generated framing
  fields may still be exact when they do not replace harness-owned meaning.
- **Lossy:** the mapping deliberately narrows a bound or collapses detail and names the consequence.
- **Explicitly unsupported:** v1 represents or detects the request but does not provide the behavior.
- **Deferred:** the non-streaming artifacts describe the boundary but a named later story must define
  and test its wire/runtime behavior.

## Request mapping

| FastGate v1 field or meaning | Current harness source | Classification | Mapping rule |
| --- | --- | --- | --- |
| `version = "v1"` | No `ProviderRequest` field | Exact | A future FastGate adapter inserts the pinned contract version. It does not alter the provider request. |
| `kind = "model_turn.request"` | No `ProviderRequest` field | Exact | The adapter inserts wire framing locally. |
| `request_id` | Command/session correlation lives outside `ProviderRequest` | Exact | The adapter creates and tracks an opaque per-operation correlation ID. It does not claim to preserve a harness command ID and does not treat the value as idempotency. |
| `model_alias` | Provider/model are harness adapter configuration, not `ProviderRequest` fields | Exact | The future adapter receives one reviewed logical alias through configuration and writes it into each FastGate request. It never copies CAH-023's OpenAI model ID implicitly. |
| Ordered `conversation[].role` | `ProviderMessage.role` is `user` or `assistant` | Exact | Preserve every role and its position. |
| `conversation[].content` | Non-empty valid UTF-8 text without a domain maximum | Lossy | FastGate v1 accepts at most 65,536 code points per message and 64 messages. The adapter must reject an oversized request locally before dispatch rather than truncate it. |
| Ordered `instructions[].source` | `RepositoryInstruction.source`, non-empty, control-safe, maximum 256 characters | Exact | Preserve each source and tuple position as one generic client-supplied instruction block. FastGate does not interpret the label as a repository path. |
| `instructions[].content` | `RepositoryInstruction.content`, non-empty text without a domain maximum | Lossy | FastGate accepts at most 32 generic instruction blocks and 65,536 code points per content value. The adapter must reject overflow before dispatch, never truncate or reorder it. |
| Empty `required_capabilities` | Current `ProviderRequest` has no tool declaration | Exact | The current harness profile always sends an empty array. |
| `required_capabilities = ["tool_calls"]` | Current request cannot declare tool schemas | Explicitly unsupported | The shape is valid for generic clients. ICGT-009 owns the v1 rejection decision/envelope and proves the fake was not called; ICGT-010 repeats and passes the ordering proof at the HTTP boundary. A future capability/tool story owns support. |
| Authentication | Not a provider-request field; CAH-023 reads its provider key only after explicit adapter selection | Deferred | ICGT-010 implements HTTP presentation without mounting a route. ICGT-011 owns actual-listener loopback enforcement and the first runtime composition, which remains unauthenticated. ICGT-021 records that explicit loopback/no-auth profile. A separate later FastGate story and profile implement authentication/TLS before non-loopback use, while the future harness adapter owns its trusted endpoint and credential configuration outside the body. |

FastGate bounds are public admission rules, not CAH workflow quotas. CAH's model-turn count,
provider-work deadline, assistant UTF-8 byte budget, and observed-tool limit remain harness-owned and
must not be copied into this request as platform quota fields.

## Result and observation mapping

| FastGate v1 meaning | Current harness meaning | Classification | Mapping rule |
| --- | --- | --- | --- |
| `output_text` | Ordered terminal-safe `ProviderTextDelta` values followed by one matching `ProviderTextCompleted` | Lossy for accepted characters and timing | FastGate accepts generic non-empty text, while CAH accepts TAB/LF but rejects every other C0/C1 control. For admissible text, a non-streaming adapter can preserve the value as one delta plus matching completion. For disallowed text it must reject the whole response locally as a fixed safe `invalid_response`, without sanitizing, truncating, emitting, or logging the value. Provider timing and chunks are also unavailable until streaming. |
| Text observed before a failed terminal | `ProviderTextDelta` and possibly `ProviderTextCompleted` may precede `ProviderFailed` | Deferred | The non-streaming failed result carries no observed text, so an adapter must not invent a legal text sequence. `model-turn-stream/v1`, owned by ICGT-012, must preserve the observation order before a failed terminal. |
| Non-empty output | A successful harness turn requires at least one non-empty delta; empty completed text cannot authorize success | Exact | FastGate v1 rejects an empty completed result. |
| Output maximum | CAH accepts at most its configured UTF-8 byte budget, never above the fixed 8,192-byte compatibility ceiling | Lossy | FastGate permits 65,536 code points for other clients. A future CAH adapter/session may reject a larger valid FastGate result under its own safety policy; it must not truncate it into success. |
| Provider-emitted tool request (`call_id`, `name`, and serialized arguments) | `ProviderToolCallRequested` preserves all three values for the harness loop | Explicitly unsupported | Non-streaming v1 has no successful tool-call representation. A later runtime must map unsolicited upstream tool output to `unsupported_upstream_output` without returning the arguments. Supporting tools requires a separately reviewed, versioned tool extension and owning story. |
| Completed-result `usage.input_tokens` and `usage.output_tokens` | `ProviderUsageReported`, each a non-negative JavaScript-safe integer, after matching completed text | Exact | Preserve both counters as non-authoritative evidence before emitting the harness completion. Absence stays absence. |
| Failed-result usage with no observed text | CAH rejects `ProviderUsageReported` unless non-empty deltas and matching `ProviderTextCompleted` came first | Lossy and currently unrepresentable | FastGate preserves the counters for other clients, but the current harness cannot consume them. A future adapter must not invent text or silently claim exactness. ICGT-021 may publish omission only as an explicitly lossy mapping; exactness requires a later CAH contract change. |
| Failure-side usage after observed text | CAH can accept usage after non-empty deltas and matching `ProviderTextCompleted`, before `ProviderFailed` | Deferred | `model-turn-stream/v1` and ICGT-012 must preserve the text-first observation order. Only that representable sequence can become exact without changing the harness. |
| `model_turn.completed` | `ProviderCompleted` after reconciled text and optional usage | Exact for non-streaming outcome | A future adapter emits one provider-neutral completion after mapping the result. Runtime and cleanup proof are not supplied by schema validation. |
| Shared failure code | `authentication_failed`, `rate_limited`, `request_rejected`, `unavailable`, or `invalid_response` | Exact where names match | Preserve the shared category, bounded message, and retryability. |
| `invalid_request` or `unsupported_capability` | No matching `ProviderFailureCode` | Lossy | A future adapter maps these safe pre-dispatch failures to `request_rejected` unless a reviewed harness change adds a distinct neutral category. |
| `unsupported_upstream_output` | Direct OpenAI unsupported output becomes `invalid_response` | Lossy | Map to `invalid_response` without copying upstream tool or payload details. FastGate retains the more precise post-dispatch category on its wire. |
| `internal_error` | Harness `unknown` | Lossy | Map to `unknown`; never expose internal text. |
| Error `message` | Non-empty, control-safe, maximum 1,024 characters | Exact | Preserve only the bounded normalized message. |
| Error `retryable` | Provider observation, never loop retry authority | Exact | Preserve the boolean without initiating a retry. |
| Raw provider error details | Deliberately absent from `ProviderFailure` | Explicitly unsupported | Neither contract carries SDK exceptions, bodies, headers, credentials, request IDs from an upstream, or raw payloads. |

The current harness provider port can express `ProviderToolCallRequested` with a call ID, name, and
unparsed serialized arguments. Its one-turn session does not execute the call: it selects
`tool_unavailable`, cancels the provider operation, and never parses the arguments. CAH-023 sends no
tool declarations and rejects unsolicited OpenAI tool output as `invalid_response`. FastGate v1
therefore exposes no tool-call result. It distinguishes a declared unsupported capability before
dispatch from `unsupported_upstream_output` after dispatch, without returning raw arguments.

## Terminal, cancellation, and cleanup mapping

| Harness semantic | Classification | v1 treatment |
| --- | --- | --- |
| Exactly one completed or failed terminal | Exact for one non-streaming response | The completed and failed schemas are disjoint by `kind`; ICGT-010 implements and tests status/framing for each non-terminated closed outcome, but no runtime route or client adapter exists yet. |
| `Provider.start()` creates a lazy operation; network work begins only when events are consumed | Deferred | A future CAH adapter owns lazy local construction. Neither ICGT-010's handler presentation nor ICGT-011's later runtime binding proves this client-side rule; ICGT-021 freezes it for the handoff. |
| `events()` is single-consumer and raises on a second claim | Deferred | The future CAH adapter owns the local single-claim guard. ICGT-012 defines FastGate stream grammar, while ICGT-021 freezes the cross-repository behavior. |
| A text-completed observation is not itself terminal | Exact for a successful non-streaming result; deferred before failure | A future adapter may synthesize the successful text observation before `ProviderCompleted`. Failure-side text completion requires `model-turn-stream/v1` under ICGT-012. |
| Partial deltas followed by failure | Deferred | The non-streaming failure body carries no partial output. ICGT-012 owns the stream grammar and ICGT-014 owns the first streamed endpoint behavior, including what admitted partial output remains visible. |
| Cancellation is control flow, not provider failure | Deferred | ICGT-015 owns transport cancellation intent, acknowledgement, races, and correlation. No cancellation document appears in this corpus. |
| No later event after terminal or accepted local cancellation | Deferred | A single parsed body has no later event, but runtime behavior is unproved. ICGT-012 owns terminal stream grammar; ICGT-015 owns the cancellation case. |
| `cancel()` is idempotent, closes the iterator, and distinguishes `cancelled` from `already_closed` | Deferred | The future CAH adapter preserves the two local outcomes. ICGT-015 owns FastGate cancellation intent, acknowledgement, and races; ICGT-021 freezes their mapping. Neither outcome is a provider failure. |
| `wait_closed()` is repeatable local cleanup confirmation | Deferred | Non-streaming schemas contain no cleanup evidence. ICGT-015 owns cancellation closure, while ICGT-016 owns deadline, cleanup grace, and bounded FastGate-local upstream reaping. |
| `force_cancel_cleanup()` is idempotent forced local task reaping | Deferred | The future CAH adapter owns this escape hatch for its local HTTP, stream, and task resources. ICGT-016 separately owns FastGate's server-side grace and bounded local upstream reaping, and ICGT-021 freezes both layers. Returning confirms only local adapter-owned closure, never remote provider termination. |
| Confirmed versus unconfirmed upstream cleanup | Deferred | Local socket/body/stream cleanup must never be described as proof that provider computation or billing stopped. ICGT-019 owns the first live-provider evidence; ICGT-021 owns the later harness handoff distinction. |

## Direct OpenAI adapter separation

CAH-023 is intentionally stricter and provider-specific: it fixes the official OpenAI endpoint,
allowlists `gpt-5.6-luna`, uses foreground streaming with `background=false` and `store=false`, fixes
reasoning effort `none` and context `current_turn`, sets `max_output_tokens=8192`, sends no tools,
disables SDK retries and ambient routing, validates one closed SDK event automaton, and locally closes
its SDK stream and client.

Those choices remain inside the direct adapter. A future FastGate adapter must use a separately
trusted FastGate endpoint, its own authentication configuration, the pinned `v1` contract, and a
logical FastGate model alias. FastGate is not an OpenAI base-URL replacement, and successful local
connection cleanup does not prove upstream cancellation.

## Honest compatibility statement

The v1 schema can represent the current harness's bounded ordered text request, map its ordered
repository guidance into generic instruction blocks, and preserve successful completed text and
usage plus safe normalized outcomes through a future adapter. FastGate also preserves optional
failure-side usage for other clients. Text-first streamed failures may become representable under ICGT-012;
no-text failure usage remains unrepresentable in the current harness and requires an explicit
ICGT-021 handoff decision or later CAH contract change. The rows above also record bound narrowing,
error-category collapse, unsupported tool capability, and deferred cancellation/cleanup behavior.
ICGT-021 freezes the candidate source/profile, ICGT-022 packages and validates it with a digest, and
a later harness-owned adapter story pins and tests that package. Until all three stages complete,
this document is reviewed design evidence rather than a joint runtime conformance claim.
