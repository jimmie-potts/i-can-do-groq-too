# ICGT-006 lesson: Defining FastGate model-turn v1 before provider code

- **Unit:** ICGT-006
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; the v1 schemas, fixtures, harness mapping, offline validator, and
  canonical-gate integration are implemented and validated
- **Story:** [ICGT-006](../../user-stories/icgt-006-select-fastgate-api.md)
- **Review priority:** High
- **Visual companion:** Not required; the source-backed Markdown lesson is the learning artifact
- **Related architecture:** [Accepted ADR 0003](../adr/0003-fastgate-api-surface.md),
  [Architecture](../architecture.md), and the
  [model-turn v1 contract](../../gateway/contracts/model-turn/v1/README.md)

> This lesson describes committed contract tooling, not an implemented inference endpoint. Schema
> conformance proves document shape; it does not prove authentication, provider work, streaming,
> cancellation, or cleanup.

## Quick summary

ICGT-006 turns ADR 0003's Option C into the first concrete FastGate client contract. It commits
three strict JSON schemas, 26 language-neutral examples, a field-by-field Code Assist Harness
mapping, and a dependency-free Python checker that runs in `./scripts/check`.

The key design lesson is sequencing: decide the client-visible meaning first, then make the later
provider domain serve that meaning. FastGate does not copy Chat Completions, Responses, or the
harness's internal types into its public contract.

## Learning objectives

After this lesson, you should be able to:

- explain why a northbound client contract precedes a southbound provider port;
- trace one valid request and one intended invalid case through the fixture checker;
- distinguish JSON syntax, schema-profile syntax, and instance validation;
- explain why unknown fields fail closed and why token counts need exact number handling;
- distinguish pre-dispatch unsupported capability from post-dispatch unsupported output; and
- describe which harness meanings are exact, lossy, unsupported, or deferred.

## Junior engineer foundation

**JSON** is a data format. **JSON Schema** is another JSON document that describes allowed data.
The schema does not receive network requests by itself; a program must load both documents and apply
the rules.

A **fixture** is a small example with a known expectation. Valid fixtures show supported shapes.
Invalid fixtures should fail for an intended rule, such as `additionalProperties` or `maximum`.
An invalid example is weak evidence if it happens to fail for an unrelated reason, so the manifest
records both the expected keyword and its JSON Pointer path.

This repository implements only the small Draft 2020-12 keyword profile its three schemas need. It
rejects unsupported schema keywords instead of silently pretending to understand them. That keeps
the validator reviewable and offline, but it is not a general-purpose replacement for a mature JSON
Schema library.

## Architecture and invariants

```text
language-neutral request/result/failure JSON
    -> strict JSON loading
    -> closed schema-profile check
    -> instance validation
    -> manifest expectation check

future runtime only:
client -> FastGate transport -> provider-neutral port -> provider adapter
```

The important invariants are:

- every document has exact `version` and `kind` framing;
- every object rejects unknown fields;
- request and model identifiers are bounded correlation/configuration values, not secrets or
  provider IDs;
- declared `tool_calls` is valid input but must later fail before provider dispatch;
- unsolicited upstream tool output is a different post-dispatch failure;
- optional usage may accompany success or failure and never authorizes billing or retry;
- schema validation does not claim a runtime echoed the same request ID; and
- streaming, cancellation, cleanup, HTTP binding, and provider behavior remain deferred.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [Contract README](../../gateway/contracts/model-turn/v1/README.md) and [request schema](../../gateway/contracts/model-turn/v1/schema/request.schema.json) | Define the public v1 promise, bounds, auth placeholder, and unknown-field policy | What can a caller express, and what can it not infer from schema conformance? |
| [Harness mapping](../../gateway/contracts/model-turn/v1/harness-mapping.md) | Preserves repository boundaries and exposes semantic loss | Which current harness meanings are exact, lossy, unsupported, or deferred? |
| [Case manifest](../../gateway/contracts/model-turn/v1/fixtures/cases.json), [minimal request](../../gateway/contracts/model-turn/v1/fixtures/valid/minimal-request.json), and [unknown-field failure](../../gateway/contracts/model-turn/v1/fixtures/invalid/unknown-request-field.json) | Make the contract language-neutral and personally inspectable | Does each invalid case name its intended rule and location? |
| [Offline checker](../../scripts/check_contract.py) | Owns strict loading, the closed keyword profile, path safety, and complete corpus execution | Can a malformed schema, duplicate fixture, symlink, orphan, or rounded token count escape? |
| [Checker tests](../../tests/test_check_contract.py) | Exercise success, failure, exact numbers, diagnostics, and manifest integrity | Do failures remain deterministic and content-free? |
| [PR review regression checklist](../pr-review-checklist.md) | Preserves missed invariant classes found after the first review | Which schema claims need a mutation test rather than another happy-path fixture? |

Personally review the request and failure schemas, the two capability-related valid fixtures, the
`check_contract` control flow, and the intended-rule mismatch test. No generated code or dependency
API hides the contract.

## Implementation walkthrough

### 1. Strict fields make version drift visible

The [request schema](../../gateway/contracts/model-turn/v1/schema/request.schema.json) requires all
seven request fields and closes the top-level object:

```json
  "type": "object",
  "additionalProperties": false,
  "required": [
    "version",
    "kind",
    "request_id",
    "model_alias",
    "conversation",
    "instructions",
    "required_capabilities"
  ],
  "properties": {
```

`additionalProperties: false` means a caller cannot add a field such as `temperature` and assume
FastGate used it. Every nested message, instruction, usage object, and failure is closed too.
Changing a required field, enum, bound, shape, or observable meaning requires another contract
version because an existing strict consumer could reject the change.

The request keeps instructions separate from conversation so a client does not have to flatten
repository guidance into an invented vendor message role. Both arrays preserve caller order.

### 2. One valid request shows the minimum honest shape

The [minimal request fixture](../../gateway/contracts/model-turn/v1/fixtures/valid/minimal-request.json)
keeps even empty instruction and capability arrays explicit:

```json
{
  "version": "v1",
  "kind": "model_turn.request",
  "request_id": "req-1",
  "model_alias": "learning-text",
  "conversation": [
    {
      "role": "user",
      "content": "Explain one model turn."
    }
  ],
  "instructions": [],
  "required_capabilities": []
}
```

`model_alias` is a FastGate routing name, not an OpenAI or Groq model ID. The future harness
adapter supplies it through its own reviewed configuration; the current harness provider request is
not changed to carry gateway routing.

### 3. The manifest makes failure intent executable

The [case manifest](../../gateway/contracts/model-turn/v1/fixtures/cases.json) does more than divide
files into valid and invalid directories. An invalid entry names the rule it is supposed to prove:

```json
{
  "name": "unknown-request-field",
  "schema": "schema/request.schema.json",
  "fixture": "fixtures/invalid/unknown-request-field.json",
  "valid": false,
  "expected": {
    "keyword": "additionalProperties",
    "instance_path": ""
  }
}
```

The empty JSON Pointer identifies the containing top-level object. If this fixture later fails only
because of some other accidental defect, the checker reports an intended-rule mismatch.

### 4. Strict loading happens before schema validation

The [checker](../../scripts/check_contract.py) opens each repository artifact once and requests only
the one-megabyte limit plus one byte. That extra byte distinguishes an exactly-at-limit file from an
oversized file without first allocating the whole artifact:

```python
with path.open("rb") as source:
    raw = source.read(MAX_JSON_BYTES + 1)
if len(raw) > MAX_JSON_BYTES:
    raise JSONArtifactError("JSON document exceeds the checker size limit")
```

After that allocation guard, it enforces 128-level nesting and exact-decimal implementation limits;
requires UTF-8; rejects duplicate object keys and JSON's non-standard `NaN`/`Infinity` spellings;
rejects decoded lone surrogates; and parses all numbers exactly:

```python
document = json.loads(
    text, object_pairs_hook=unique_object, parse_constant=reject_constant,
    parse_float=exact_decimal, parse_int=exact_decimal,
)
require_unicode_scalars(document)
return document
```

Exact decimal parsing matters near the token bound. Binary floating-point could round
`9007199254740990.5` into an integer-looking value. `Decimal` preserves the fraction, so the
schema's `integer` rule rejects it. A very large but syntactically valid number such as `1e999`
stays a number and then fails the applicable `maximum`; it is not mislabeled as malformed JSON.
The size, nesting, and numeric implementation-range guards protect this repository checker only. A
refusal at any guard is an artifact error, not a normative claim that the source document is
malformed JSON.

The manifest's symlink and path checks assume a stable checkout. They reject repository artifacts
that are already symlinks, but they are not an atomic defense against another process replacing a
path during validation. Opening once and reading through that descriptor keeps the byte bound true
even in that race; fully atomic component traversal would be platform-specific and is outside this
offline gate's threat model.

The checker next rejects any schema keyword outside its explicit profile. This prevents a future
author from adding `$ref`, `oneOf`, or another unsupported rule that the local validator would
otherwise ignore. The profile caps enum lists at 64 entries before its pairwise uniqueness check, so
a repository-controlled schema cannot turn the offline gate into unbounded quadratic work.

### 5. Mutation tests protect rules that examples cannot

A valid fixture containing `"version": "v1"` proves that v1 is accepted. By itself, it does not
prove the field remains required, that `v2` is rejected, or that the file keeps the right schema ID.
The checker therefore pins canonical metadata separately from instance validation:

```python
CANONICAL_SCHEMA_METADATA = {
    REQUEST_SCHEMA: ("urn:fastgate:model-turn:v1:request", "model_turn.request"),
    SUCCESS_SCHEMA: ("urn:fastgate:model-turn:v1:success", "model_turn.completed"),
    FAILURE_SCHEMA: ("urn:fastgate:model-turn:v1:failure", "model_turn.failed"),
}
```

Targeted checks give clear errors, but manually listing selected fields cannot prove every nested
rule remains frozen. The checker therefore also compares a SHA-256 fingerprint of each complete
validation-affecting schema tree:

```python
expected_fingerprint = V1_SCHEMA_VALIDATION_FINGERPRINTS.get(relative)
if schema_validation_fingerprint(schema) != expected_fingerprint:
    errors.append(
        f"schema {relative}: validation rules must match the frozen v1 contract"
    )
```

The fingerprint excludes only `title` and `description` when they are schema annotations. A property
or enum value that happens to be named `title` or `description` still affects the fingerprint. It sorts
object keys and the semantically unordered `required` and `enum` arrays, and it normalizes exact JSON
numbers. Formatting, annotation, or harmless ordering edits therefore do not look like protocol
changes, while any nested type, bound, property, pattern, constant, or enum change does. The three
expected fingerprints are independent review anchors; the gate also checks that their inventory
exactly matches the canonical schemas and never regenerates them automatically.

The schema profile also enforces closed nested objects and refuses to compile arbitrary patterns:

```python
if node.get("additionalProperties") is not False:
    add(path, "object schemas require additionalProperties to be false")

if node["pattern"] not in AUDITED_PATTERN_MATCHERS:
    add(path, "pattern must be an audited language-neutral expression")
```

The exact allowlist contains only the identifier and control-safe single-line schema `pattern`
expressions used by model-turn v1. It is intentionally narrower than either Python or general
ECMAScript regex support.
That prevents Python-only syntax from receiving a false language-neutral approval and makes every
future pattern addition a visible review decision. The current expressions use `(?![\\s\\S])` as a
true end check because `$` may match before a trailing newline.

The focused tests mutate one invariant at a time while keeping existing fixture values usable. They
prove the gate rejects mistyped, wrong-version, and duplicate IDs; broadened or optional framing;
removed or added canonical root fields; root and nested open objects; Python-only named groups; a
`model_alias` bound that drifts from `request_id`; and previously unexercised nested request,
completed-result, and failure rules. A separate assertion proves annotation and unordered-array
edits retain the same fingerprint. The reusable questions are recorded in the
[PR review checklist](../pr-review-checklist.md) so later reviews start with this evidence.

### 6. Every fixture must be accounted for

`check_contract` safely resolves each manifest path under either `schema/`,
`fixtures/valid/`, or `fixtures/invalid/`. It rejects absolute paths, `..`, aliases such as
`./`, symlink components, duplicate case names, duplicate fixture paths, missing files, and
unlisted fixture files.

The main happy/failure decision is intentionally small:

```python
violations = validate_instance(fixture, schema)
if case["valid"] and violations:
    first = violations[0]
    errors.append(
        f"{label}: expected valid but failed {first.keyword} at {first.instance_path!r}"
    )
elif not case["valid"] and not violations:
    errors.append(f"{label}: expected invalid but was accepted")
elif not case["valid"] and expected is not None and expected not in violations:
    first = violations[0]
    errors.append(
        f"{label}: intended rule mismatch; observed {first.keyword} "
        f"at {first.instance_path!r}"
    )
```

A valid case must have no violations. An invalid case must have at least one violation and must
include its intended one. Additional failures are allowed because one malformed document can
violate several independent rules.

Artifact failures are different. If a fixture is unreadable or exceeds the checker's local size
guard, no protocol document was parsed, so that operational refusal cannot satisfy an expected
`json` violation. `JSONArtifactError` takes a separate path and the focused suite writes an
oversized but valid request to prove it remains a checker error.

The meaningful failure test changes the expected keyword without changing the invalid fixture:

```python
self.cases["cases"][1]["expected"] = {
    "keyword": "maximum",
    "instance_path": "/count",
}
self.write_json(self.cases_path, self.cases)

errors = self.check()

self.assertTrue(any("intended rule mismatch" in error for error in errors), errors)
```

That test proves the checker does not merely celebrate any rejection.

### 7. Capability timing changes the meaning of failure

The schema accepts `required_capabilities: ["tool_calls"]` even though v1 has no tool schema. This
lets a future server reject the request honestly with `unsupported_capability` before starting
billable provider work.

If an upstream emits a tool call after dispatch, FastGate cannot claim it performed a preflight
rejection. The result instead uses `unsupported_upstream_output`, may preserve usage already
observed, and excludes raw tool arguments. The
[pre-dispatch fixture](../../gateway/contracts/model-turn/v1/fixtures/valid/failed-unsupported-capability.json)
has no usage; the
[post-dispatch fixture](../../gateway/contracts/model-turn/v1/fixtures/valid/failed-unsupported-upstream-output.json)
shows bounded usage.

That failure-side usage is exact on the FastGate wire but has two different harness mappings. Usage
on a no-text failure is lossy and currently unrepresentable because CAH accepts usage only after text
deltas and matching completed text. A future adapter must neither drop the usage silently nor invent
text; ICGT-020 must explicitly accept the loss or require a later CAH contract change. Only usage
after real observed text is deferred to ICGT-011's `model-turn-stream/v1` sequence.

The [harness mapping](../../gateway/contracts/model-turn/v1/harness-mapping.md) keeps the current
direct OpenAI adapter separate and pins the reviewed harness evidence at
`ce76b4f9a3be5ea49f252616db0ced6ec4e8cdd7`. It maps only what a future CAH-owned
`FastGateProvider` would need and names later owners for streaming, cancellation, ordinary cleanup,
forced local task reaping, authentication, and the cross-repo handoff. FastGate's generic output
text remains broader than CAH's terminal-text policy because CAH is one client; the future adapter
must reject a disallowed whole response as fixed safe `invalid_response`, never sanitize, truncate,
emit, or log it.

## Why Option C remains the choice

- **Chat Completions subset:** familiar and smaller, but its older message/completion conventions
  invite a compatibility claim broader than the tested behavior.
- **Responses subset:** closer to the current direct OpenAI harness path, but imports a larger
  vendor-owned typed event model that providers support unevenly.
- **FastGate-owned model turn:** adds an adapter obligation for every client, but exposes only
  bounded provider-neutral meaning this project owns.

Only Option C has schemas and fixtures. FastGate v1 is not an OpenAI base-URL replacement and claims
compatibility with neither rejected API.

## What changed during implementation

The first checker run failed before any case could be accepted. Human-readable case names used
spaces while the manifest profile required stable slugs, and enum/constant schema nodes omitted the
explicit string type required by the closed checker profile. The artifacts were corrected rather
than weakening validation.

Adversarial review then found that the harness may observe usage before a failed terminal. The first
failure schema would have lost that evidence, so failure results now allow the same optional bounded
usage object as completed results. The review also forced the contract to state that its required
`request_id` failure envelope begins only after a valid identifier can be recovered; ICGT-009 owns a
separate bounded transport rejection when no safe client identifier exists. Caller authentication
remains a later profile, and `authentication_failed` means upstream authentication only.

The numeric checker initially used binary floats. Review demonstrated a fractional count near the
JavaScript-safe integer limit could be rounded into a false integer. The final checker uses
standard-library `Decimal`, and focused tests preserve both integer and maximum semantics.

The completed contract review also made the parse profile normative, added duplicate-key and lone-
surrogate fixtures, and separated the checker's size, nesting, and exact-number implementation guards
from ICGT-009's future raw HTTP-body bound. It named the `model-turn-stream/v1` extension convention
so later stream, cancellation, and cleanup fixtures cannot silently change the base model-turn v1
meaning.

The pull-request review then found five invariants that valid fixtures did not lock by themselves:
canonical IDs, exact document framing, recursively closed objects, language-neutral pattern syntax,
and `model_alias`/`request_id` parity. The gate now checks those rules directly, and mutation tests
prove each failure. The lesson is not merely “add more fixtures”; it is to distinguish an example of
today's value from executable protection against tomorrow's schema drift.

An independent review of the fix found that the new framing check initially assumed every retained
canonical schema was an object. A profile-valid string root could therefore raise `KeyError`. The
checker now validates that structural assumption and returns a bounded error, with a mutation test
that proves the failure stays contained.

A second review wave challenged the boundaries around that validator. Local file-size, nesting, or
exact-decimal range failures could otherwise be mistaken for normative malformed JSON; artifact
failures now have their own error path and bounded diagnostics. Diagnostic paths escape control
characters, and enum uniqueness work is capped. Exact fractional usage and a valid escaped surrogate pair
moved into the shared corpus instead of living only in Python tests, and a regression preserves the
pair's source escape spelling. An independent Draft 2020-12 validator matches
all 23 schema-governed cases when fed exact decimals; its default binary-float parse misclassifies
the fractional value after rounding. That contrast is concrete evidence that consumers must
implement the contract's exact-number parse rule as well as its schema.

The same wave found architecture gaps. The CAH-specific `repository_instructions` public name became
generic ordered `instructions`, while the mapping still preserves CAH provenance, order, and content.
The harness snapshot was refreshed and the full operation lifecycle added lazy start,
single-consumer event claim, cancellation result distinctions, and `force_cancel_cleanup()`. The
result mapping now records terminal-text narrowing and no-text failure usage as real losses instead
of changing the generic FastGate contract or inventing observations.
Planned ICGT-007 now preserves optional failure usage. ICGT-009 owns complete admission and one
provider-neutral fake execution, including schema-valid `tool_calls` rejection with a zero-call test;
ICGT-010 owns the first loopback-only endpoint and exhaustive wire outcome mapping.

The next automated review found two remaining false assurances. First, selected invariant checks
still allowed a nested bound or enum to redefine v1 when no fixture reached that edge. The semantic
fingerprints now freeze every validation-affecting rule while leaving annotations and irrelevant
ordering editable. Second, `read_bytes()` checked length only after allocating the whole file. The
checker now reads at most `MAX_JSON_BYTES + 1`, and a recording-stream test verifies the underlying
read request, one binary open, success exactly at the limit, and rejection one byte above it rather
than merely observing the final error.

No visual companion was created. The project policy now treats the Markdown lesson as the required
learning artifact and visuals as optional. The governing ICGT-002 story was amended too, so the
current rule does not conflict with an older Done acceptance criterion.

## Failure scenarios to study

- An unknown request field is silently ignored before a paid operation.
- A valid fixture passes only because its schema used an unsupported keyword.
- A valid fixture still passes after its schema ID, framing, or duplicated bound drifts.
- A nested bound or enum changes without invalidating any current fixture or changing the version.
- A Python-only regular expression is mistaken for a language-neutral JSON Schema pattern.
- A nested object is opened even though the top-level object remains closed.
- An invalid fixture fails, but not for the rule its manifest claims.
- An oversized valid artifact is mistaken for normative malformed JSON.
- A file-size guard measures an already-unbounded allocation instead of constraining the read.
- A fractional usage count rounds into an integer at a large numeric boundary.
- A Python test covers a portability trap that never reaches the shared client corpus.
- A symlinked or escaping manifest path reads a file outside the contract corpus.
- One client's field name or terminal-text policy silently narrows the provider-neutral base contract.
- A pre-dispatch capability rejection is confused with post-dispatch unsupported output.
- A provider reports usage before failing and the client discards the observation.
- Local forced task cleanup is described as proof that remote provider work stopped.
- Schema success is described as proof of authentication, endpoint, cancellation, or cleanup.

## Production expansion

A production gateway would normally use a mature JSON Schema implementation, compile schemas once,
enforce a separate total HTTP-body byte limit, map status codes and authentication at the transport
boundary, and test its actual request/response handler against the same fixtures. This local checker
stays small because ICGT-006 validates committed artifacts, not untrusted production traffic.

ICGT-007 can now define provider-neutral non-streaming Go values downstream of this client contract,
including optional bounded usage observed before failure. ICGT-009 owns complete admission,
provider-neutral fake execution, and mandatory no-dispatch capability rejection; ICGT-010 owns the
loopback-only endpoint and all wire outcome mapping. ICGT-011 through
ICGT-015 add stream grammar, observable cancellation, and cleanup evidence. ICGT-018 provides the
first live-provider cleanup evidence, and ICGT-020 later freezes the cross-repository handoff.

## Practical exercises

- Add a temporary unknown field to the minimal request and predict the keyword and JSON Pointer.
- Change an invalid manifest entry's expected keyword and explain why the checker must fail.
- Temporarily broaden a framing `const` to an enum and confirm the canonical invariant check fails.
- Increase `conversation.maxItems` without adding a fixture and confirm the semantic lock still fails.
- Explain why `true` is not an integer even though Python's `bool` subclasses `int`.
- Compare `unsupported_capability` with `unsupported_upstream_output` in terms of paid work.
- Explain why an oversized valid fixture cannot prove a normative JSON parse rule.
- Find one lossy harness mapping and describe the future adapter's safe behavior.
- Draft the narrowest statement an admission test could add after ICGT-009.

## Key takeaways

- Client-visible meaning drives the provider seam, not the other way around.
- Closed schemas turn unsupported fields into visible failures.
- Fixtures are stronger when they name the exact rule they intend to prove.
- Mutation tests distinguish example coverage from invariant coverage.
- A complete semantic fingerprint backs up readable targeted checks without treating annotations or
  harmless ordering as protocol changes.
- Resource bounds must constrain the underlying operation before allocation, not inspect the result
  afterward.
- Exact pattern allowlists avoid overstating cross-language regex support.
- Exact decimal handling matters for integer contracts near large bounds.
- Cross-language hazards belong in the shared corpus, not only one implementation's tests.
- Usage is non-authoritative evidence and can exist even when a turn fails.
- Provider-neutral contracts stay generic even when one client supplies the first requirements.
- Contract tooling proves artifacts agree; it does not implement or certify a service.

## Glossary

- **Northbound:** the interface FastGate exposes to clients.
- **Southbound:** the interface FastGate uses to call provider adapters.
- **Conformance fixture:** a version-pinned example with an expected validation outcome.
- **Closed schema:** an object contract that rejects unknown fields.
- **JSON Pointer:** a path such as `/usage/input_tokens` locating a value in a JSON document.
- **Semantic loss:** meaning dropped or changed during translation.
- **Mutation test:** a test that deliberately changes one protected rule and expects the gate to fail.
- **Semantic fingerprint:** a digest of normalized validation meaning used to detect contract drift.
- **Audited allowlist:** the complete, explicitly reviewed set of values a checker may execute.
- **Pre-dispatch:** before provider work starts.
- **Post-dispatch:** after provider work may already have started or become billable.

## Teach-back questions

1. Why must the FastGate client contract be reviewed before ICGT-007 defines provider-domain types?
2. Why can a valid v1 fixture keep passing after a nested schema rule drifts, and how do the semantic
   fingerprint and mutation tests close that gap without locking prose or ordering?
3. Why are pre-dispatch unsupported capability and post-dispatch unsupported output separate error
   codes, and where may usage appear?

## Further reading

- [ICGT-006 delivery contract](../../user-stories/icgt-006-select-fastgate-api.md)
- [FastGate model-turn v1 contract](../../gateway/contracts/model-turn/v1/README.md)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [OpenAI migration guide: Chat Completions to Responses](https://developers.openai.com/api/docs/guides/migrate-to-responses)
