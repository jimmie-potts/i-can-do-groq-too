# 2026-08-02 ICGT-006 model-turn v1 contract

- **Story:** [ICGT-006](../icgt-006-select-fastgate-api.md)
- **Decision:** [ADR 0003](../../docs/adr/0003-fastgate-api-surface.md)
- **Contract:** [FastGate model-turn v1](../../gateway/contracts/model-turn/v1/README.md)
- **Lesson:** [Defining FastGate model-turn v1](../../docs/lessons/icgt-006-selecting-client-protocol.md)
- **Visual:** Not required; the Markdown lesson is the learning artifact

## Observation

ICGT-006 began after the FastGate lifecycle but before any inference endpoint or provider-domain
type. That order allowed the client-visible contract to drive the provider seam. The selected
Option C remains a small FastGate-owned model turn rather than a Chat Completions or Responses
subset. Only Option C receives schemas and fixtures; the rejected APIs remain non-normative
comparison evidence.

The Code Assist Harness mapping was reconciled against commit
`8870ba9907979accd16fbaa690d6d2a218fdb9de` (`Harden Luna adapter boundaries`). That revision is
implementation evidence, not the later adapter-ready handoff pin. The harness keeps workflow,
adapter, stream-consumption, cancellation, and local cleanup ownership. FastGate keeps its wire
schema, admission, provider transport, and normalized outcomes.

## Locked contract choices

- The base path is `gateway/contracts/model-turn/v1/` with separate strict request, completed, and
  failed JSON Schemas.
- Every object rejects unknown fields. Every document has exact `version` and `kind` framing.
- Requests require a bounded opaque `request_id`, FastGate logical `model_alias`, 1 through 64
  ordered user/assistant messages, 0 through 32 ordered repository instructions, and an explicit
  0-or-1 capability list.
- `tool_calls` is the only v1 capability spelling. It is schema-valid so the later server can
  reject it before provider dispatch; v1 does not define tool schemas or tool results.
- Completed output is non-empty and bounded to 65,536 Unicode scalar values. Usage is optional,
  non-negative, and capped at the JavaScript-safe integer maximum.
- Failures use one bounded normalized error. Optional usage is allowed on a failure because an
  upstream may report token evidence before a later failed terminal.
- `unsupported_capability` means pre-dispatch rejection. `unsupported_upstream_output` means an
  already-dispatched upstream produced a semantic result v1 cannot represent.
- The required failure `request_id` envelope begins only after FastGate can recover a valid client
  identifier. ICGT-009 and ICGT-010 own malformed or caller-authentication failures without one.
- Caller authentication remains outside the body. `Authorization: Bearer <credential>` is a
  non-normative placeholder, not an implemented header contract.
- Base versions use `model-turn/vN`; independently versioned extensions use
  `model-turn-<name>/vN`, their own schema IDs, manifests, and fixtures. ICGT-011 owns
  `model-turn-stream/v1`; later cancellation and cleanup stories own separate behavior profiles.
- ICGT-009 owns the raw HTTP-body byte cap. The per-field schema bounds and the offline checker's
  one-megabyte artifact guard do not silently select that transport limit.

## Parse and validation profile

The contract first requires UTF-8 JSON with unique object names, standard finite JSON number
spellings, and decoded Unicode scalar values. Valid escaped surrogate pairs decode normally; lone
surrogates fail. Exact decimal parsing keeps `1.0` an integer while preserving a fractional value
near 9,007,199,254,740,991 instead of rounding it into a false integer.

`scripts/check_contract.py` implements only the Draft 2020-12 keywords used by the three schemas.
It rejects unsupported or malformed schema keywords, validates every manifest case, requires each
invalid fixture to include its intended keyword and JSON Pointer, rejects escaping or noncanonical
paths, rejects symlinks and duplicates, and reports orphaned fixture files. Diagnostics name rules
and paths without echoing fixture values.

The final checker is 530 nonblank lines, above the story's roughly-400-line review heuristic. The
extra scope was not speculative behavior: final review showed that Unicode-scalar parsing,
canonical-schema inventory, cross-schema parity, and complete valid/invalid coverage were required
for the existing conformance claim to be true. Deferring those checks would leave a false green
gate, while splitting them into another production module would not reduce the semantic review unit.
The lesson narrows personal review to the strict loader, schema profile, canonical inventory/parity,
and manifest control flow.

The final corpus contains 7 valid and 17 invalid examples. It includes request, success, failure,
upper-bound, unknown-field, unsupported-capability, post-dispatch failure, exact usage, duplicate
key, and malformed-Unicode evidence.

## What changed under review

The first validator run exposed two artifact/profile disagreements: case names contained spaces even
though the manifest required lowercase slugs, and string enums/constants omitted the explicit type
required by the closed schema profile. The artifacts were fixed rather than weakening the checker.

Correctness review found that the initial failure schema discarded usage observed before a failed
terminal. Failure results now allow the same optional bounded usage object as successful results.
The same review required explicit admitted-ID scope, error-code meanings, named extension paths,
and a separate raw-body-limit handoff.

A deeper harness-grammar review showed that failure-side usage is not yet exact for the future CAH
adapter: the current harness accepts usage before failure only after text deltas and matching
completed text, while the non-streaming FastGate failure carries no observed text. The mapping now
classifies that cross-repository sequence as deferred to ICGT-011's `model-turn-stream/v1`; an
adapter must not drop the usage silently or invent text.

The first numeric implementation used Python binary floats. A fractional count near the safe
integer boundary could round into an integer. The checker now uses standard-library `Decimal` for
both JSON integer and decimal spellings. A final language-neutral review found that escaped lone
surrogates could pass Python while the harness requires UTF-8-encodable text and Go may normalize
them; the normative parse profile and two strict-parser fixtures close that gap.

## Validation evidence

- `python3 scripts/check_contract.py` accepted all 7 valid fixtures and rejected all 17 invalid
  fixtures for their recorded outcomes.
- `python3 -m unittest tests.test_check_contract` passed all 21 focused checker tests.
- `python3 -m unittest discover -s tests -p 'test_*.py'` passed all 38 Python tests.
- An independently installed Draft 2020-12 validator matched all 21 schema-governed fixture
  classifications; the three strict-parse fixtures remain owned by the normative JSON profile.
- `./scripts/check` passed environment isolation, repository policy and links, the complete Python
  suite, the explicit model-turn contract stage, Go toolchain/module/format/vet/tests, race
  prerequisites, and race tests.
- Independent reviews covered contract semantics and harness ownership, strict parsing and schema
  validation, path/diagnostic safety, status honesty, and the source-backed lesson.
- `git diff --check` passed.

No provider credential, external dependency, network call, `go.sum`, visual lesson, endpoint,
provider port, SDK, runtime capability check, streaming behavior, or harness adapter was added.

## Follow-up

ICGT-007 is next. It may define only the smallest provider-neutral non-streaming Go request, result,
failure, and port needed by ICGT-008's deterministic fake. It must stay downstream of model-turn v1
and must not implement HTTP, a live provider, streaming, or harness orchestration.
