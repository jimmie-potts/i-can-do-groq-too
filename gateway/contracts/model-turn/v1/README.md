# FastGate model-turn contract v1

ICGT-006 owns these language-neutral contract artifacts. They define the JSON shapes FastGate and
future clients may pin independently of provider-domain or transport code. They do **not** implement an
HTTP endpoint, authenticate a caller, select a provider, perform inference, stream output, cancel
work, or prove cleanup behavior.

ICGT-010 separately implements and tests an injectable HTTP presentation handler for these shapes.
The default service does not mount it. The approved, implementation-ready
[ICGT-011 story](../../../../user-stories/icgt-011-bind-local-demo-runtime.md) remains Planned; under
[ADR 0005](../../../../docs/adr/0005-local-demo-runtime-profile.md), it will bind a stateless local
demo with concrete loopback `*net.TCPListener` enforcement and fail-fast bounded concurrency.

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

The checker opens each file once and requests at most 1,000,001 bytes: the one-megabyte limit plus
one byte that proves the artifact is oversized. It rejects that extra byte before decoding, so the
limit constrains the underlying allocation instead of measuring an already-unbounded read. That
limit, the 128-level decoded nesting limit, and the exact-decimal implementation range protect the
offline repository gate. They are artifact guards, not model-turn v1 rules or future HTTP admission
limits. A syntactically valid document refused by any guard cannot count as a normative `json`
fixture failure.

Manifest path checks reject committed symlinks and escapes in a stable checkout. They are a static
repository policy, not an atomic defense against another process replacing paths during the check.
The single-descriptor bounded read still preserves the memory limit if a concurrent replacement
occurs; the offline gate otherwise assumes its checkout is not being mutated while it runs.

The offline checker fingerprints every validation-affecting value in each canonical schema. It
normalizes parsed object order, the semantically unordered `required` and `enum` arrays, and equal
number spellings, while excluding only schema-node `title` and `description` annotations. Therefore
changing any nested type, property, bound, pattern, constant, or enum fails the frozen v1 lock even
when every current fixture still passes. The existing exact ID, root framing, closed-object, and
duplicated-rule checks remain because they provide more readable diagnostics than a fingerprint
alone.

The checker executes only the two committed, precompiled schema `pattern` expressions for
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
wire representation larger still. ICGT-009 implements an additional runtime raw-body cap of exactly
8 MiB, or 8,388,608 bytes, and permits at most 16 simultaneously open JSON object or array
containers. An otherwise valid exact-limit body may proceed; observing one additional byte rejects
it before parsing or provider dispatch.

The raw cap intentionally admits the maximum decoded fields in a straightforward ASCII encoding
while allowing a heavily escaped or multibyte shape-conforming v1 document to be rejected. It is
independent of the checker's 1,000,000-byte fixture-artifact guard and 128-level artifact depth guard;
those repository limits must not become runtime behavior accidentally.

Array order is semantic. `instructions` are generic caller-supplied instruction blocks kept separate
from conversation; `source` is an opaque provenance label, not necessarily a file or path. FastGate
does not discover instructions, apply client-specific precedence, or reinterpret their order.
`model_alias` is not a provider model ID. ICGT-009 accepts only the fixed logical
alias `learning-text` after capability admission. Any other schema-valid alias receives a generic
correlated `invalid_request` with `The request is invalid.`, `retryable: false`, no usage, and zero
provider dispatch. A future routing layer may expand that vocabulary only through reviewed
configuration and behavior.

`request_id` is correlation, not an idempotency key. Reusing it does not authorize deduplication,
safe retry, replay, or reuse of a billable result.

The body has no credential field. Caller authentication is an out-of-body transport concern reserved
for a later non-loopback authentication profile and implementation story. The non-normative
placeholder is `Authorization: Bearer <credential>`; it shows where authentication belongs without
selecting a token format or implemented header contract. The implemented ICGT-010 handler defines HTTP
presentation but does not mount a route. Planned ICGT-011 owns inference-route startup, concrete
loopback `*net.TCPListener` enforcement, a stateless fixed-output demo, and fail-fast concurrency after transport
preflight. It adds neither a Host allowlist nor caller authentication and cannot claim authenticated
or non-loopback operation.

### Capability admission

`["tool_calls"]` is deliberately schema-valid so a client can state a requirement rather than hide
it in prompt text. Model-turn v1 has no tool definitions or tool result shape. The ICGT-009 runtime
rejects that declared requirement **before alias selection and provider dispatch**
with `unsupported_capability`, the exact message
`The required capability is not supported.`, `retryable: false`, and no usage. A request combining
`tool_calls` with an unknown alias reports the capability failure. ICGT-009 proves the fake is not
invoked; ICGT-010 repeats the integration test at the HTTP boundary and observes zero provider
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
ICGT-007 preserves optional failure-side usage in the provider-neutral failed outcome, and ICGT-010
maps that observation into this wire envelope.

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

ICGT-009 owns the fixed `unsupported_capability` message and the generic correlated
`invalid_request` message `The request is invalid.`; both use `retryable: false`, omit usage, and
precede provider dispatch. An unexpected schema-to-provider inconsistency instead uses
`internal_error`, the fixed message `The request could not be processed.`, `retryable: false`, and no
usage rather than blaming the client or exposing the validation detail. ICGT-009 implements these
outcomes and verifies their exact bodies against the contract fixture.

After its one invocation, ICGT-009 also validates the returned provider alternative with the
provider-domain contract. ICGT-010 preserves a valid result, direct normalized failure, or matching
caller-context termination. An invalid result/error combination, raw or wrapped
error, or fabricated context termination becomes the same correlated fixed `internal_error` after one
dispatch. Invalid errors are not formatted, unwrapped through `errors.Is`/`errors.As`, retained, or
exposed; invalid result content is not exposed; and the fixed outcome has no usage. Runtime tests
cover raw, wrapped, non-comparable, and typed-nil errors plus invalid and mixed results.

ICGT-010 implements HTTP presentation for every admission and provider outcome, including status mapping
for every response-producing outcome and response abort for matching caller-context termination. For
a provider-origin failure, it copies the provider-owned `code`, `retryable`, and optional usage
observation unchanged; it does not reinterpret retryability. Admission owners author their fixed
code, safe message, retryability, and usage absence. The schema alone does not prove that a runtime
chose the truthful code or flag; the ICGT-010 handler tests supply that presentation evidence. The
delivered [ICGT-010 story](../../../../user-stories/icgt-010-present-model-turn-over-http.md) locks the
exact request target, transport-preflight precedence, statuses, media types, fixed provider messages,
and matching-context response abort. The handler exists, but runtime binding remains unimplemented.
The linked [ICGT-011 lesson](../../../../docs/lessons/icgt-011-safe-local-runtime.md) is planned and
must not be treated as runtime evidence until that story is implemented and validated.

Provider exceptions, response bodies, headers, credentials, raw tool arguments, and unbounded text
do not belong in this shape. Usage remains observation rather than billing proof or retry authority,
including when it accompanies a failure.

This failed-result shape assumes FastGate has already parsed and admitted a valid `request_id` that
it can echo. ICGT-009 trusts an ID only after the whole bounded body passes strict
JSON parsing and the ID independently satisfies the v1 identifier rule. Malformed, oversized,
read-failed, duplicate-key, invalid-Unicode, non-object, missing-ID, or invalid-ID input receives the
exact 16-byte ASCII response `invalid request\n`—the text `invalid request` followed by one line-feed
byte; that response is not a `model_turn.failed` document.
ICGT-010 assigns its HTTP status and media type. Caller authentication is outside this envelope
and remains unimplemented through ICGT-011; `authentication_failed` means upstream authentication
only. The first ICGT-021/022 handoff may remain loopback-only and unauthenticated only after its
review explicitly selects and tests Host, Origin, CORS, DNS-rebinding, and caller-authentication
policy for the live-provider runtime. A separate FastGate authentication/TLS implementation and
reviewed profile must precede non-loopback use.

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

ICGT-012 owns materializing `gateway/contracts/model-turn-stream/v1/`; ICGT-015 owns a separately
versioned cancellation behavior profile; and ICGT-016 owns a separately versioned cleanup/deadline
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
