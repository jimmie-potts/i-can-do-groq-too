# Code Assist Harness mapping for FastGate model-turn v1

## Snapshot and ownership

ICGT-006 reviewed Code Assist Harness commit `8870ba9907979accd16fbaa690d6d2a218fdb9de`
(`Harden Luna adapter boundaries`). That commit is evidence for this mapping, not an immutable or
jointly published handoff. ICGT-020 owns an adapter-ready snapshot; a later harness story owns a
separate `FastGateProvider` and pins the packaged contract.

The reviewed harness sources are:

- `src/code_assist_harness/provider/models.py` for provider-neutral request and observation values;
- `src/code_assist_harness/provider/port.py` for operation, cancellation, and cleanup semantics;
- `src/code_assist_harness/provider_session.py` for one-turn grammar and terminal ownership;
- `src/code_assist_harness/provider/openai_config.py` and `openai_responses.py` for the direct OpenAI
  adapter; and
- `tests/provider/test_provider_models.py`, `tests/test_provider_session.py`, and
  `tests/provider/test_openai_responses.py` for deterministic evidence.

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
| Ordered `repository_instructions[].source` | `RepositoryInstruction.source`, non-empty, control-safe, maximum 256 characters | Exact | Preserve source and tuple order. The label is not resolved as a path by FastGate. |
| `repository_instructions[].content` | Non-empty instruction text without a domain maximum | Lossy | FastGate accepts at most 32 instructions and 65,536 code points per content value. The adapter must reject overflow before dispatch, never truncate or reorder it. |
| Empty `required_capabilities` | Current `ProviderRequest` has no tool declaration | Exact | The current harness profile always sends an empty array. |
| `required_capabilities = ["tool_calls"]` | Current request cannot declare tool schemas | Explicitly unsupported | The shape is valid for generic clients, but the later v1 runtime must return `unsupported_capability` before provider work. A future capability/tool story owns support. |
| Authentication | Not a provider-request field; CAH-023 reads its provider key only after explicit adapter selection | Deferred | ICGT-009 owns the first endpoint binding; ICGT-020 freezes the server/client authentication split for the handoff. A future harness adapter owns its trusted endpoint and credential configuration outside the body. |

FastGate bounds are public admission rules, not CAH workflow quotas. CAH's model-turn count,
provider-work deadline, assistant UTF-8 byte budget, and observed-tool limit remain harness-owned and
must not be copied into this request as platform quota fields.

## Result and observation mapping

| FastGate v1 meaning | Current harness meaning | Classification | Mapping rule |
| --- | --- | --- | --- |
| `output_text` | Ordered `ProviderTextDelta` values followed by one matching `ProviderTextCompleted` | Exact for completed text; lossy for timing | A non-streaming adapter can preserve the final text and synthesize the neutral completed sequence required by the harness. It cannot preserve provider delta timing or chunk boundaries. Streaming owns that later mapping. |
| Text observed before a failed terminal | `ProviderTextDelta` and possibly `ProviderTextCompleted` may precede `ProviderFailed` | Deferred | The non-streaming failed result carries no observed text, so an adapter must not invent a legal text sequence. `model-turn-stream/v1`, owned by ICGT-011, must preserve the observation order before a failed terminal. |
| Non-empty output | A successful harness turn requires at least one non-empty delta; empty completed text cannot authorize success | Exact | FastGate v1 rejects an empty completed result. |
| Output maximum | CAH accepts at most its configured UTF-8 byte budget, never above the fixed 8,192-byte compatibility ceiling | Lossy | FastGate permits 65,536 code points for other clients. A future CAH adapter/session may reject a larger valid FastGate result under its own safety policy; it must not truncate it into success. |
| Provider-emitted tool request (`call_id`, `name`, and serialized arguments) | `ProviderToolCallRequested` preserves all three values for the harness loop | Explicitly unsupported | Non-streaming v1 has no successful tool-call representation. A later runtime must map unsolicited upstream tool output to `unsupported_upstream_output` without returning the arguments. Supporting tools requires a separately reviewed, versioned tool extension and owning story. |
| Completed-result `usage.input_tokens` and `usage.output_tokens` | `ProviderUsageReported`, each a non-negative JavaScript-safe integer, after matching completed text | Exact | Preserve both counters as non-authoritative evidence before emitting the harness completion. Absence stays absence. |
| Failed-result `usage.input_tokens` and `usage.output_tokens` | CAH accepts usage before failure only after non-empty deltas and matching `ProviderTextCompleted` | Deferred | FastGate preserves the counters for other clients, but v1 failure carries no observed text from which a CAH adapter can construct the required sequence. The adapter must not drop usage silently or invent text; `model-turn-stream/v1` and ICGT-011 own an exact mapping. |
| `model_turn.completed` | `ProviderCompleted` after reconciled text and optional usage | Exact for non-streaming outcome | A future adapter emits one provider-neutral completion after mapping the result. Runtime and cleanup proof are not supplied by schema validation. |
| Shared failure code | `authentication_failed`, `rate_limited`, `request_rejected`, `unavailable`, `invalid_response`, or `unknown` | Exact where names match | Preserve the shared category, bounded message, and retryability. `internal_error` maps to harness `unknown`. |
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
| Exactly one completed or failed terminal | Exact for one non-streaming response | The completed and failed schemas are disjoint by `kind`; endpoint status/framing remains a later transport decision. |
| A text-completed observation is not itself terminal | Exact for a successful non-streaming result; deferred before failure | A future adapter may synthesize the successful text observation before `ProviderCompleted`. Failure-side text completion requires `model-turn-stream/v1` under ICGT-011. |
| Partial deltas followed by failure | Deferred | The non-streaming failure body carries no partial output. ICGT-011 owns the stream grammar and ICGT-013 owns the first streamed endpoint behavior, including what admitted partial output remains visible. |
| Cancellation is control flow, not provider failure | Deferred | ICGT-014 owns transport cancellation intent, acknowledgement, races, and correlation. No cancellation document appears in this corpus. |
| No later event after terminal or accepted local cancellation | Deferred | A single parsed body has no later event, but runtime behavior is unproved. ICGT-011 owns terminal stream grammar; ICGT-014 owns the cancellation case. |
| `cancel()` is idempotent and closes the local event iterator | Deferred | ICGT-014 owns the equivalent FastGate client/runtime cancellation contract and its fixtures. |
| `wait_closed()` is repeatable local cleanup confirmation | Deferred | Non-streaming schemas contain no cleanup evidence. ICGT-014 owns cancellation closure, while ICGT-015 owns deadline and cleanup-grace behavior. |
| Confirmed versus unconfirmed upstream cleanup | Deferred | Local socket/body/stream cleanup must never be described as proof that provider computation or billing stopped. ICGT-018 owns the first live-provider evidence; ICGT-020 owns the later harness handoff distinction. |

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

The v1 schema can represent the current harness's bounded ordered text request, ordered repository
instructions, successful completed text and usage, and safe normalized outcome through a future
adapter. FastGate also preserves optional failure-side usage for other clients, but the current
harness cannot consume it exactly without the failure-side text sequence deferred to
`model-turn-stream/v1`. The rows above explicitly record that deferral, bound narrowing,
error-category collapse, unsupported tool capability, and deferred cancellation/cleanup behavior.
Until ICGT-020 and a later harness-owned adapter story freeze and test a shared package, this
document is reviewed design evidence rather than a conformance claim about either runtime.
