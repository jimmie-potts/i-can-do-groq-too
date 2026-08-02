# 2026-07-31 bootstrap decisions

- **Stories:** ICGT-001, ICGT-002, ICGT-003
- **Source context:** [Project brief](../../docs/project-brief.md)
- **Architecture:** [ADR 0001](../../docs/adr/0001-separate-learning-monorepo.md),
  [ADR 0002](../../docs/adr/0002-fake-first-openai-first-live.md), and
  [ADR 0003](../../docs/adr/0003-fastgate-api-surface.md) (proposed at bootstrap)

## Observed starting state

The checkout contained only empty app-managed `.agents/` and `.codex/` directories. It was not a
Git repository and had no files, history, branch, remote, manifest, or license.

The GitHub account `jimmie-potts` is authenticated. Global Git identity is
`Jimmie Potts <jimmie.potts@gmail.com>`, the configured default branch is `main`, and the preferred
GitHub operation protocol is SSH. No GitHub repository named `jimmie-potts/i-can-do-groq-too`
existed at bootstrap, so no remote was guessed during foundation work. Publication later created
that repository explicitly as public after user approval.

The first SSH push exposed an invalid WSL system configuration include owned by `nobody:nogroup`.
Direct authentication with `ssh -F /dev/null` succeeded, so this checkout records that command in
the repository-local `core.sshCommand` setting rather than changing system ownership or global Git
configuration. The exact failure and workaround are documented in
[Troubleshooting](../../docs/troubleshooting.md).

The local WSL environment has Node 22.22.1 and Python 3.14.4. Go, Docker integration, and ShellCheck
were unavailable. The foundation therefore uses dependency-free Bash/Python checks and defers Go
toolchain and module choices to ICGT-004. Docker is not a foundation prerequisite.

## Code Assist Harness reconciliation

The current harness already implements provider-neutral requests and streamed observations, a lazy
single-consumer operation port, explicit awaited cancellation/cleanup, and a strict deterministic
fake. Its provider-backed turn, hard limits, and direct OpenAI Responses adapter remain planned.

No harness code or active M1 sequence needs to change for these projects. The important future
constraint is that FastGate receives a separate harness adapter. The direct OpenAI adapter is not a
generic OpenAI-compatible client: its official endpoint, exact model allowlist, Responses stream
automaton, disabled retries, and exclusions remain intact.

The harness MVP currently excludes Git-state mutation and remote worker deployment. A future Git or
worktree feature would remain a harness concern if separately approved; it is not current product
behavior. ModelEndpoint Operator must not claim to deploy harness workers without a new harness ADR.

## Resolved decisions

- Keep the five infrastructure components in this separate learning monorepo.
- Keep current repository name; use “Inference Cloud Lab / Agent Infrastructure Kit” as a subtitle,
  not a rename.
- Build FastGate first, followed by LatencyLab, operator, TenantPlane, and FleetSim.
- Use Go for infrastructure cores and TypeScript only for a justified visualization.
- Implement a deterministic fake first and OpenAI as the first live provider.
- Keep LangChain and LangGraph out of infrastructure cores.
- Keep raw KV-cache management out of hosted-provider scope.
- Use one-to-one story and lesson companions with personal review checkpoints.

## Open decisions at bootstrap

- FastGate's first external API and stream grammar.
- Go version and root-versus-multi-module strategy.
- License selection.
- Task runner beyond `./scripts/check`.
- Database migration and optional dashboard choices.

## 2026-08-01 protocol resolution

ADR 0003 is now accepted with Option C: a small, versioned, FastGate-owned model-turn protocol. This
selects the architectural direction only. ICGT-006 remains Planned and owns the exact non-streaming
v1 schema, bounds, mapping, valid/invalid fixtures, and offline validator. Chat Completions and
Responses compatibility facades remain outside v1 and require separate reviewed client evidence.

## 2026-08-01 Go toolchain and module resolution

[ADR 0004](../../docs/adr/0004-go-toolchain-and-module-strategy.md) resolves the bootstrap's Go
decision with Go 1.26.5 as the minimum and initial exact CI version, one future repository-root
module named `github.com/jimmie-potts/i-can-do-groq-too`, and no workspace or nested modules. Local
Go may be newer, but the offline gate will use only the invoked local distribution. ICGT-005 owns
creating the module and code after explicit toolchain approval and verification.

## Validation evidence

On 2026-07-31, the final `./scripts/check` pass checked 50 repository files and its eleven durable
checker regression tests passed. Those tests cover invalid UTF-8, titled and escaping links,
secret-filename handling without content disclosure, Git ignore behavior, duplicate lesson/story
IDs, status parity, teach-back count, and required CI/test artifacts. Earlier disposable initialized
Git copies also proved broken-link and trailing-whitespace failures. ICGT-001 through ICGT-003 were
marked Done together, and their lessons were changed to Verified against implementation.
