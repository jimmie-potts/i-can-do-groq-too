# ICGT-004 lesson: Go toolchain and module boundaries

- **Unit:** ICGT-004
- **Milestone:** M1 - FastGate non-streaming walking skeleton
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; ADR 0004 and setup/handoff documentation select Go 1.26.5,
  one root module, and `github.com/jimmie-potts/i-can-do-groq-too`; no Go module, source,
  dependency, or service exists
- **Story:** [ICGT-004](../../user-stories/icgt-004-select-go-toolchain.md)
- **Review priority:** High
- **Visual companion:** Not required for this documentation-only decision unit
- **Related architecture:** [ADR 0004](../adr/0004-go-toolchain-and-module-strategy.md),
  [Local setup](../setup.md), [Roadmap](../roadmap.md), and
  [FastGate README](../../gateway/README.md)

> This lesson describes an accepted and validated documentation decision. It does not claim that Go
> is installed or that `go.mod`, `go.sum`, `go.work`, Go packages, dependencies, or a service exist.

## Quick summary

The repository will start with Go 1.26.5 and one root module. That keeps the first dependency and
import boundary simple while FastGate is the only implementation target. A checked-in `go.work`
would add coordination for multiple modules that do not exist, so it is prohibited until a real
second module is justified.

ICGT-004 records that decision. ICGT-005 will create the files and first service behavior only after
the local Go toolchain is explicitly approved and verified.

## Learning objectives

After this unit, you should be able to:

- distinguish a Git repository, Go module, package, and executable command;
- explain the different jobs of `go.mod`, `go.sum`, and `go.work`;
- explain why the `go` directive is a minimum rather than an exact local compiler lock;
- predict how `GOTOOLCHAIN=local` changes a too-old-toolchain failure; and
- name evidence that would justify splitting a component into another module.

## Why this unit matters

Running `go mod init` looks like a small setup command, but it writes a long-lived import identity
and dependency boundary. Generating five modules because the roadmap names five components would
make that architecture decision nearly invisible and would add versioning and test coordination
before the project has source.

The opposite mistake is allowing Go to repair the environment automatically. A convenient
toolchain or dependency download can make a local check pass without proving that the repository is
reproducible in a prepared offline environment.

## Junior engineer foundation

### Repository, module, package, and command are different layers

- The **Git repository** stores and versions all five component areas.
- The **Go module** is the dependency, import, and versioning boundary declared by `go.mod`.
- A **package** is a directory of related Go source files compiled together inside a module.
- A **command** is an executable program whose package is named `main`.

One repository can hold one module with many packages and commands. Separate services and
deployments do not require separate modules.

### `go.mod` is the module's identity card

It records the module path, minimum Go version, external module requirements, and special
resolution rules. The future module path is an import prefix; writing it does not create a GitHub
repository.

The future ICGT-005 file is specified as:

```go
module github.com/jimmie-potts/i-can-do-groq-too

go 1.26.5
```

This is a handoff specification, not a file that exists today. Standard-library packages do not
become `require` entries.

### `go.sum` is a checksum ledger

When external modules are resolved, Go records hashes for module archives and module metadata in
`go.sum`. Those hashes help prove that the downloaded bytes still match previously reviewed bytes.

`go.sum` does not list only direct dependencies, contain the dependency source, or make a clean
machine offline by itself. The dependency contents must still be available from a prepared module
cache, vendor directory, or another reviewed mechanism. Because ICGT-005 uses only the standard
library, no `go.sum` is expected initially.

### `go.work` coordinates multiple modules

A workspace lists multiple local modules that Go should treat as active together. This helps when
editing two independently versioned modules before publishing one of them.

That convenience can hide a failure: code may compile inside the workspace while an individual
module cannot compile on its own. This project has only one selected module, so a workspace has no
job yet. The future gate uses `GOWORK=off` and rejects a repository `go.work`.

### Minimum version is not the same as an exact local pin

The future `go 1.26.5` line means that older Go versions must refuse the module. It does not reject
a newer installed compiler. CI will initially use exactly 1.26.5 for a reproducible baseline, while
contributors may use 1.26.5 or newer.

With the normal automatic setting, Go may locate or download a newer compiler when a module asks
for one. `GOTOOLCHAIN=local` changes that behavior: use the compiler already invoked and fail if it
is too old.

An explicit `toolchain` directive is only needed to name a preferred compiler that differs from the
minimum. When it is absent, Go treats the module as having an implicit preference matching the `go`
line. Therefore omitting the directive does not create the offline guarantee;
`GOTOOLCHAIN=local` does.

## Decision and alternatives

ADR 0004 selects:

- exact Go 1.26.5 as the minimum and initial CI baseline instead of a drifting 1.26 patch;
- no separate preferred version in an explicit `toolchain` directive;
- `GOTOOLCHAIN=local` in the gate to disable automatic compiler switching independently of that
  directive;
- one root module instead of component modules plus `go.work`; and
- the existing GitHub repository identity as the module path.

Multiple modules remain a valid later design when there is evidence. Current component names alone
are not evidence.

## Offline gate handoff

ICGT-005 must make the future Go stage fail closed:

```text
accepted module files: ./go.mod only
accepted go.mod content: module github.com/jimmie-potts/i-can-do-groq-too
                         go 1.26.5
rejected initially: any other go.mod content, go.work, nested go.mod, go.sum

GOTOOLCHAIN=local
GOPROXY=off
GONOPROXY=none
GOSUMDB=off
GOWORK=off
GOFLAGS=-mod=readonly
```

It then format-checks Git-tracked and non-ignored Go files, runs `go vet ./...`, `go test ./...`,
and `go test -race ./...`. Race validation must fail clearly when the platform, cgo, or C compiler
preflight is unavailable; it must not silently skip the important concurrency check.

These controls stop Go's own download paths. They do not form a general network sandbox, so each
story must also keep default tests on injected or loopback resources.

## Practical walkthrough

1. Read [ADR 0004](../adr/0004-go-toolchain-and-module-strategy.md) and locate the selected option,
   rejected costs, future manifest, gate contract, and split triggers.
2. Read [Local setup](../setup.md) and separate the commands that inspect the environment from the
   user-approved steps that change `/usr/local/go`.
3. Pretend local Go is 1.26.4. Follow `GOTOOLCHAIN=local` to the intended failure rather than an
   automatic download.
4. Pretend ModelEndpoint Operator later needs independently released Kubernetes dependencies. Use
   the ADR's split criteria to decide whether that is measured evidence for another module.
5. Confirm that no module or source file exists yet; the exact scaffold belongs to ICGT-005.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [ADR 0004](../adr/0004-go-toolchain-and-module-strategy.md) | Owns the accepted version, identity, layout, and future split rule | What evidence—not directory names—would justify a second module? |
| [Local setup](../setup.md) | Separates user-approved installation from offline validation | Which steps can change the system, and which are read-only? |
| [ICGT-004 story](../../user-stories/icgt-004-select-go-toolchain.md) | Keeps the decision unit within its reviewed scope | Which files and behaviors are deliberately left to ICGT-005? |
| [`scripts/check`](../../scripts/check) | Shows the current conditional Go hook that ICGT-005 must replace | Why must Go validation fail rather than disappear once `go.mod` exists? |

## Implementation code samples

None. This documentation-only unit created no `go.mod`, `go.work`, Go source, dependency, or
executable. Its implemented artifacts are ADR 0004, setup and handoff documentation, and the
updated repository status projections.

## Failure scenarios to study

### The installed compiler is too old

The future `go.mod` says `go 1.26.5`, but the invoked compiler is older. With
`GOTOOLCHAIN=local`, Go exits instead of downloading a replacement. The actionable response is to
prepare a supported compiler outside the gate, then rerun the same check.

### A nested module escapes root tests

`go test ./...` from the root does not prove that an accidental nested module was tested. ICGT-005
must inspect the repository file inventory before package tests and reject any nested `go.mod`.

### A workspace hides an independent build failure

An ambient or checked-in `go.work` can make one module resolve another local module without its
published dependency metadata. `GOWORK=off` ensures the accepted root module is tested by itself.

### An external dependency appears during an offline run

`GOPROXY=off` and `GONOPROXY=none` prevent proxy and direct version-control fetches. In ICGT-005,
the stricter manifest rule rejects any dependency declaration. A later dependency story must
prepare the content and checksums explicitly before the offline gate begins.

### Race validation cannot start

The race detector needs a supported platform, cgo, and a C compiler. Treat a missing prerequisite
as an environment failure with setup guidance, not permission to skip the test.

## What changed during implementation

Official release evidence selected Go 1.26.5 rather than an unverified or floating version. The
existing GitHub origin justified the canonical module identity. Comparing the five-component
roadmap found no independent release, consumer, toolchain, or measured dependency pressure that
would justify multiple modules today.

Review also clarified that the `go` directive is a minimum: exact reproducibility belongs to the CI
toolchain selection, while a newer local compiler remains acceptable. The offline environment was
expanded to guard direct private-module fetches and ambient workspaces, and the race-test handoff
now names its platform and compiler preflight.

No Go installation occurred. That preserves the agreed boundary between a documented environment
change and repository validation.

## Validation evidence

- The Go version and support claim was checked against the official release history on 2026-08-01.
- Toolchain, module, workspace, race-detector, and Linux installation behavior was checked against
  official Go documentation.
- The pre-change `./scripts/check` run checked 50 repository files; all eleven policy tests passed.
- The post-change `./scripts/check` run checked 53 repository files; all eleven policy tests passed.

## Production expansion

### Example production scenario

Suppose ModelEndpoint Operator gains its own release cadence and external consumers while its
Kubernetes dependencies materially slow FastGate security review and CI. That is evidence for an
operator module with an independent compatibility promise. A root `go.work` could then help local
cross-module development, while CI would still test each module separately with `GOWORK=off`.

### Representative capabilities and tools

These are examples to evaluate later, not dependencies selected by ICGT-004:

- [Go workspaces](https://go.dev/doc/tutorial/workspaces) coordinate changes across multiple local
  modules.
- [Go vendoring](https://go.dev/ref/mod#vendoring) can make reviewed dependency source available
  without a module download during a build.
- [Go module proxies](https://go.dev/ref/mod#goproxy-protocol) can centralize dependency acquisition
  and policy outside an offline gate.
- [GitHub `setup-go`](https://github.com/actions/setup-go) can provision CI from the version in
  `go.mod` instead of maintaining a second version literal.

### Local versus production

| Dimension | This repository | Production expansion |
| --- | --- | --- |
| Module scope | One root module and dependency graph | Independently versioned modules justified by release or dependency evidence |
| Development | No workspace | Optional `go.work` for coordinated local changes across real modules |
| Validation | One offline root gate | Per-module CI with `GOWORK=off`, plus an optional integrated workspace check |
| Dependencies | Standard library only in ICGT-005 | Reviewed proxy, prepared cache, or vendoring policy |
| Toolchain | `go.mod` baseline and exact initial CI version | Deliberate compatibility matrix or hermetic toolchain when measured needs justify it |

### Trade-offs and graduation signals

Multiple modules can isolate dependency and release risk, but they add compatibility promises,
version updates, independent CI, and cross-module testing. Graduate only after an external consumer,
independent release, incompatible toolchain cadence, materially isolated dependency graph, or
measured CI/build pain makes that cost worthwhile. ModelEndpoint Operator is a plausible first
candidate, but the project should measure the pressure rather than prebuild the boundary.

## Practical exercises

- Draw two packages and two executable commands inside one module; identify which boundaries affect
  imports, dependencies, processes, and deployments.
- Explain what information belongs in `go.mod` versus `go.sum`.
- Predict the difference between running a too-old compiler with `GOTOOLCHAIN=auto` and
  `GOTOOLCHAIN=local`.
- List the evidence you would require before adding a second module and `go.work`.

## Key takeaways

- A repository, module, package, command, and deployment are different boundaries.
- `go.mod` declares identity and requirements; `go.sum` records external-module hashes; `go.work`
  coordinates multiple local modules.
- Go 1.26.5 is the minimum and exact initial CI baseline, while newer local Go versions are allowed.
- Offline checks fail on missing preparation rather than downloading or editing their way to green.
- One root module is the simplest design supported by current evidence; later splits remain
  possible through another reviewed decision.

## Glossary

- **Module:** Versioned dependency and import boundary declared by `go.mod`.
- **Module path:** Permanent import prefix that identifies a module.
- **Package:** Directory of related Go files compiled together.
- **Command:** Executable Go package named `main`.
- **Toolchain:** The `go` command, compiler, standard library, and related tools in a Go
  distribution.
- **Workspace:** Local coordination of multiple modules through `go.work`.
- **Checksum ledger:** Hash records in `go.sum` used to authenticate resolved external module
  content and metadata.

## Teach-back questions

1. Why can five independently deployed components still begin inside one Go module?
2. What does each of `go.mod`, `go.sum`, and `go.work` contribute, and what does it not contribute?
3. What exactly happens when a local Go version is older than `go 1.26.5` and the gate sets
   `GOTOOLCHAIN=local`?

## Further reading

- [ICGT-004 delivery contract](../../user-stories/icgt-004-select-go-toolchain.md)
- [Go release history](https://go.dev/doc/devel/release)
- [Go toolchain selection](https://go.dev/doc/toolchain)
- [Go modules reference](https://go.dev/ref/mod)
- [Go workspaces tutorial](https://go.dev/doc/tutorial/workspaces)
- [Go race detector](https://go.dev/doc/articles/race_detector)
