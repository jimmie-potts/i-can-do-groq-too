# 2026-08-01 ICGT-004 toolchain and module decision

- **Story:** [ICGT-004](../icgt-004-select-go-toolchain.md)
- **Decision:** [ADR 0004](../../docs/adr/0004-go-toolchain-and-module-strategy.md)
- **Setup:** [Local development setup](../../docs/setup.md)

## Observation

The current WSL environment is Ubuntu 26.04 on Linux/amd64. Neither `go` nor `gofmt` is installed.
GCC 15.2 is available, so the current platform has the C compiler needed by Go race testing after
Go is installed. The canonical Git remote is
`git@github.com:jimmie-potts/i-can-do-groq-too.git`.

Official documentation checked on 2026-08-01 shows that Go 1.26.5 was released on 2026-07-07 with
security and bug fixes. The Go toolchain documentation confirms that the `go` directive is a
minimum requirement and `GOTOOLCHAIN=local` disables automatic toolchain switching. The module and
workspace documentation confirms that a `go.work` coordinates multiple modules; it is not required
for a multi-component repository.

## Decision and consequence

The future scaffold is one repository-root module named
`github.com/jimmie-potts/i-can-do-groq-too`, with `go 1.26.5`, no `toolchain` directive, no
`go.work`, and no nested modules. ICGT-005 remains standard-library-only, so `go.sum` is not
expected in that unit.

CI will initially use exactly Go 1.26.5. Local contributors may use Go 1.26.5 or newer, but the
offline gate will use only the locally invoked distribution and will reject a compiler older than
the module minimum instead of downloading one. Once created, the root `go.mod` is the
machine-readable CI version source; the workflow must consume it rather than repeat the literal.

## Scope boundary

ICGT-004 installed no software and created no `go.mod`, `go.sum`, `go.work`, Go package,
dependency, executable, endpoint, or service behavior. It records the accepted design and the exact
handoff only.

ICGT-005 owns:

- explicit user-approved toolchain preflight;
- materializing the root `go.mod`;
- validating the exact manifest and rejecting workspaces or nested modules;
- adding CI toolchain provisioning;
- format, vet, ordinary-test, and race-test gate stages; and
- the first FastGate lifecycle behavior.

## Validation evidence

Before documentation changes, `./scripts/check` checked 50 repository files and all eleven policy
tests passed. After the documentation change, the same gate checked 53 repository files and all
eleven policy tests passed. `git diff --check` also passed, and the repository contained no
`go.mod`, `go.sum`, `go.work`, `go.work.sum`, or Go source file.

## Official sources

- [Go release history](https://go.dev/doc/devel/release)
- [Go toolchain selection](https://go.dev/doc/toolchain)
- [Go modules reference](https://go.dev/ref/mod)
- [Go workspace tutorial](https://go.dev/doc/tutorial/workspaces)
- [Go race detector](https://go.dev/doc/articles/race_detector)
- [Go Linux installation](https://go.dev/doc/install?down=)
