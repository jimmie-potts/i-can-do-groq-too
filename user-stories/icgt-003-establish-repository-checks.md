# ICGT-003 - Establish repository-wide checks

- **Status:** Done
- **Milestone:** M0 - Repository and learning foundation
- **Dependencies:** ICGT-002
- **Lesson:** [Repository checks](../docs/lessons/icgt-003-repository-checks.md)
- **Review priority:** Normal

## User story

> As a contributor, I want one offline repository gate from the first commit so that documentation,
> Git hygiene, and later code validation share a single trustworthy entry point.

## Scope

- Add `./scripts/check` as the canonical entry point.
- Check Git whitespace, required foundation files, UTF-8/LF/final-newline hygiene, local Markdown
  links, and forbidden local environment filenames.
- Enforce one-to-one story/lesson IDs, status parity, personal-review metadata, and exactly three
  teach-back questions.
- Add dependency-free regression tests for the checker's important rejection paths.
- Add a minimal GitHub Actions workflow invoking the same gate.
- Run Go formatting and tests automatically after the reviewed root `go.mod` or `go.work` exists,
  while failing clearly when repository Go code exists without the toolchain.
- Keep the foundation gate offline and dependency-free beyond Bash, Git, and Python 3.11+.

## Acceptance criteria

1. `./scripts/check` is executable and passes in the initialized repository.
2. A broken relative Markdown link fails with its file and line.
3. CRLF, trailing whitespace, missing final newline, and invalid UTF-8 fail safely.
4. `.env`, `.env.*`, and `*.env` files are rejected while `.env.example` remains allowed.
5. Required status, architecture, lesson, story, and scoped agent files are checked.
6. Story/lesson IDs and statuses agree, review checkpoints exist, and every lesson has exactly three
   teach-back questions.
7. CI uses a SHA-pinned checkout action with read-only repository permission and no persisted
   credentials.
8. No live provider, credential, package install, or network call occurs in the gate.
9. Go stages are honest: absent before the reviewed root module exists and mandatory afterward.
10. Regression tests cover invalid UTF-8, titled links, repository escape, secret filenames, Git
    ignore behavior, duplicate story/lesson IDs, status mismatch, teach-back count, and required
    CI/test artifacts.

## Human review checkpoint

- **Production path:** Review `scripts/check` stage ordering.
- **Failure/test path:** Review link resolution, repository-escape rejection, and secret-filename
  handling in `scripts/check_repository.py`.
- **Invariant:** The same offline entry point is used locally and in CI.
- **Deferred:** Go lint/race/coverage tooling and dependency policy arrive with Go code.

## Validation

- Run `./scripts/check`.
- Run `python3 -m unittest discover -s tests -p 'test_*.py'`.
- In a temporary copy, seed a broken link, trailing whitespace, and `.env.local`; prove each fails.
- Inspect the workflow for permissions, timeout, action pin, and credential persistence.

## Documentation impact

Documents the current gate and records its actual passing evidence in the linked lesson.

## Out of scope

- Installing Go, Docker, ShellCheck, or a task runner.
- Claiming application tests before application code exists.
- Publishing the repository.
