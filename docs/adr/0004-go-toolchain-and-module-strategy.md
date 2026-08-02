# ADR 0004: Use Go 1.26.5 and one root module

- **Status:** Accepted
- **Date:** 2026-08-01
- **Accepted:** 2026-08-01
- **Scope:** Go toolchain policy, module identity, monorepo layout, and offline validation

## Context

This repository plans five infrastructure components, but FastGate is the only near-term Go
implementation target. Separate components, executables, and deployment units do not automatically
need separate Go modules. A Go module is a dependency, import, and versioning boundary rather than
an architecture-diagram boundary.

At ICGT-004 acceptance, the WSL environment had no Go installation and the repository contained no
Go module, source, or dependency. ICGT-004 had to select the future layout before ICGT-005 could
materialize it. The selection also had to preserve the canonical gate's offline rule: repository
validation may use tools and dependencies already available in the environment, but it must not
download a compiler, module, or checksum data.

As of 2026-08-01, Go 1.26.5 is the current stable patch release. It was released on 2026-07-07 and
includes security and bug fixes. The official [Go release history](https://go.dev/doc/devel/release)
also states that a major Go release is supported until two newer major releases exist.

The existing canonical Git remote is `github.com/jimmie-potts/i-can-do-groq-too`, so that identity
can be used without inventing a repository or organization.

## Toolchain policy options

### A. Name only the Go 1.26 language line

The future module could declare `go 1.26`, while contributors and CI use whatever 1.26 patch is
available.

Advantages:

- accepts more local 1.26 installations; and
- requires fewer patch-only documentation updates.

Costs:

- contributors and CI may exercise different compiler, standard-library, and security fixes; and
- reproducing a failure requires discovering the exact patch used by each environment.

### B. Use Go 1.26.5 as the minimum and initial CI version

The future module declares `go 1.26.5`, and CI provisions exactly Go 1.26.5.

Advantages:

- the repository and CI state one reproducible baseline;
- the baseline includes the current supported security fixes; and
- an older local toolchain fails clearly instead of being treated as equivalent.

Costs:

- contributors must install at least that patch; and
- patch upgrades require an intentional repository maintenance change.

### C. Name a separate preferred toolchain and retain automatic switching

The future module could use a `toolchain` directive when its preferred development compiler differs
from its minimum `go` version, while leaving `GOTOOLCHAIN` in its default automatic mode. Even when
the directive is omitted, Go treats the module as having an implicit preference matching the `go`
line. `GOTOOLCHAIN`, not the presence of an explicit directive, controls whether Go may switch or
download.

Advantages:

- separates a language compatibility floor from a preferred development toolchain; and
- can make upgrades convenient in networked development environments.

Costs:

- the default Go behavior may search for or download another toolchain; and
- that hidden environment mutation conflicts with the offline gate.

## Module-layout options

### A. One module at the repository root

All components initially share one dependency and import boundary. Packages, `internal/`
directories, commands, tests, scoped guidance, and architecture rules still separate component
responsibilities.

Advantages:

- one dependency graph and one normal `go test ./...` scope;
- simple cross-component learning and refactoring while interfaces are still being discovered; and
- no speculative module or workspace files for documentation-only components.

Costs:

- component dependencies share one module graph; and
- independent releases would require a later split.

### B. One module per component with a root `go.work`

FastGate, LatencyLab, ModelEndpoint Operator, TenantPlane, and FleetSim could each have a module,
coordinated locally by a workspace.

Advantages:

- dependency and release boundaries are explicit from the beginning; and
- components can eventually support different toolchain or release schedules.

Costs:

- creates boundaries for components that have no source yet;
- requires every module to be tested independently as well as together; and
- workspace resolution can hide whether an individual module builds outside the monorepo.

### C. Put the first module under `gateway/`

Only FastGate could begin as a nested module, leaving the repository root outside any module.

Advantages:

- limits the first dependency graph to the first implemented component; and
- makes a future multi-module layout easier to anticipate.

Costs:

- makes the root quality gate and future shared packages more complicated immediately; and
- optimizes for a split before independent release or dependency pressure exists.

## Decision

Select toolchain option B and module-layout option A.

ICGT-005 will create exactly one module file at the repository root with this content:

```go
module github.com/jimmie-potts/i-can-do-groq-too

go 1.26.5
```

The `go` line is the minimum accepted version and Go 1.26.5 is also the exact version ICGT-005 must
initially provision and test in CI. A contributor may use a newer local Go version;
`GOTOOLCHAIN=local` means "use the compiler already invoked," not "require exact patch equality."
No separate preferred version is needed, so the module will not contain an explicit `toolchain`
directive. Omitting that directive does not disable switching; the gate's `GOTOOLCHAIN=local`
setting does. The official
[toolchain documentation](https://go.dev/doc/toolchain) explains that the `go` line is a minimum
requirement, that a newer requirement is rejected by an older local-only toolchain, and that
`GOTOOLCHAIN=local` always uses the toolchain bundled with the invoked `go` command.

The initial repository will have:

- one root `go.mod` beginning in ICGT-005;
- no `go.work`;
- no nested `go.mod`; and
- no `go.sum` while only the standard library is used.

When a later story first resolves an external module, it must commit the resulting `go.sum` and
decide how a clean offline validation environment receives that dependency. `go.sum` records
cryptographic hashes for resolved external module content and metadata; it is not a substitute for
making those files locally available. The [module reference](https://go.dev/ref/mod) defines the
module download, checksum, and environment behavior.

## Offline gate contract for ICGT-005

ICGT-005 must turn the decision into enforced repository behavior. The gate will:

1. require the exact module inventory to be the root `go.mod`;
2. reject a repository `go.work` or nested `go.mod`;
3. require both `go` and the matching `gofmt` from that local Go distribution;
4. verify that `go env GOMOD` resolves to the root file;
5. require the root `go.mod` content to match the accepted two-directive manifest exactly, rejecting
   every additional directive—including `toolchain`, `godebug`, `tool`, `ignore`, `require`,
   `exclude`, `replace`, and `retract`—and reject `go.sum` during ICGT-005;
6. format-check every Git-tracked or non-ignored Go source file with NUL-safe filename handling;
7. preflight race support by checking the platform, `CGO_ENABLED`, and C compiler, failing clearly
   instead of silently skipping it; and
8. run `go vet ./...`, `go test ./...`, and `go test -race ./...`.

The Go stages will use this environment:

```text
GOTOOLCHAIN=local
GOPROXY=off
GONOPROXY=none
GOSUMDB=off
GOWORK=off
GOFLAGS=-mod=readonly
GOENV=off
```

`GONOPROXY=none` prevents an ambient private-module pattern from bypassing `GOPROXY` with a direct
version-control fetch. `GOWORK=off` prevents a parent-directory workspace from changing the module
under test. Read-only module mode prevents validation from editing dependency metadata. Before any
repository child stage, the gate re-executes with an explicit allowlist that omits ambient credential
variables. It replaces `HOME`, gives Go a fresh temporary `XDG_CONFIG_HOME`, records
`go telemetry off` there, and verifies both the disabled mode and isolated telemetry directory
without reading the user's global Git or Go configuration as authority. This is environment
isolation, not a general filesystem or network sandbox.

CI may download the reviewed Go distribution while preparing the job, before the canonical gate
starts. Once ICGT-005 creates it, the root `go.mod` is the machine-readable version source of truth.
The SHA-pinned CI setup action must read `go-version-file: go.mod` rather than repeat `1.26.5`, and
the job must verify that the provisioned `go env GOVERSION` matches that directive before invoking
the gate. `./scripts/check` itself remains network-free and credential-free. ICGT-005 owns adding
that CI provisioning and testing its failure behavior. No third-party Go linter is selected before
code demonstrates a need for one.

These Go environment settings prevent toolchain, proxy, direct-version-control, and checksum
downloads. They do not technically sandbox arbitrary network calls made by test code. Each code
story must therefore keep default tests on injected or loopback resources and add explicit tests for
that boundary. ICGT-005's manifest check also prevents an ambient module cache from hiding an
unreviewed dependency.

## Installation policy

The supported WSL setup is the official Linux distribution installed inside WSL at
`/usr/local/go`, with `/usr/local/go/bin` on the WSL shell's `PATH`. Installation or replacement is
a user-approved environment action, never a side effect of the repository gate. The official
[Linux installation guide](https://go.dev/doc/install?down=) warns not to extract a new archive over
an existing `/usr/local/go` tree.

ICGT-004 documents the preflight and installation procedure but does not perform it. ICGT-005 may
start only after `command -v go`, `command -v gofmt`, and `GOTOOLCHAIN=local go version` confirm a
local Go 1.26.5 or newer installation, and the race preflight confirms a supported platform with
cgo and a C compiler. CI remains pinned to exactly 1.26.5.

## Reasons to split later

A component may become a separate module only when a reviewed story presents evidence such as:

- an independent version, release, or external consumer;
- a different supported Go version or toolchain cadence;
- measurable build, test, cache, or security-review cost from unrelated dependencies;
- a separate compatibility guarantee owned by another team; or
- package and import rules proving insufficient to prevent harmful coupling.

Separate executables, directories, deployments, or architecture owners are not enough by
themselves. ModelEndpoint Operator is a plausible first candidate if Kubernetes dependencies later
impose measurable cost on FastGate, but this ADR does not preselect that outcome.

If a split becomes justified, its owning story must add or supersede an ADR, add the nested module,
and test every module independently with `GOWORK=off`. A checked-in `go.work` is considered only
after a real second module exists.

## Consequences and sequencing

- Learners begin with one visible dependency boundary and one import prefix.
- All five planned components may remain architecturally independent inside one module.
- ICGT-005 owns creating `go.mod`, source, gate enforcement, and CI toolchain provisioning.
- ICGT-005 remains standard-library-only, so `go.sum` is not expected in that story.
- No Go installation, module, workspace, checksum file, source package, or dependency is created by
  this decision unit.
- A future patch update is a reviewed maintenance change. A change to the language/toolchain line,
  module identity, workspace policy, or split criteria must update or supersede this ADR.
