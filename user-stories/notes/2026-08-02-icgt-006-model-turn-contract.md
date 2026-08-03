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
`ce76b4f9a3be5ea49f252616db0ced6ec4e8cdd7`. That merged revision is
implementation evidence, not the later adapter-ready handoff pin. The harness keeps workflow,
adapter, stream-consumption, cancellation, and local cleanup ownership. FastGate keeps its wire
schema, admission, provider transport, and normalized outcomes.

## Locked contract choices

- The base path is `gateway/contracts/model-turn/v1/` with separate strict request, completed, and
  failed JSON Schemas.
- Every object rejects unknown fields. Every document has exact `version` and `kind` framing.
- Requests require a bounded opaque `request_id`, FastGate logical `model_alias`, 1 through 64
  ordered user/assistant messages, 0 through 32 generic ordered instruction blocks, and an explicit
  0-or-1 capability list. The mapping preserves CAH repository-instruction provenance without making
  that client-specific name part of the base protocol.
- `tool_calls` is the only v1 capability spelling. It is schema-valid so the later server can
  reject it before provider dispatch; v1 does not define tool schemas or tool results.
- Completed output is non-empty and bounded to 65,536 Unicode scalar values. Usage is optional,
  non-negative, and capped at the JavaScript-safe integer maximum.
- Failures use one bounded normalized error. Optional usage is allowed on a failure because an
  upstream may report token evidence before a later failed terminal.
- `unsupported_capability` means pre-dispatch rejection. `unsupported_upstream_output` means an
  already-dispatched upstream produced a semantic result v1 cannot represent.
- The required failure `request_id` envelope begins only after FastGate can recover a valid client
  identifier. ICGT-009 owns a separate bounded transport rejection when no safe ID can be recovered.
- Caller authentication is not implemented by ICGT-009 or ICGT-010; the envelope's
  `authentication_failed` code refers only to FastGate authenticating to its selected upstream.
- Caller authentication remains outside the body. `Authorization: Bearer <credential>` is a
  non-normative placeholder, not an implemented header contract.
- Base versions use `model-turn/vN`; independently versioned extensions use
  `model-turn-<name>/vN`, their own schema IDs, manifests, and fixtures. ICGT-011 owns
  `model-turn-stream/v1`; later cancellation and cleanup stories own separate behavior profiles.
- ICGT-009 owns the raw HTTP-body byte cap. The per-field schema bounds and the offline checker's
  size, nesting, and exact-number implementation guards do not silently select transport limits.

## Parse and validation profile

The contract first requires UTF-8 JSON with unique object names, standard finite JSON number
spellings, and decoded Unicode scalar values. Valid escaped surrogate pairs decode normally; lone
surrogates fail. Exact decimal parsing keeps `1.0` an integer while preserving a fractional value
near 9,007,199,254,740,991 instead of rounding it into a false integer.

`scripts/check_contract.py` implements only the Draft 2020-12 keywords used by the three schemas.
It rejects unsupported or malformed schema keywords, validates every manifest case, requires each
invalid fixture to include its intended keyword and JSON Pointer, rejects escaping or noncanonical
paths, rejects symlinks and duplicates, and reports orphaned fixture files. It also pins exact unique
schema IDs, root property/required sets, constant framing, recursively closed objects, two explicitly
audited patterns, a 64-entry enum cap, and duplicated identifier rules. Diagnostics name rules and
bounded escaped paths without echoing fixture values.

The reviewed checker is 637 nonblank lines, above the story's roughly-400-line review heuristic. The
extra scope was not speculative behavior: final review showed that Unicode-scalar parsing,
canonical-schema inventory, cross-schema parity, and complete valid/invalid coverage were required
for the existing conformance claim to be true. Deferring those checks would leave a false green
gate, while splitting them into another production module would not reduce the semantic review unit.
The lesson narrows personal review to the strict loader, schema profile, canonical inventory/parity,
and manifest control flow.

The final corpus contains 8 valid and 18 invalid examples. It includes request, success, failure,
upper-bound, unknown-field, unsupported-capability, post-dispatch failure, exact and fractional
usage, duplicate-key, valid escaped-surrogate-pair, and malformed-Unicode evidence.

## What changed under review

The first validator run exposed two artifact/profile disagreements: case names contained spaces even
though the manifest required lowercase slugs, and string enums/constants omitted the explicit type
required by the closed schema profile. The artifacts were fixed rather than weakening the checker.

Correctness review found that the initial failure schema discarded usage observed before a failed
terminal. Failure results now allow the same optional bounded usage object as successful results.
The same review required explicit admitted-ID scope, error-code meanings, named extension paths,
and a separate raw-body-limit handoff.

A deeper harness-grammar review showed that failure-side usage is not one uniformly deferred case.
The current harness accepts usage before failure only after text deltas and matching completed text,
while a non-streaming FastGate failure may carry usage with no observed text. The mapping classifies
that no-text case as lossy and currently unrepresentable; ICGT-020 may publish omission only as
explicitly lossy, while exactness requires a later CAH contract change. Only a failure after real observed text is deferred to ICGT-011's
`model-turn-stream/v1`. An adapter must not drop evidence silently or invent text.

The first numeric implementation used Python binary floats. A fractional count near the safe
integer boundary could round into an integer. The checker now uses standard-library `Decimal` for
both JSON integer and decimal spellings. A final language-neutral review found that escaped lone
surrogates could pass Python while the harness requires UTF-8-encodable text and Go may normalize
them; the normative parse profile and two strict-parser fixtures close that gap.

The first pull-request review exposed a different class of false green: valid examples still passed
when schema metadata or rules were broadened. Five focused comments led to exact per-file and unique
schema-ID checks, required constant `version`/`kind` framing, recursive
`additionalProperties: false`, an exact two-pattern portability allowlist, and
`model_alias`/`request_id` parity. Mutation tests now alter each of those rules and prove the gate
fails. These reusable review questions live in the
[PR review regression checklist](../../docs/pr-review-checklist.md), which repository instructions
require future reviews to consult and extend.

The independent response review then replaced one canonical schema with a profile-valid string
root. The first framing implementation indexed object-only fields and raised `KeyError`; it now
checks its own root-shape assumption and emits a bounded diagnostic. That regression is also in the
review checklist because validator hardening must not create new traceback paths.

Nine additional review comments exposed boundary and planning defects beyond the original schema
mutations:

- Repository artifact read, size, nesting, and numeric-range failures now use a separate error path, so an
  operational refusal cannot satisfy a manifest expectation for normative malformed JSON.
- The shared corpus—not only Python unit tests—now includes the fractional safe-integer case and a
  valid escaped surrogate pair whose lexical source spelling is regression-tested.
- The base request renamed CAH-specific `repository_instructions` to generic ordered
  `instructions`; the mapping owns the client-specific translation.
- The CAH snapshot was refreshed to `ce76b4f9a3be5ea49f252616db0ced6ec4e8cdd7` and reviewed across
  output validation, failure usage, ordinary cleanup, and `force_cancel_cleanup()`.
- FastGate kept generic output text. A future stricter CAH adapter rejects the entire incompatible
  value as fixed safe `invalid_response` without sanitizing, truncating, emitting, or logging it.
- ICGT-007 and ICGT-008 now carry failure-side usage through the provider contract and fake;
  ICGT-009 owns the mandatory pre-dispatch `tool_calls` rejection and proves the fake is not called,
  while ICGT-010 must repeat zero-call evidence at the HTTP boundary.
- The Done ICGT-002 story now agrees with the current user-approved policy: the Markdown lesson is
  mandatory and visuals are optional, never a completion gate.

These defect classes and the questions that expose them are recorded in the
[PR review regression checklist](../../docs/pr-review-checklist.md) for future reviews.

Independent follow-up review then pinned exact canonical root fields, made deeply nested artifacts
fail with bounded diagnostics rather than recursion errors, and completed the CAH lifecycle inventory
with lazy start, single-consumer events, cancellation results, and forced local reaping. The final
test review also replaced a broad superclass assertion with exact normative-error classification for
duplicate keys, non-finite spellings, lone surrogates, invalid syntax, and invalid UTF-8; otherwise a
future artifact-error regression could still leave that unit test green.

## Validation evidence

- `python3 scripts/check_contract.py` accepted all 8 valid fixtures and rejected all 18 invalid
  fixtures for their recorded outcomes.
- `python3 -m unittest tests.test_check_contract` passed all 32 focused checker tests.
- `python3 -m unittest discover -s tests -p 'test_*.py'` passed all 49 Python tests.
- An independently installed Draft 2020-12 validator with exact-decimal JSON decoding matched all 23
  schema-governed fixture classifications. With default binary-float decoding, its sole mismatch was
  the fractional usage case because the source number rounded before schema validation. The three
  `json` fixtures remain owned by the normative strict parse profile.
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
