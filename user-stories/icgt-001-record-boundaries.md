# ICGT-001 - Record architecture boundaries and decisions

- **Status:** Done
- **Milestone:** M0 - Repository and learning foundation
- **Dependencies:** None
- **Lesson:** [Repository boundaries](../docs/lessons/icgt-001-repository-boundaries.md)
- **Review priority:** High

## User story

> As the learner and system owner, I want the five projects and Code Assist Harness to have explicit
> ownership boundaries so that later code does not merge agent behavior, inference transport,
> tenancy, deployment, and scheduling accidentally.

## Scope

- Recover and summarize the source conversation.
- Define current-versus-planned component status.
- Record the monorepo and separate-harness decision.
- Reconcile the actual current harness provider port and roadmap.
- Record provider, retry, limit, audit, caching, framework, and operator ownership.
- Preserve the FastGate API mismatch as a proposed ADR.

## Acceptance criteria

1. Root and component READMEs state that no services are implemented.
2. Architecture names one owner for every cross-project concern.
3. ADR 0001 keeps this repository separate from the harness.
4. ADR 0002 reconciles fake-first with OpenAI-first-live sequencing.
5. ADR 0003 describes Responses versus Chat Completions without pretending they are equivalent.
6. The harness direct OpenAI adapter is preserved as a baseline; FastGate requires a separate future
   adapter.
7. Current harness Git/worktree and remote-worker behavior is not overclaimed.
8. All local document links pass the repository gate.

## Human review checkpoint

- **Production path:** None; this is a documentation-only architecture unit.
- **Failure/test path:** Review the conflict analysis in `docs/architecture.md` and proposed ADR
  0003.
- **Invariant:** Agent workflow authority remains in Code Assist Harness; inference platform
  authority remains here.
- **Deferred:** FastGate wire protocol, Go module layout, license, and live-provider code.

## Validation

- Run `./scripts/check`.
- Search for statements that present planned components as implemented.
- Reconcile ownership against the actual harness architecture and provider port.

## Documentation impact

Creates the root/component status map, architecture, brief, glossary, ADR index, and decisions.

## Out of scope

- Editing Code Assist Harness.
- Creating a remote repository or publishing a pull request.
- Selecting FastGate's API, Go version, module layout, or license.
- Implementing any service or provider.
