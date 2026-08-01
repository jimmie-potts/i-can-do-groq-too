# Repository guidelines

## Purpose and product boundaries

I Can Do Groq Too is a learning-first inference infrastructure monorepo. The intended components are
FastGate, LatencyLab, ModelEndpoint Operator, TenantPlane, and FleetSim. The current repository is a
documentation and quality-gate foundation only; do not describe planned services as implemented.

`code-assist-harness` is a separate repository and client. It owns coding workflow state, tools,
approvals, audit records, transcripts, and correctness evaluation. This repository owns
provider transport, operational benchmarking, tenancy, deployment reconciliation, and capacity
simulation. Do not copy harness orchestration into FastGate or move gateway routing into the
harness.

## Repository organization

- `gateway/` is FastGate and is the first implementation target.
- `latency-lab/`, `model-operator/`, `control-plane/`, and `fleet-simulator/` contain status-honest
  component briefs until their first implementation stories begin.
- `docs/architecture.md` is the system boundary; accepted choices live in `docs/adr/`.
- `docs/lessons/` contains one learning companion for each implementation-ready story.
- `user-stories/` is the dependency-ordered delivery source of truth.
- `user-stories/notes/` records durable discoveries and validation without becoming a second
  backlog.

Do not add empty planned directories. Introduce source, test, deployment, or configuration paths in
the story that first uses them. Commit a language lockfile whenever dependency resolution changes.

## Small-unit workflow

Work on one approved story at a time. Before editing code:

1. Read the story, lesson, relevant ADRs, and scoped `AGENTS.md` files.
2. Explain the concepts, ownership boundaries, trade-offs, failure modes, and test strategy.
3. Produce a code-free plan for review.
4. Record unresolved architecture questions instead of silently choosing a framework or dependency.

Each code-bearing story should normally introduce one observable behavior and one important failure
mode. Aim for at most two new production abstractions and roughly 400 changed production lines,
excluding tests, generated files, lockfiles, and learning material. These are review heuristics, not
quotas; when a coherent unit must be larger, explain why it cannot be split safely before coding.

After implementation:

1. Summarize the important control flow and ownership decisions.
2. Identify the small set of production and test files the user should review personally.
3. Run focused tests and `./scripts/check`.
4. Run a separate review focused on correctness, concurrency, cleanup, security, and missing tests.
5. Update the linked lesson with exact source-backed examples, observed trade-offs, validation, and
   three teach-back questions.

## Human review contract

Every story has a **Human review checkpoint** naming:

- the most important production path;
- at least one failure, cancellation, or test path;
- the invariant the reviewer should be able to explain; and
- behavior deliberately deferred to later stories.

Do not hide core behavior behind generated code, reflection-heavy frameworks, or large dependency
APIs. Prefer explicit state and small interfaces. Generated code may support a boundary only when
the hand-written ownership and validation rules remain easy to inspect.

## Lesson requirements

Every implementation-ready story has exactly one Markdown companion under `docs/lessons/`. Planned
lessons must label pseudocode and future paths honestly. A completed code-bearing lesson must:

- link the exact important source and test paths;
- show focused excerpts for the happy path and one meaningful failure path;
- explain syntax and control flow in small logical chunks;
- compare the local implementation with a realistic production expansion;
- record what failed or changed during implementation; and
- end with exercises, takeaways, glossary terms, and teach-back questions.

High-review-priority code-bearing stories also receive a visual companion under
`docs/lessons/assets/`. Create the deck only after code and written evidence are stable, then render
and inspect it before treating it as completion evidence. Documentation-only foundation stories do
not require a deck unless a visual materially improves the decision review.

## Go conventions

Go is the default language for backend and infrastructure components. Do not create a Go module or
pin a version before ICGT-004 resolves the module/toolchain ADR. ICGT-005 then owns materializing
exactly that selected module/workspace layout and making the offline gate discover all of it.

Once Go exists:

- use idiomatic Go, `gofmt`, and GoDoc for exported APIs;
- pass `context.Context` explicitly across blocking, network, stream, and shutdown boundaries;
- do not create unbounded goroutines, queues, retries, caches, request bodies, or metric labels;
- make ownership and cleanup visible; every started goroutine or stream needs a stopping rule;
- wrap errors with safe operational context without copying credentials, prompts, response bodies,
  or raw provider errors;
- use table-driven tests when they improve boundary visibility; and
- include success, failure, cancellation, and timeout evidence for asynchronous behavior.

TypeScript may be introduced only for a UI where visualization materially improves learning. It
must not own gateway, control-plane, reconciliation, or scheduling decisions.

## Architecture rules

- Keep provider SDK and wire types inside provider adapters.
- Keep FastGate data-plane behavior separate from TenantPlane control-plane authority.
- Keep the harness's workflow limits separate from platform quota and rate-limit enforcement.
- Treat external providers, clients, networks, and usage events as unreliable or duplicative.
- Make provider capabilities explicit; never silently discard unsupported request fields.
- Avoid layered retries. FastGate may own narrowly defined provider/network retries; the harness
  owns workflow decisions and must not unknowingly retry the same billable operation.
- Propagate client cancellation through FastGate to the active upstream request.
- Do not log prompt contents, credentials, authorization headers, raw provider payloads, or
  unbounded error text by default.
- Reconciliation must be idempotent. Metering and audit ingestion must tolerate duplicates.
- FleetSim remains an offline model and never becomes an unreviewed live routing authority.
- Hosted-provider prompt caching is an optimization, not durable task state. Raw KV-cache control is
  out of scope until a self-hosted runtime story explicitly introduces it.

ADR 0003 selects a small FastGate-owned model-turn protocol as the initial public contract. ICGT-006
owns its exact schema, bounds, fixtures, and offline validation. Do not expose Responses or Chat
Completions as the first endpoint, claim OpenAI compatibility, or add a compatibility facade without
a separate reviewed client need.

## Testing and definition of done

The canonical gate is `./scripts/check`. It must remain offline and credential-free. Focused tests
speed iteration but do not replace the full gate before a story is marked Done.

A code-bearing story is Done only when:

1. Its acceptance criteria and human review checkpoint are satisfied.
2. Happy-path and meaningful failure-path tests pass.
3. Cancellation, timeout, cleanup, and bounds are tested when relevant.
4. No provider/network dependency enters a provider-neutral domain package.
5. Public contracts and non-obvious invariants are documented.
6. The linked lesson describes observed implementation rather than planned behavior.
7. The repository status, roadmap, and ADRs remain accurate.
8. The complete offline gate passes.

## Git and pull requests

Use `main` as the default branch. Use short imperative commit subjects and one logical story per
commit when practical. Prefer branch names such as `codex/icgt-005-bootstrap-fastgate`.

Do not commit unrelated user changes. Do not create a remote repository, push, open a pull request,
or mark one ready unless the user asks for publication. When publication is requested, report the
story, important review paths, validation commands, and remaining risks in the pull request.

## Security and configuration

Never commit credentials or `.env` files. A future `.env.example` contains names only and blank or
obviously fake values. Live-provider tests require explicit opt-in and remain excluded from the
default gate even when ambient credentials exist. Strip secrets from child-process environments and
test diagnostics. Avoid prompt logging by default; use bounded, non-content correlation metadata.
