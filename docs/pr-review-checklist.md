# Pull-request review regression checklist

This checklist preserves defect classes that earlier reviews missed so later pull-request reviews
can challenge the same assumptions deliberately. It supplements the active story's human review
checkpoint; it does not replace understanding the changed behavior.

## How to use it

Before approving a pull request:

1. Read the story, lesson, relevant ADRs, and scoped `AGENTS.md` files.
2. Apply every section below that matches the changed files.
3. For each claimed invariant, ask whether committed happy-path fixtures merely demonstrate the
   current value or whether a focused mutation proves the gate rejects drift.
4. Run the focused tests and `./scripts/check` after the review changes.
5. When review exposes a reusable missed-case class, add its question and regression evidence here.

The recurring lesson is that **example coverage is not invariant coverage**. A fixture containing
`"version": "v1"` proves that value is accepted; it does not prove `v2` is rejected, the field stays
required, or the schema file keeps the correct identity. Mutation tests make that distinction
visible by changing one rule while leaving the existing fixtures otherwise valid.

## Public JSON Schema contracts

Apply this section whenever a pull request creates or changes a schema, schema checker, fixture
manifest, version rule, or duplicated public field.

### Canonical identity

- Does each canonical file have the exact `$id` assigned to that path and version?
- Are all canonical IDs unique?
- Would the gate reject a typo, a wrong version prefix, and a duplicate ID independently?

Why: checking only that `$id` is a non-empty string allows registries and consumers to resolve the
wrong document even while all example instances still pass.

### Exact document framing

- Do canonical root property names and required-field sets remain exact for that file and version?
- Are `version` and `kind` required at the document root?
- Are they strings with one exact `const`, rather than a broader enum or optional property?
- Does each canonical result type have its own expected kind?

Why: a fixture containing the current values does not stop a schema author from silently turning an
exact v1 document into a negotiable family.

### Complete version lock

- Does an independently anchored snapshot or semantic fingerprint include every
  validation-affecting schema value, including nested types, bounds, properties, patterns,
  constants, and enums?
- Does it exclude annotations contextually, so a schema annotation may change without accidentally
  ignoring a property or enum value that happens to be named `description` or `title`?
- Does it normalize only semantically unordered data such as object keys, `required`, and `enum`,
  rather than treating editorial reordering as a contract change?
- Do representative mutations change an unexercised nested rule in every canonical schema while
  keeping existing fixtures valid?
- Would updating a frozen v1 fingerprint be reviewed as a public-contract change rather than
  routine test maintenance?

Why: selected field checks and happy-path fixtures can both remain green after an unexercised nested
bound or enum silently changes the meaning of an already-published version.

### Closed objects

- Does every object schema require `additionalProperties` to be exactly `false`?
- Does the check recurse through nested objects, not only the root?
- Is there a mutation test that opens a nested object and proves the gate fails?

Why: accepting `true` lets clients send unsupported data that implementations may ignore or
interpret inconsistently.

### Language-neutral patterns

- Does the checker compile or execute only explicitly audited patterns?
- Would it reject syntax accepted by the checker language but not by other JSON Schema consumers?
- Is any newly allowed pattern reviewed for portability and denial-of-service risk before it enters
  the allowlist?

The model-turn v1 checker intentionally allows exactly two precompiled schema `pattern`
expressions: the identifier rule and the control-safe single-line rule. This is an audited
allowlist, not a claim of general ECMAScript regular-expression validation. The expressions use a
negative lookahead true-end check because `$` may also match immediately before a trailing newline.

### Duplicated semantic rules

- Which fields intentionally share one public rule across schemas or within a schema?
- Does the gate compare every copy, including same-document pairs such as `model_alias` and
  `request_id`?
- Would changing only one bound or pattern fail before a contract release?

Why: duplicated rules can drift without breaking fixtures whose values remain inside both the old
and new bounds.

### Checker failure containment

- Does the checker validate a node's shape before indexing fields that only that shape guarantees?
- Can a malformed or surprising schema produce a bounded diagnostic rather than a traceback?
- Are collections capped before any quadratic or otherwise expensive validation step?
- Are repository-controlled path names escaped before they reach terminal or CI diagnostics?
- Is there a mutation test for the checker's own structural assumptions, not only the public rule?

Why: hardening one invariant can introduce a new crash path if the new check assumes earlier
validation established more than it actually did.

### Protocol failures versus repository guard failures

- Can a checker I/O, file-size, path, nesting, numeric-range, or other implementation-limit failure
  accidentally satisfy a fixture that expects malformed protocol JSON?
- Does a byte limit constrain the underlying read before allocation—for example, by requesting at
  most `limit + 1`—instead of measuring an already-unbounded buffer?
- Does a mechanism-level test record requested and cumulative bytes rather than proving only the
  eventual oversized-file error?
- Are artifact failures reported separately from normative parse or schema violations?
- Do tests assert the exact failure class where one category subclasses another, rather than using
  a broad assertion that would accept either category?
- Are there regressions where oversized and deeply nested but syntactically valid artifacts cannot
  pass as the intended invalid protocol example?

Why: an operational guard proves only that the local checker refused an artifact. It must not become
evidence that another implementation should reject the document for the manifest's protocol rule.

### Static path policy versus atomic safety

- Does documentation distinguish rejecting symlinks in a stable repository snapshot from atomically
  preventing a concurrent path replacement?
- If concurrent mutation is outside the threat model, do resource limits still hold for the file
  actually opened?
- Does one opened descriptor own the bounded read, rather than a separate size check followed by an
  unbounded reopen?

Why: a pre-open path check can express repository policy without being race-proof. Overstating that
guarantee hides the real assumption, while a descriptor-bound limit still protects the checker from
an unexpectedly large target.

### Shared cross-language boundary cases

- Does the committed language-neutral corpus include the numeric, Unicode, and regular-expression
  cases most likely to behave differently across runtimes?
- Are both sides represented when useful, such as a valid escaped surrogate pair beside an invalid
  lone surrogate?
- When a fixture exists to exercise lexical parsing, does a test preserve the significant source
  spelling rather than only the equivalent decoded value?
- Does an independent consumer expose any parser limitation that the normative profile must make
  explicit?

Why: an implementation-only unit test can make one checker correct while every future consumer still
misses the portability trap.

## Cross-system contracts and mappings

Apply this section when a pull request maps another repository, client, provider, or lifecycle into
the local contract.

### Provider-neutral base vocabulary

- Does the public base contract describe generic meaning rather than one current client's type or
  field name?
- Is client-specific provenance translated at the adapter boundary without losing order or content?
- Would another legitimate client understand the field without knowing the mapped repository?

Why: one client is evidence for the contract, not the owner of every public name or bound.

### Snapshot freshness and complete surface comparison

- Is the external contract pinned to the current reviewed immutable revision?
- Did the review compare request, result, failure, operation creation/laziness, stream claim and
  cardinality, cancellation result values, ordinary cleanup, and forced cleanup instead of only the
  happy-path data types?
- Does the document distinguish implementation evidence from the later release/handoff pin?

Why: a stale or partial snapshot can make an exactness claim false even when every cited local file is
internally consistent.

### Lossy mappings and invented observations

- For every semantic, is the classification exact, lossy, unsupported, or deferred for the correct
  reason?
- Does a narrower client either reject an unsafe whole value with fixed safe behavior or document an
  intentionally lossy omission explicitly? Does it avoid silent narrowing, sanitizing, or false
  exactness in either case?
- Does a translation avoid inventing text, usage, completion, remote cancellation, or billing
  evidence merely to satisfy the destination grammar?

Why: a syntactically possible mapping can still fabricate an observation or erase meaning. Local
task/stream closure in particular does not prove remote provider termination.

### Carry observations through every planned seam

- If the public contract preserves optional usage or other evidence on failure, can the planned
  provider result, fake, endpoint mapping, and client adapter all carry it?
- Are absence and an observed zero represented distinctly?
- Is preserved evidence kept separate from retry authority, billing proof, and raw error detail?

Why: documenting an observation at the wire while a downstream type cannot represent it guarantees a
future silent drop.

## Story ownership and governing policy

### Earliest enforceable owner

- Is a mandatory rejection assigned to the first story that can actually enforce it, rather than a
  vague later capability milestone?
- Does the acceptance evidence prove the forbidden downstream call did not happen?
- Are later discovery, support, emulation, and routing work kept distinct from the initial rejection?

Why: “unsupported for now” is not an implementation plan. Paid or side-effecting work must be blocked
at the earliest executable admission boundary.

### Governing-source consistency

- When review changes a policy, were the governing story and acceptance criteria updated along with
  current guidance, templates, lessons, and indices?
- Does historical evidence remain clearly historical rather than becoming accidental precedent?
- Does repository validation enforce the authoritative policy without relying on prose in only one
  descendant document?

Why: fixing only the newest instructions leaves an older Done story or template as a conflicting
source of truth for later work.

## Evidence added after PR #4

[PR #4](https://github.com/jimmie-potts/i-can-do-groq-too/pull/4) exposed the schema, tooling,
portability, cross-system mapping, lifecycle, downstream-ownership, and policy classes above. The
fixes and executable evidence live in:

- [canonical invariant and pattern checks](../scripts/check_contract.py);
- [mutation-focused checker tests](../tests/test_check_contract.py);
- [ICGT-006 learning companion](lessons/icgt-006-selecting-client-protocol.md); and
- [ICGT-006 implementation note](../user-stories/notes/2026-08-02-icgt-006-model-turn-contract.md).

The current regression mutations cover mistyped, wrong-version, and duplicate schema IDs;
broadened or optional document framing; removed or added canonical root fields; root and nested open
objects; Python-only named-group syntax; `model_alias` bound drift; and a canonical non-object root
that must fail without a traceback. A complete semantic fingerprint additionally catches an
unexercised request bound, completed-output bound, and failure-code enum while ignoring annotations
and semantically irrelevant ordering. Oversized or deeply nested valid artifacts cannot satisfy a
normative JSON-failure expectation, and a recording stream proves the byte guard bounds the
underlying read before allocation, opens once, accepts the exact limit, and rejects one extra byte.
Strict-parser tests require the exact normative error class for
duplicate keys, non-finite spellings, lone surrogates, invalid syntax, and invalid UTF-8, so the
artifact-error subclass cannot accidentally satisfy them. Shared fixtures preserve the exact-number
and escaped-surrogate-pair portability cases, including the pair's source escape spelling. The
remaining checks are durable review questions because their evidence spans mapping and story
documents rather than one executable unit.
