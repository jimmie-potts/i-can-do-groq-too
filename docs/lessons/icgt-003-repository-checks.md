# ICGT-003 lesson: A truthful offline repository gate

- **Unit:** ICGT-003
- **Milestone:** M0 - Repository and learning foundation
- **Lesson status:** Verified against implementation
- **Implementation status:** Done; the local/CI gate and seeded negative paths are validated
- **Story:** [ICGT-003](../../user-stories/icgt-003-establish-repository-checks.md)
- **Review priority:** Normal
- **Visual companion:** Not required
- **Related architecture:** [Root agent guidelines](../../AGENTS.md)

> This lesson describes the current foundation checks. Go stages remain conditional until a Go
> module exists.

## Quick summary

ICGT-003 gives local development and CI one offline entry point. It checks what the repository
actually contains today and is designed to grow when Go code arrives.

## Learning objectives

After this unit, you should be able to:

- explain why a canonical gate should not install or update dependencies;
- trace stage failure through an actionable diagnostic;
- distinguish a current check from a future conditional stage; and
- explain how local link resolution avoids both network access and repository escape.

## Why this unit matters

Documentation is executable project state: broken links, stale status, or secret-bearing files can
mislead every later story. A single command also prevents CI from validating a different workflow
than developers run.

## Junior engineer foundation

### Exit status is the shell contract

A program returning zero means success; a non-zero status means failure. `set -euo pipefail` makes a
Bash script stop on failed commands, unset variables, and failed pipeline stages rather than hiding
them.

### Offline does not mean weak

The gate can validate encoding, line endings, links, required files, formatting, and unit tests with
no provider call. A common misconception is that “integration” always requires the internet. Most
integration boundaries can first be exercised with deterministic fakes.

## Key concepts

- **Canonical gate:** the one command required before delivery.
- **Text hygiene:** UTF-8, LF endings, final newline, and no trailing whitespace.
- **Repository escape:** a relative link that resolves outside the repository boundary.
- **Conditional stage:** a check that becomes mandatory when its owning artifact exists.

## Architecture and invariants

```text
./scripts/check
  -> remove provider credentials and force offline Go behavior
  -> working, staged, and committed Git whitespace
  -> dependency-free repository/link/learning policy
  -> repository-checker regression tests
  -> Go format and tests only when reviewed root go.mod/go.work exists
```

CI invokes the same script. It does not duplicate validation logic in workflow YAML.

## Practical walkthrough

1. Run `./scripts/check` from any directory context through its resolved repository root.
2. Follow each labeled stage in `scripts/check`.
3. Review how `check_repository.py` excludes tool-owned directories without reading secrets.
4. Trace one relative Markdown target from its source file to a resolved repository path.
5. Seed a failure in a temporary copy and read the file/line diagnostic.

## Personal code review map

| Review path | Why it matters | Question to answer |
| --- | --- | --- |
| [`scripts/check`](../../scripts/check) | Owns stage order and conditional Go behavior | When does Go become mandatory? |
| [`scripts/check_repository.py`](../../scripts/check_repository.py) | Owns text, file, link, and learning-metadata policy | How are repository escape and story/lesson drift rejected? |
| [`tests/test_check_repository.py`](../../tests/test_check_repository.py) | Locks important rejection paths | Can malformed input fail without a traceback or content leak? |
| [CI workflow](../../.github/workflows/check.yml) | Proves local/remote parity and least privilege | Does CI invoke the same gate without persisted credentials? |

## Implementation code samples

### Sample 1: one stage owner grows with repository artifacts

From [`scripts/check`](../../scripts/check):

```bash
export GOPROXY=off
export GOSUMDB=off
export GOTOOLCHAIN=local

run_stage "Git whitespace" check_git_whitespace
run_stage "Repository policy and Markdown links" python3 scripts/check_repository.py
run_stage "Repository policy tests" python3 -m unittest discover -s tests -p 'test_*.py'

if [[ -f go.mod || -f go.work ]]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Go modules exist, but the Go toolchain is unavailable." >&2
    exit 1
  fi
  run_stage "Go formatting" check_go_formatting
  run_stage "Go tests" go test ./...
fi
```

The current stages run without project dependencies. The explicit root `go.mod`/`go.work` signal
avoids activating Go because an ignored dependency happens to contain a module. Missing Go then
fails explicitly rather than letting code validation disappear. ICGT-004 will lock the root module
or workspace layout before source exists; ICGT-005 will refine the Go stages against that choice.

### Sample 2: Git owns the content inventory; secret names are checked separately

From [`scripts/check_repository.py`](../../scripts/check_repository.py):

```python
result = subprocess.run(
    ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
    cwd=ROOT,
    check=False,
    stdout=subprocess.PIPE,
    stderr=subprocess.DEVNULL,
)

for directory, directory_names, file_names in os.walk(ROOT):
    directory_names[:] = [name for name in directory_names if name not in IGNORED_PARTS]
    base = Path(directory)
    matches.extend(base / name for name in file_names if is_environment_file(base / name))
```

Git's own ignore rules decide which tracked or untracked files receive content checks, so ignored
editor and dependency artifacts cannot fail repository policy accidentally. Environment filenames
need a separate name-only walk because `.gitignore` intentionally hides them; that walk prunes
tool/dependency trees and never opens the recognized secret-bearing file.

### Sample 3: a local link must remain inside the repository

From [`scripts/check_repository.py`](../../scripts/check_repository.py):

```python
destination = (path.parent / file_target).resolve()
try:
    destination.relative_to(ROOT)
except ValueError:
    errors.append(f"link escapes repository: {relative_path}:{line_number}: {target}")
    continue
if not destination.exists():
    errors.append(f"broken local link: {relative_path}:{line_number}: {target}")
```

`resolve()` normalizes `..` and symlinks. `relative_to(ROOT)` succeeds only when the resulting path
is still under the repository. That check happens before existence, so an existing file outside the
repository is not accepted as a portable documentation target.

## Failure scenarios to study

- A local link points outside the repository: reject it even if the target exists.
- A future `go.mod` is committed but Go is unavailable: fail clearly instead of skipping code tests.
- `.env.local` or another `*.env` appears: reject the filename without reading or printing its
  value. All recognized environment-file contents, including an allowed `.env.example`, are skipped
  by this gate.
- A Markdown code block contains example link syntax: ignore it to prevent false positives.

## What changed during implementation

The environment had no Go, Docker, or ShellCheck. Rather than install or claim them, the foundation
uses existing Bash, Git, and Python and makes Go checks conditional on the first Go module.

Validation on 2026-07-31 produced:

- `./scripts/check`: passed after checking 50 repository files;
- `python3 -m unittest discover -s tests -p 'test_*.py'`: eleven tests passed;
- invalid UTF-8 failed safely rather than re-entering the Markdown parser;
- titled CommonMark links were accepted while escaping links were rejected;
- `.env.local` was rejected by filename without printing its sentinel contents;
- ignored dependency and editor files did not enter the content scan;
- duplicate story/lesson IDs, status drift, and teach-back-count drift were rejected; and
- removal of the CI workflow or checker regression suite was rejected as a missing foundation file.

Initial disposable-copy probes also rejected a broken link and trailing whitespace. Those probes
exposed that `scripts/check` correctly expects an initialized Git repository; the probe was fixed by
initializing its temporary copy rather than weakening the production gate.

## Production expansion

Future Go stories may add `go vet`, a reviewed linter, race tests, coverage thresholds, dependency
licenses, vulnerability scanning, container checks, and contract compatibility tests. Each tool
adds versioning and maintenance cost, so it should arrive with the behavior it protects.

## Practical exercises

- Seed a broken local link in a temporary copy and predict the diagnostic.
- Add a Markdown example inside a fenced block and confirm it is ignored.
- Explain why `go test` must not be silently skipped after `go.mod` exists.

## Key takeaways

- Local and CI validation share one offline entry point.
- Checks describe current artifacts honestly and become mandatory with their owning story.
- Secret-file policy rejects names without exposing contents.

## Glossary

- **Gate:** a command whose success is required for delivery.
- **LF:** Unix newline byte `\n`.
- **SHA pin:** an immutable action revision rather than a moving tag.

## Teach-back questions

1. Why should CI call `./scripts/check` instead of reimplementing each stage in YAML?
2. How does the link checker prevent a relative target from escaping the repository?
3. What signal should trigger adding a new quality tool rather than installing it at bootstrap?

## Further reading

- [ICGT-003 delivery contract](../../user-stories/icgt-003-establish-repository-checks.md)
- [GitHub Actions security hardening](https://docs.github.com/en/actions/security-for-github-actions/security-guides/security-hardening-for-github-actions)
