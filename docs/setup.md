# Local development setup

This guide separates two operations that have different safety boundaries:

- **environment preparation** installs tools and may use the network or administrator privileges;
- **repository validation** runs `./scripts/check` with already prepared tools and remains offline
  and credential-free.

The current documentation foundation needs Git, Bash, and Python 3.11 or newer. Go is not required
by the current gate. A reviewed local Go toolchain becomes an ICGT-005 entry condition before that
story creates the first module and FastGate source. No Go installation was performed by ICGT-004.

## Selected Go setup

[ADR 0004](adr/0004-go-toolchain-and-module-strategy.md) selects:

| Concern | Decision |
| --- | --- |
| Minimum Go version | Go 1.26.5 |
| Initial CI version | Exactly Go 1.26.5 |
| Local contributor version | Go 1.26.5 or newer, used with `GOTOOLCHAIN=local` |
| Future module file | Repository-root `go.mod` |
| Future module path | `github.com/jimmie-potts/i-can-do-groq-too` |
| Workspace | No `go.work` until a reviewed second module exists |
| Initial dependency checksum file | No `go.sum` while only the standard library is used |

The future `go.mod` is intentionally not present yet. ICGT-005 owns creating it after the local
toolchain preflight succeeds.

## What the three Go files mean

### `go.mod`: the module's identity card and dependency requirements

`go.mod` tells Go:

- the module's permanent import prefix;
- the minimum Go version needed to use it;
- which external modules and versions are required; and
- any explicitly reviewed replacement or exclusion rules.

The future root file will begin as:

```go
module github.com/jimmie-potts/i-can-do-groq-too

go 1.26.5
```

That file does not create or publish a GitHub repository. It only defines how Go code inside the
module is named. Standard-library imports such as `context` and `net/http` do not appear in its
dependency list.

### `go.sum`: the external-dependency checksum ledger

When Go resolves external modules, `go.sum` records cryptographic hashes for the module archives
and `go.mod` metadata it has used. Those hashes help detect changed or substituted dependency
content.

`go.sum` is not:

- the authoritative list of direct requirements—that belongs in `go.mod`;
- a copy of dependency source code;
- a guarantee that the dependency is already available offline; or
- an exact lockfile in the same sense as every other package manager.

Do not create or hand-edit an empty `go.sum`. ICGT-005 uses only the standard library, so no
`go.sum` is expected. The first later story that resolves an external module must commit the
generated file and define how clean offline validation obtains the dependency contents.

### `go.work`: a local coordinator for multiple modules

A `go.work` file lists several modules that should be treated as active together. It is useful when
developing changes across two or more independently meaningful modules before publishing one for
the other to consume.

It is not required merely because a repository has several components, packages, commands, or
deployments. A workspace can also make code appear to work locally even when one module cannot
build independently. This repository therefore prohibits a checked-in `go.work` until a reviewed
story justifies a real second module and defines independent-module CI behavior.

## WSL preflight

Run these read-only commands inside the WSL distribution, not in Windows PowerShell:

```bash
uname -s
uname -m
command -v go
command -v gofmt
```

The current project workspace is Linux on `x86_64`/amd64. If `command -v` prints nothing, Go is not
installed on the WSL `PATH`. A Windows Go executable exposed under `/mnt/c` is not the selected WSL
toolchain.

## User-approved Go installation

Installation changes system state and requires explicit approval. When installation is approved:

1. Open the official [Go downloads](https://go.dev/dl/) page and select
   `go1.26.5.linux-amd64.tar.gz` for the current WSL architecture.
2. Download the archive and compare `sha256sum go1.26.5.linux-amd64.tar.gz` with the checksum shown
   on that official page.
3. Check whether `/usr/local/go` already exists. If it does, stop and inspect it. Back up or remove
   the old tree only after confirming that replacement is intended.
4. Follow the official [Linux installation instructions](https://go.dev/doc/install?down=) to
   extract the verified archive into `/usr/local`, creating a fresh `/usr/local/go`. Never extract
   a new archive over an existing Go tree.
5. Add this line once to the WSL user's `~/.profile`, then start a new shell:

   ```bash
   export PATH="/usr/local/go/bin:$PATH"
   ```

These instructions deliberately do not contain an automated deletion command. Replacing an
existing system toolchain should remain an inspectable user decision.

## Verify the prepared environment

After installation, run:

```bash
command -v go
command -v gofmt
GOTOOLCHAIN=local go version
GOTOOLCHAIN=local go env GOOS GOARCH CGO_ENABLED CC
GOTOOLCHAIN=local GOPROXY=off GONOPROXY=none GOSUMDB=off GOWORK=off \
  go test -race -run '^$' sync
```

For the selected archive, the important results are `/usr/local/go/bin/go`,
`/usr/local/go/bin/gofmt`, `go1.26.5`, `linux`, and `amd64`. A newer local Go version is also allowed,
but CI will initially use exactly 1.26.5. The final command asks Go itself to compile the standard
library's `sync` tests with race instrumentation without running them. That is a stronger preflight
than passing `go env CC` to `command -v`, because Go permits the compiler setting to contain both an
executable and command-line options.

Do not use `go env -w` to persist the repository's offline settings globally. The quality gate owns
those settings for its child processes so unrelated Go work is not changed.

## Expected failure behavior

| Symptom | Meaning | Response |
| --- | --- | --- |
| `go: command not found` | Go is absent from the WSL `PATH` | Complete the approved installation or correct `~/.profile`; do not let the gate skip Go checks after `go.mod` exists. |
| `go.mod requires go >= 1.26.5` with `GOTOOLCHAIN=local` | The installed local compiler is too old | Install a reviewed Go 1.26.5 or newer toolchain; do not enable automatic download during the gate. |
| `go test -race` reports no cgo or C compiler | Race-test prerequisites are incomplete | Enable cgo on the supported platform and install/select the required compiler before continuing. |
| Module lookup is disabled by `GOPROXY=off` | A required dependency is missing locally | Treat it as a dependency-preparation/design failure; do not let the gate fetch it. |
| A parent workspace changes local behavior | Go found an ambient `go.work` | The gate uses `GOWORK=off`; test the accepted root module independently. |
| Read-only module mode requests an update | `go.mod` or `go.sum` does not match imports | Resolve and review dependency metadata outside the offline validation run, then commit it. |

The Go environment settings stop Go's toolchain and module download paths; they are not a general
network sandbox for arbitrary test code. Default tests must separately use injected or loopback
resources and never require live providers.

## Run repository validation

Before ICGT-005, the current documentation gate remains:

```bash
./scripts/check
```

After ICGT-005 creates the accepted root module, the same command will also verify the manifest,
format Go files, run vet, run ordinary tests, and run race tests. CI prepares the exact toolchain
first and then calls this same canonical command; the gate itself does not install anything.

## ICGT-005 start condition

ICGT-005 may begin only after:

1. the user explicitly approves any required Go installation;
2. the commands above prove a local Go 1.26.5 or newer toolchain and race prerequisites; and
3. its own code-free implementation plan is reviewed.

Until then, the absence of `go.mod`, `go.sum`, `go.work`, and Go source is intentional.

## Official references

- [Go release history and support policy](https://go.dev/doc/devel/release)
- [Go toolchain selection](https://go.dev/doc/toolchain)
- [Go modules reference](https://go.dev/ref/mod)
- [Go multi-module workspace tutorial](https://go.dev/doc/tutorial/workspaces)
- [Go race detector requirements](https://go.dev/doc/articles/race_detector)
- [Go Linux installation](https://go.dev/doc/install?down=)
