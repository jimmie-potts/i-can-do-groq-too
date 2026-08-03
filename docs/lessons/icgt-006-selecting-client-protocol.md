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
three strict JSON schemas, 24 language-neutral examples, a field-by-field Code Assist Harness
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
    "repository_instructions",
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
  "repository_instructions": [],
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

The [checker](../../scripts/check_contract.py) loads each repository artifact with a one-megabyte
safety limit, requires UTF-8, rejects duplicate object keys, rejects JSON's non-standard `NaN` and
`Infinity` spellings, rejects decoded lone surrogates, and parses all numbers exactly:

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

The checker next rejects any schema keyword outside its explicit profile. This prevents a future
author from adding `$ref`, `oneOf`, or another unsupported rule that the local validator would
otherwise ignore.

### 5. Every fixture must be accounted for

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

### 6. Capability timing changes the meaning of failure

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

That failure-side usage is exact on the FastGate wire but deferred for the current harness mapping.
The harness accepts usage before failure only after text deltas and matching completed text, while
the non-streaming FastGate failure carries no observed text. A future adapter must neither drop the
usage silently nor invent text; ICGT-011's `model-turn-stream/v1` profile owns the exact sequence.

The [harness mapping](../../gateway/contracts/model-turn/v1/harness-mapping.md) keeps the current
direct OpenAI adapter separate. It maps only what a future CAH-owned `FastGateProvider` would need
and names later owners for streaming, cancellation, cleanup, authentication, and the cross-repo
handoff.

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
`request_id` failure envelope begins only after a valid identifier can be recovered; ICGT-009 and
ICGT-010 own malformed or unauthenticated transport responses with no safe client identifier.

The numeric checker initially used binary floats. Review demonstrated a fractional count near the
JavaScript-safe integer limit could be rounded into a false integer. The final checker uses
standard-library `Decimal`, and focused tests preserve both integer and maximum semantics.

The completed contract review also made the parse profile normative, added duplicate-key and lone-
surrogate fixtures, and separated the checker's one-megabyte artifact guard from ICGT-009's future
raw HTTP-body bound. It named the `model-turn-stream/v1` extension convention so later stream,
cancellation, and cleanup fixtures cannot silently change the base model-turn v1 meaning.

No visual companion was created. The project policy now treats the Markdown lesson as the required
learning artifact and visuals as optional.

## Failure scenarios to study

- An unknown request field is silently ignored before a paid operation.
- A valid fixture passes only because its schema used an unsupported keyword.
- An invalid fixture fails, but not for the rule its manifest claims.
- A fractional usage count rounds into an integer at a large numeric boundary.
- A symlinked or escaping manifest path reads a file outside the contract corpus.
- A pre-dispatch capability rejection is confused with post-dispatch unsupported output.
- A provider reports usage before failing and the client discards the observation.
- Schema success is described as proof of authentication, endpoint, cancellation, or cleanup.

## Production expansion

A production gateway would normally use a mature JSON Schema implementation, compile schemas once,
enforce a separate total HTTP-body byte limit, map status codes and authentication at the transport
boundary, and test its actual request/response handler against the same fixtures. This local checker
stays small because ICGT-006 validates committed artifacts, not untrusted production traffic.

ICGT-007 can now define provider-neutral non-streaming Go values downstream of this client contract.
ICGT-009 and ICGT-010 own endpoint admission and normalized transport failures. ICGT-011 through
ICGT-015 add stream grammar, observable cancellation, and cleanup evidence. ICGT-018 provides the
first live-provider cleanup evidence, and ICGT-020 later freezes the cross-repository handoff.

## Practical exercises

- Add a temporary unknown field to the minimal request and predict the keyword and JSON Pointer.
- Change an invalid manifest entry's expected keyword and explain why the checker must fail.
- Explain why `true` is not an integer even though Python's `bool` subclasses `int`.
- Compare `unsupported_capability` with `unsupported_upstream_output` in terms of paid work.
- Find one lossy harness mapping and describe the future adapter's safe behavior.
- Draft the narrowest statement a server test could add after ICGT-009 implements an endpoint.

## Key takeaways

- Client-visible meaning drives the provider seam, not the other way around.
- Closed schemas turn unsupported fields into visible failures.
- Fixtures are stronger when they name the exact rule they intend to prove.
- Exact decimal handling matters for integer contracts near large bounds.
- Usage is non-authoritative evidence and can exist even when a turn fails.
- Contract tooling proves artifacts agree; it does not implement or certify a service.

## Glossary

- **Northbound:** the interface FastGate exposes to clients.
- **Southbound:** the interface FastGate uses to call provider adapters.
- **Conformance fixture:** a version-pinned example with an expected validation outcome.
- **Closed schema:** an object contract that rejects unknown fields.
- **JSON Pointer:** a path such as `/usage/input_tokens` locating a value in a JSON document.
- **Semantic loss:** meaning dropped or changed during translation.
- **Pre-dispatch:** before provider work starts.
- **Post-dispatch:** after provider work may already have started or become billable.

## Teach-back questions

1. Why must the FastGate client contract be reviewed before ICGT-007 defines provider-domain types?
2. How does the manifest prove an invalid fixture failed for its intended reason rather than an
   accidental one?
3. Why are pre-dispatch unsupported capability and post-dispatch unsupported output separate error
   codes, and where may usage appear?

## Further reading

- [ICGT-006 delivery contract](../../user-stories/icgt-006-select-fastgate-api.md)
- [FastGate model-turn v1 contract](../../gateway/contracts/model-turn/v1/README.md)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [OpenAI migration guide: Chat Completions to Responses](https://developers.openai.com/api/docs/guides/migrate-to-responses)
