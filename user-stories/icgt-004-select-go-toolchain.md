# ICGT-004 - Select the Go toolchain and module strategy

- **Status:** Done
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Dependencies:** ICGT-003
- **Lesson:** [Go toolchain and modules](../docs/lessons/icgt-004-go-toolchain-and-modules.md)
- **Decision:** [ADR 0004](../docs/adr/0004-go-toolchain-and-module-strategy.md)
- **Review priority:** High

## User story

> As a learner, I want the Go version, module identity, monorepo layout, and installation preflight to
> be explicit before source code exists so that reproducibility and package boundaries are decisions
> I can review rather than tool-generated accidents.

## Scope

- Verify current supported Go releases from official sources at implementation time.
- Choose and record a minimum/pinned Go version policy.
- Decide one root module versus `go.work` with component modules.
- Decide the initial module path without inventing a GitHub remote that does not exist.
- Document WSL installation and version verification without silently modifying the user's system.
- Define how `./scripts/check` discovers the chosen module layout and prevents toolchain or module
  downloads during its offline run.
- Name the exact `go.mod`/`go.work` files, module paths, and gate commands that ICGT-005 must
  materialize; this unit records the handoff but creates none of those files.
- Accept an ADR before creating `go.mod` or Go source.

## Acceptance criteria

1. A focused ADR records alternatives, selected Go version policy, exact module file layout, module
   identities, gate-discovery commands, and consequences for later components.
2. Official current Go documentation supports any time-sensitive version claim.
3. Setup instructions distinguish installing Go from repository validation and require explicit
   user approval for system changes.
4. The canonical gate's future Go discovery path matches the accepted layout and keeps
   `GOTOOLCHAIN=local`, `GOPROXY=off`, and `GOSUMDB=off` during validation.
5. No GitHub remote, repository visibility, or license is guessed.
6. No Go module, dependency, or service code is added by this decision unit.
7. The lesson explains module path, module, workspace, toolchain selection, and offline validation in
   plain language.

## Human review checkpoint

- **Production path:** None; review the accepted toolchain/module ADR and selected future repository
  paths.
- **Failure/test path:** Trace how a missing/mismatched local Go version or requested toolchain
  download fails before source validation is skipped.
- **Invariant:** Repository checks use the reviewed local toolchain and never download a replacement.
- **Deferred:** ICGT-005 materializes the selected root `go.mod`, gate discovery, service source,
  and health endpoint; dependencies, additional modules, workspaces, and provider contracts remain
  later work.

## Validation

- Check every version claim against official Go documentation.
- Review the ADR against the five-component roadmap.
- Run `./scripts/check` before and after documentation changes.

## Documentation impact

Added the Go toolchain/module ADR and updated setup, FastGate, roadmap, and lesson documentation.

## Out of scope

- Installing Go without explicit user approval.
- Creating a GitHub repository or remote.
- Creating `go.mod`, `go.work`, source packages, or dependencies.
- Defining the exact schema and fixtures for the selected FastGate client protocol.
