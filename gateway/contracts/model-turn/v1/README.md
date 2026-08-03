# FastGate model-turn contract v1

ICGT-006 owns these language-neutral contract artifacts. They define the JSON shapes FastGate and
future clients may pin before provider-domain or transport code exists. They do **not** implement an
HTTP endpoint, authenticate a caller, select a provider, perform inference, stream output, cancel
work, or prove cleanup behavior.

## Contract files

- `schema/request.schema.json` validates one non-streaming model-turn request.
- `schema/success.schema.json` validates one completed result.
- `schema/failure.schema.json` validates one normalized failure.
- `fixtures/cases.json` is the reviewable case manifest. Its `schema` and `fixture` values are relative
  to this `v1/` directory.
- `fixtures/valid/` and `fixtures/invalid/` contain language-neutral examples.
- `harness-mapping.md` maps this contract to the Code Assist Harness snapshot reviewed for ICGT-006.

All contract objects are strict: unknown properties are rejected at the top level and in nested
messages, instructions, usage, and errors. Every document carries both `version` and `kind` so a
consumer can reject the wrong contract before interpreting the rest of the body.

## Normative JSON parse profile

Before schema validation, a document must be UTF-8 JSON with unique object names and only the JSON
number spellings defined by RFC 8259; `NaN` and `Infinity` are not accepted. Decoded object names and
string values must contain Unicode scalar values, so a valid escaped surrogate pair is accepted as
its scalar while a lone surrogate is rejected. Numbers are compared by their exact mathematical
value: booleans are not integers, an integral decimal such as `1.0` is an integer, and a fractional
decimal remains fractional even near a large bound.

The checker's one-megabyte per-file read limit, 128-level decoded nesting limit, and exact-decimal
implementation range protect the offline repository gate. They are artifact guards, not model-turn
v1 rules or future HTTP admission limits. A syntactically valid document refused by any guard cannot
count as a normative `json` fixture failure.

The offline checker also pins each canonical file to its exact unique schema ID, exact root property
and required-field sets, exact `version` and `kind`, closed-object policy, and deliberately duplicated
identifier rules. It executes only the two committed, precompiled schema `pattern` expressions for
identifiers and control-safe strings. That exact audited allowlist prevents the Python checker from
accepting a pattern that a language-neutral JSON Schema consumer may interpret differently; adding
another pattern requires an explicit portability review and checker change.

The checker profile also caps a schema `enum` at 64 entries before comparing entries for uniqueness.
That bounds offline review work; it constrains repository schema artifacts, not values in a
model-turn document.

## Why this is a FastGate-owned contract

The rejected choices remain useful comparison points, but they are not additional schemas:

| Option | Useful property | Why it is not v1 |
| --- | --- | --- |
| A. Chat Completions subset | Familiar, comparatively small request/response surface | Its older message/completion conventions do not preserve every typed operation meaning the harness will need, and a partial implementation invites an overstated compatibility claim. |
| B. Responses subset | Closer to the harness's current direct OpenAI path and richer typed output | It imports a larger vendor-owned state machine and would bias FastGate toward uneven provider support. |
| C. FastGate-owned model turn | Names only the bounded provider-neutral behavior FastGate owns | Selected; clients need an adapter and the project must version its own contract carefully. |

OpenAI's official
[migration guide](https://developers.openai.com/api/docs/guides/migrate-to-responses) explains the
differences between its Chat Completions and Responses APIs. That provider comparison is
non-normative context; FastGate v1 claims compatibility with neither API.

## Request

A `model_turn.request` contains:

- `version`: exactly `v1`;
- `kind`: exactly `model_turn.request`;
- `request_id`: an opaque correlation value of 1 through 128 ASCII characters, beginning with an
  ASCII letter or digit and then using only letters, digits, `.`, `_`, `:`, or `-`;
- `model_alias`: a FastGate-owned logical model name with the same lexical bound as `request_id`;
- `conversation`: 1 through 64 ordered `user` or `assistant` messages, each with 1 through 65,536
  Unicode code points of content;
- `instructions`: 0 through 32 ordered `{source, content}` objects. A source contains 1
  through 256 code points and no C0/C1 control, DEL, U+2028, or U+2029 character. Content contains 1
  through 65,536 code points; and
- `required_capabilities`: 0 or 1 declared capabilities. The only v1 spelling is `tool_calls`.

These are decoded field bounds, not an aggregate request or raw JSON byte bound. A request using
many maximum-sized values can contain several million code points, and JSON escaping can make its
wire representation larger still. ICGT-009 must publish the exact raw-body byte cap and test it as
an additional transport-admission rule. It must state honestly when a shape-conforming v1 document
can still be rejected by that cap; the validator's fixture-file limit must not become the endpoint
limit accidentally.

Array order is semantic. `instructions` are generic caller-supplied instruction blocks kept separate
from conversation; `source` is an opaque provenance label, not necessarily a file or path. FastGate
does not discover instructions, apply client-specific precedence, or reinterpret their order.
`model_alias` is not a provider model ID, and a future routing layer may map it only through reviewed
configuration.

`request_id` is correlation, not an idempotency key. Reusing it does not authorize deduplication,
safe retry, replay, or reuse of a billable result.

The body has no credential field. Caller authentication is an out-of-body transport concern reserved
for a later non-loopback authentication profile and implementation story. The non-normative
placeholder is `Authorization: Bearer <credential>`; it shows where authentication belongs without
selecting a token format or implemented header contract. ICGT-010's first endpoint remains
loopback-only, must refuse inference-route startup on a non-loopback listener, and cannot claim
authenticated operation.

### Capability admission

`["tool_calls"]` is deliberately schema-valid so a client can state a requirement rather than hide
it in prompt text. Model-turn v1 has no tool definitions or tool result shape. The later runtime must
therefore reject that declared requirement **before provider dispatch** with
`unsupported_capability`. ICGT-009 owns that pre-dispatch admission rule and proves the fake is not
invoked; ICGT-010 repeats the integration test at the HTTP boundary and must observe zero provider
calls.

That is different from an upstream provider unexpectedly producing tool output after dispatch. A
later adapter must fail closed with `unsupported_upstream_output`; it must not label paid,
post-dispatch work as a preflight rejection and must not copy raw tool arguments into the error.

## Completed result

A `model_turn.completed` document returns the same `request_id` and one non-empty `output_text`
bounded to 65,536 Unicode code points. This provider-neutral transport text is not constrained by a
specific client's terminal policy; a client may reject a shape-valid value under a stricter local
safety rule. It may include one strict `usage` object containing
non-negative `input_tokens` and `output_tokens`, each no larger than 9,007,199,254,740,991.

The schema checks the identifier's shape, not equality across two separate documents. The later
runtime and endpoint contract tests must prove that a result echoes the admitted request's exact
identifier.

Usage is a provider observation. It is not billing proof, quota authority, permission to continue,
or evidence that a retry is free.

## Failed result

A `model_turn.failed` document returns the same `request_id` and one strict error object:

- `code` is one of `invalid_request`, `authentication_failed`, `rate_limited`,
  `request_rejected`, `unsupported_capability`, `unavailable`, `invalid_response`,
  `unsupported_upstream_output`, or `internal_error`;
- `message` contains 1 through 1,024 code points and excludes C0/C1 control, DEL, U+2028, and U+2029;
  and
- `retryable` is a boolean observation, not permission to retry.

It may also carry the same strict `usage` object as a completed result. This preserves bounded,
non-authoritative token evidence when an upstream reports usage before a later failure. A
pre-dispatch rejection normally has no usage because no provider work should have started.
ICGT-007 must preserve optional failure-side usage in the provider-neutral failed outcome, and
ICGT-010 later owns mapping that observation into this wire envelope.

The error codes have these v1 meanings:

| Code | Meaning inside the model-turn envelope |
| --- | --- |
| `invalid_request` | FastGate can recover a valid `request_id`, but the client input fails contract or semantic admission. |
| `authentication_failed` | FastGate could not authenticate to the selected upstream. Caller authentication is outside this envelope. |
| `rate_limited` | FastGate or the selected upstream refused work because a bounded rate or capacity limit was active. |
| `request_rejected` | An otherwise valid, supported request was refused for a non-rate policy reason. |
| `unsupported_capability` | FastGate rejected a declared requirement before provider dispatch. |
| `unavailable` | The selected route could not provide a usable upstream operation. |
| `invalid_response` | The upstream response was malformed or violated the selected adapter's contract. |
| `unsupported_upstream_output` | After dispatch, the upstream produced a valid semantic output that model-turn v1 cannot represent. |
| `internal_error` | FastGate failed unexpectedly and exposes no internal or provider details. |

ICGT-009 owns the fixed `unsupported_capability` message and `retryable: false` admission outcome.
ICGT-010 owns HTTP status mapping for every admission and provider outcome, plus retryability mapping
for the remaining codes. The schema validates the normalized envelope but does not prove that a
future runtime chose the truthful code or flag.

Provider exceptions, response bodies, headers, credentials, raw tool arguments, and unbounded text
do not belong in this shape. Usage remains observation rather than billing proof or retry authority,
including when it accompanies a failure.

This failed-result shape assumes FastGate has already parsed and admitted a valid `request_id` that
it can echo. ICGT-009 owns the bounded transport response for malformed or otherwise rejected input
when no safe identifier is available. Caller authentication is outside this envelope and remains
unimplemented through ICGT-010; `authentication_failed` means upstream authentication only. The first
ICGT-020/021 handoff remains loopback-only and unauthenticated. A separate FastGate
authentication/TLS implementation and reviewed profile must precede non-loopback use.

## Versioning rule

`v1` is an exact contract profile, not a negotiable family. Because unknown fields are rejected,
changing a required field, bound, enum, object shape, or observable meaning requires a new version
and a separate schema and fixture directory. Expanding an enum is also versioned because an existing
strict consumer may reject the new value. Editorial clarification that does not alter validation or
observable meaning may remain within v1.

Base contract versions use `gateway/contracts/model-turn/vN/` and schema IDs beginning
`urn:fastgate:model-turn:vN:`. An extension never changes the meaning of those base documents. Each
extension uses its own `gateway/contracts/model-turn-<name>/vN/` directory, schema-ID prefix, strict
case manifest, and fixtures. A semantic or fixture-expectation change within either a base contract
or an extension requires that contract's next version.

ICGT-011 owns materializing `gateway/contracts/model-turn-stream/v1/`; ICGT-014 owns a separately
versioned cancellation behavior profile; and ICGT-015 owns a separately versioned cleanup/deadline
profile. Those stories add their fixtures only when the corresponding behavior is executable. Tool
calls remain explicitly unsupported until a separately reviewed tool story names and versions its
own extension.

## Exact compatibility claim

The strongest document-level claim these artifacts support is:

> A document is shape-conformant with the FastGate model-turn v1 request, completed-result, or
> failed-result contract when its bytes pass the normative JSON parse profile and the decoded value
> validates against the corresponding committed schema.

The fixture manifest is executable evidence that the committed examples receive their recorded
classifications; examples are not extra constraints on arbitrary documents. Parse and schema
validation prove shape only. They do not prove an endpoint, HTTP status mapping, body admission,
authentication, runtime capability rejection, provider compatibility, inference behavior, stream
ordering, cancellation, cleanup, Code Assist Harness integration, Chat Completions compatibility,
or Responses compatibility.

## Fixture expectations

Each manifest entry supplies the exact `schema`, `fixture`, and `valid` fields. An invalid case also
names the JSON Schema keyword expected to reject it and an RFC 6901 `instance_path` inside
`expected`. Missing-property and additional-property failures point to the containing object because
no valid child value exists to point at.

The corpus favors representative boundaries over exhaustive generated combinations so a reviewer
can inspect every case. `scripts/check_contract.py` executes the manifest offline, but that contract
tooling does not turn these artifacts into service behavior.
