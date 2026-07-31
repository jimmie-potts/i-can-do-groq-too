# ICGT-004 lesson: Go toolchain and module boundaries

- **Unit:** ICGT-004
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Planned
- **Implementation status:** Planned; no Go toolchain ADR or module exists
- **Story:** [ICGT-004](../../user-stories/icgt-004-select-go-toolchain.md)
- **Review priority:** High
- **Visual companion:** Not required for this decision-only unit
- **Related architecture:** [Roadmap](../roadmap.md) and [FastGate README](../../gateway/README.md)

> This lesson describes questions the next decision unit must resolve. It does not select a current
> Go version or module layout.

## Quick summary

This unit separates four ideas that are easy to conflate: the Go compiler/toolchain, a module's
dependency and import identity, a workspace that coordinates modules, and the Git repository that
stores them.

## Learning objectives

After this unit, you should be able to explain the difference between Go version, module path,
`go.mod`, and `go.work`; compare one root module with component modules; and explain why an offline
quality gate must refuse automatic toolchain/module downloads.

## Why this unit matters

The module choice shapes import paths, component coupling, test commands, release boundaries, and
how the quality gate finds code. Choosing it implicitly in the first generated command would make a
long-lived architecture decision nearly invisible.

## Junior engineer foundation

- The **toolchain** is the local compiler and standard tooling.
- A **module** is a versioned import/dependency boundary declared by `go.mod`.
- A **module path** is the import identity; it often resembles a repository URL but does not create
  that remote.
- A **workspace** (`go.work`) lets local development use several modules together.

A common misconception is that a monorepo requires multiple modules. One module can contain many
packages and commands. Multiple modules add release and dependency boundaries, but also more
coordination.

## Key concepts

- **Minimum version:** oldest Go language/toolchain behavior the repository supports.
- **Pinned toolchain policy:** how contributors and CI select a known version.
- **Offline validation:** use already installed/resolved tools and dependencies only.
- **Module boundary:** the unit of import identity and dependency resolution.

## Architecture and invariants

The decision must cover all five planned components without creating their source trees early. The
canonical gate must use `GOTOOLCHAIN=local`, `GOPROXY=off`, and `GOSUMDB=off` so a missing toolchain or
module cache fails rather than mutating the environment.

## Practical walkthrough

1. Verify current supported Go releases from official sources.
2. Compare one root module with root `go.work` plus per-component modules.
3. Predict imports, `go test` commands, and releases for each option.
4. Decide the initial module identity independently from GitHub remote creation.
5. Record installation as a user-approved environment step, not a quality-gate side effect.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| Planned toolchain/module ADR | Locks a long-lived boundary visibly | What future change would force a second module? |
| [`scripts/check`](../../scripts/check) | Owns offline tool invocation | Can it download a toolchain or dependency? |

## Implementation code samples

None. This is a planned decision unit and must not fabricate `go.mod` contents.

## Failure scenarios to study

- `go.mod` requests a newer toolchain and `go` attempts an automatic download.
- A nested module is added but the root gate never tests it.
- The module path claims a remote/organization the user did not choose.
- Five component modules create versioning overhead before any component has code.

## What changed during implementation

No implementation evidence exists yet. The bootstrap observed that Go is unavailable locally, so
this decision precedes service code and any installation requires explicit user approval.

## Production expansion

Large Go monorepos may add module proxies, generated dependency graphs, build systems, hermetic
toolchains, and multi-platform release pipelines. Graduate when build time, independent versioning,
or team ownership produces measured pressure—not merely because the roadmap names five components.

## Practical exercises

- Draw imports for FastGate and LatencyLab under one-module and two-module layouts.
- Predict what `GOTOOLCHAIN=local` does when `go.mod` requests an unavailable newer version.
- Explain why choosing a module path does not create a GitHub repository.

## Key takeaways

- Toolchain, module, workspace, and Git repository are separate decisions.
- Offline checks must fail rather than download missing tools or dependencies.
- Start with the fewest module boundaries justified by current code.

## Glossary

- **Module:** Go dependency/import versioning boundary.
- **Workspace:** local coordination of multiple Go modules.
- **Toolchain:** compiler and standard Go commands.

## Teach-back questions

1. What is the difference between a module path and a Git remote?
2. How can automatic toolchain selection violate an offline quality gate?
3. What measured signal would justify splitting one module into component modules?

## Further reading

- [ICGT-004 delivery contract](../../user-stories/icgt-004-select-go-toolchain.md)
- [Go modules reference](https://go.dev/ref/mod)
- [Go workspaces tutorial](https://go.dev/doc/tutorial/workspaces)
