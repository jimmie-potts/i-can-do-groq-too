#!/usr/bin/env python3
"""Run dependency-free repository hygiene and local Markdown-link checks."""

from __future__ import annotations

import os
import re
import subprocess
import sys
import textwrap
from pathlib import Path
from typing import NamedTuple
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parent.parent
IGNORED_PARTS = {
    ".git",
    ".agents",
    ".codex",
    ".mypy_cache",
    ".pytest_cache",
    ".ruff_cache",
    ".venv",
    "__pycache__",
    "bin",
    "build",
    "coverage",
    "dist",
    "node_modules",
    "tmp",
}
TEXT_SUFFIXES = {
    "",
    ".go",
    ".json",
    ".md",
    ".py",
    ".sh",
    ".toml",
    ".txt",
    ".yaml",
    ".yml",
}
REQUIRED_PATHS = (
    ".editorconfig",
    ".gitattributes",
    ".gitignore",
    ".github/workflows/check.yml",
    "README.md",
    "AGENTS.md",
    "scripts/check",
    "scripts/check_contract.py",
    "scripts/check_repository.py",
    "tests/test_check_contract.py",
    "tests/test_check_repository.py",
    "control-plane/README.md",
    "fleet-simulator/README.md",
    "latency-lab/README.md",
    "model-operator/README.md",
    "docs/project-brief.md",
    "docs/architecture.md",
    "docs/roadmap.md",
    "docs/troubleshooting.md",
    "docs/adr/README.md",
    "docs/lessons/AGENTS.md",
    "docs/lessons/README.md",
    "user-stories/AGENTS.md",
    "user-stories/README.md",
    "gateway/README.md",
    "gateway/AGENTS.md",
    "gateway/contracts/model-turn/v1/README.md",
    "gateway/contracts/model-turn/v1/harness-mapping.md",
    "gateway/contracts/model-turn/v1/schema/request.schema.json",
    "gateway/contracts/model-turn/v1/schema/success.schema.json",
    "gateway/contracts/model-turn/v1/schema/failure.schema.json",
    "gateway/contracts/model-turn/v1/fixtures/cases.json",
    "gateway/cmd/fastgate/main.go",
    "gateway/internal/service/service.go",
    "gateway/internal/service/service_test.go",
)
MARKDOWN_LINK = re.compile(r"!?\[[^\]]*\]\(([^)]+)\)")
STORY_FILENAME = re.compile(r"^(icgt-\d{3})-.+\.md$")
LESSON_FILENAME = re.compile(r"^(icgt-\d{3})-.+\.md$")
STORY_STATUS = re.compile(r"^- \*\*Status:\*\* (.+)$", re.MULTILINE)
LESSON_STATUS = re.compile(r"^- \*\*Lesson status:\*\* (.+)$", re.MULTILINE)
LESSON_UNIT = re.compile(r"^- \*\*Unit:\*\* (ICGT-\d{3})$", re.MULTILINE)
EXPECTED_LESSON_STATUS = {
    "Planned": "Planned",
    "In progress": "Implementation companion",
    "Blocked": "Implementation companion - blocked",
    "Done": "Verified against implementation",
}
EXPECTED_GO_MOD = b"module github.com/jimmie-potts/i-can-do-groq-too\n\ngo 1.26.5\n"
FORBIDDEN_GO_METADATA = {"go.work", "go.work.sum", "go.sum"}
IMMUTABLE_ACTION_REF = re.compile(r"[0-9a-f]{40}")
WORKFLOW_STEPS_KEY = re.compile(r"^(?P<indent> *)steps:\s*(?:#.*)?$")
WORKFLOW_STEP_START = re.compile(
    r"^(?P<indent> *)-\s+(?P<key>[A-Za-z0-9_-]+):\s*(?P<value>.*)$"
)
WORKFLOW_MAPPING_KEY = re.compile(
    r"^(?P<indent> +)(?P<key>[A-Za-z0-9_-]+):\s*(?P<value>.*)$"
)
WORKFLOW_BLOCK_SCALAR = re.compile(r":\s*[|>][0-9+-]*\s*(?:#.*)?$")
WORKFLOW_BLOCK_SCALAR_VALUE = re.compile(r"[|>][0-9+-]*")
EXPECTED_GOVERSION_SCRIPTS = {
    "GOTOOLCHAIN=local go env GOVERSION",
    """expected_version="$(awk '$1 == "go" { print "go" $2 }' go.mod)"
actual_version="$(GOTOOLCHAIN=local go env GOVERSION)"
if [[ "$actual_version" != "$expected_version" ]]; then
  echo "Expected $expected_version from go.mod, got $actual_version" >&2
  exit 1
fi""",
}


class WorkflowStep(NamedTuple):
    """One structurally identified GitHub Actions step."""

    position: int
    steps_block: int
    fields: dict[str, str]
    with_fields: dict[str, str]
    run_script: str | None


def repository_files() -> list[Path]:
    """Return tracked and nonignored untracked files using Git's canonical ignore rules."""
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0:
        raise RuntimeError("unable to enumerate repository files; initialize Git before checking")
    paths = (ROOT / os.fsdecode(raw_path) for raw_path in result.stdout.split(b"\0") if raw_path)
    return sorted(path for path in paths if path.is_file() or path.is_symlink())


def environment_files() -> list[Path]:
    """Find forbidden environment filenames without opening them or ignored dependency trees."""
    matches: list[Path] = []
    for directory, directory_names, file_names in os.walk(ROOT):
        directory_names[:] = [name for name in directory_names if name not in IGNORED_PARTS]
        base = Path(directory)
        matches.extend(base / name for name in file_names if is_environment_file(base / name))
    return sorted(matches)


def check_required_paths(errors: list[str]) -> None:
    """Require the status, planning, lesson, and scoped-agent foundation."""
    for relative_path in REQUIRED_PATHS:
        if not (ROOT / relative_path).is_file():
            errors.append(f"missing required file: {relative_path}")


def check_text_hygiene(path: Path, errors: list[str]) -> bool:
    """Reject CRLF, invalid UTF-8, trailing whitespace, and missing final newlines."""
    if path.suffix.lower() not in TEXT_SUFFIXES:
        return True
    relative_path = path.relative_to(ROOT)
    try:
        raw = path.read_bytes()
        text = raw.decode("utf-8")
    except UnicodeDecodeError:
        errors.append(f"invalid UTF-8: {relative_path}")
        return False
    if b"\r\n" in raw or b"\r" in raw:
        errors.append(f"non-LF line ending: {relative_path}")
    if raw and not raw.endswith(b"\n"):
        errors.append(f"missing final newline: {relative_path}")
    for line_number, line in enumerate(text.splitlines(), start=1):
        if line.rstrip(" \t") != line:
            errors.append(f"trailing whitespace: {relative_path}:{line_number}")
    return True


def iter_markdown_links(path: Path) -> list[tuple[int, str]]:
    """Extract Markdown links outside fenced code blocks."""
    links: list[tuple[int, str]] = []
    in_fence = False
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        if line.lstrip().startswith("```"):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        links.extend((line_number, match.group(1).strip()) for match in MARKDOWN_LINK.finditer(line))
    return links


def markdown_destination(raw_target: str) -> str:
    """Return a CommonMark destination while discarding an optional link title."""
    stripped = raw_target.strip()
    if stripped.startswith("<"):
        closing = stripped.find(">")
        if closing != -1:
            return stripped[1:closing]
    return stripped.split(maxsplit=1)[0] if stripped else ""


def check_markdown_links(path: Path, errors: list[str]) -> None:
    """Verify relative file links without making network requests."""
    relative_path = path.relative_to(ROOT)
    for line_number, raw_target in iter_markdown_links(path):
        target = markdown_destination(raw_target)
        if not target or target.startswith(("#", "http://", "https://", "mailto:")):
            continue
        file_target = unquote(target.split("#", maxsplit=1)[0].split("?", maxsplit=1)[0])
        if not file_target:
            continue
        destination = (path.parent / file_target).resolve()
        try:
            destination.relative_to(ROOT)
        except ValueError:
            errors.append(f"link escapes repository: {relative_path}:{line_number}: {target}")
            continue
        if not destination.exists():
            errors.append(f"broken local link: {relative_path}:{line_number}: {target}")


def check_secret_filenames(files: list[Path], errors: list[str]) -> None:
    """Reject local environment files while allowing the reviewed example name."""
    for path in files:
        name = path.name
        if is_environment_file(path) and name != ".env.example":
            errors.append(
                f"local environment file is not allowed in the repository tree: "
                f"{path.relative_to(ROOT)}"
            )


def check_go_layout(files: list[Path], errors: list[str]) -> None:
    """Require the reviewed root manifest and reject extra Go metadata boundaries."""
    root_manifest = ROOT / "go.mod"
    if not root_manifest.is_file() or root_manifest.is_symlink():
        errors.append("missing regular root Go manifest: go.mod")
    else:
        try:
            manifest = root_manifest.read_bytes()
        except OSError:
            errors.append("unable to read root Go manifest: go.mod")
        else:
            if manifest != EXPECTED_GO_MOD:
                errors.append("root go.mod does not match ADR 0004 exactly")

    for path in files:
        relative_path = path.relative_to(ROOT)
        if path.name == "go.mod" and relative_path != Path("go.mod"):
            errors.append(f"nested Go module is not allowed: {relative_path}")
        if path.name in FORBIDDEN_GO_METADATA:
            errors.append(f"Go metadata is not allowed in ICGT-005: {relative_path}")


def workflow_steps(workflow: str) -> list[WorkflowStep]:
    """Extract block-style workflow steps while ignoring run-script contents."""
    raw_lines = workflow.splitlines(keepends=True)
    structure_lines = raw_lines.copy()
    block_indent: int | None = None
    positions: list[int] = []
    position = 0

    for index, raw_line in enumerate(raw_lines):
        positions.append(position)
        position += len(raw_line)
        line = raw_line.rstrip("\r\n")
        stripped = line.strip()
        indent = len(line) - len(line.lstrip(" "))

        if block_indent is not None:
            if not stripped or indent > block_indent:
                structure_lines[index] = "\n" if raw_line.endswith("\n") else ""
                continue
            block_indent = None

        if WORKFLOW_BLOCK_SCALAR.search(line):
            block_indent = indent

    steps: list[WorkflowStep] = []
    for index, line in enumerate(structure_lines):
        steps_match = WORKFLOW_STEPS_KEY.match(line.rstrip("\r\n"))
        if steps_match is None:
            continue

        steps_indent = len(steps_match.group("indent"))
        item_indent: int | None = None
        item_starts: list[int] = []
        end = index + 1
        while end < len(structure_lines):
            candidate = structure_lines[end].rstrip("\r\n")
            stripped = candidate.strip()
            if not stripped or stripped.startswith("#"):
                end += 1
                continue
            indent = len(candidate) - len(candidate.lstrip(" "))
            if indent <= steps_indent:
                break

            item_match = WORKFLOW_STEP_START.match(candidate)
            if item_match is not None:
                candidate_indent = len(item_match.group("indent"))
                if item_indent is None:
                    item_indent = candidate_indent
                if candidate_indent == item_indent:
                    item_starts.append(end)
            end += 1

        for item_index, start in enumerate(item_starts):
            stop = item_starts[item_index + 1] if item_index + 1 < len(item_starts) else end
            steps.append(
                parse_workflow_step(
                    raw_lines,
                    structure_lines,
                    positions[start],
                    positions[index],
                    start,
                    stop,
                )
            )
    return steps


def parse_workflow_step(
    raw_lines: list[str],
    structure_lines: list[str],
    position: int,
    steps_block: int,
    start: int,
    stop: int,
) -> WorkflowStep:
    """Parse direct fields and the direct with mapping for one workflow step."""
    first_match = WORKFLOW_STEP_START.match(structure_lines[start].rstrip("\r\n"))
    if first_match is None:
        raise ValueError("workflow step does not begin with a mapping key")

    item_indent = len(first_match.group("indent"))
    field_indent = item_indent + 2
    fields = {first_match.group("key"): yaml_scalar(first_match.group("value"))}
    field_lines = {first_match.group("key"): start}

    for index in range(start + 1, stop):
        mapping_match = WORKFLOW_MAPPING_KEY.match(structure_lines[index].rstrip("\r\n"))
        if mapping_match is None or len(mapping_match.group("indent")) != field_indent:
            continue
        key = mapping_match.group("key")
        fields[key] = yaml_scalar(mapping_match.group("value"))
        field_lines[key] = index

    with_fields: dict[str, str] = {}
    with_line = field_lines.get("with")
    if with_line is not None:
        nested_indent: int | None = None
        for index in range(with_line + 1, stop):
            line = structure_lines[index].rstrip("\r\n")
            if not line.strip() or line.lstrip().startswith("#"):
                continue
            indent = len(line) - len(line.lstrip(" "))
            if indent <= field_indent:
                break
            mapping_match = WORKFLOW_MAPPING_KEY.match(line)
            if mapping_match is None:
                continue
            if nested_indent is None:
                nested_indent = indent
            if indent == nested_indent:
                with_fields[mapping_match.group("key")] = yaml_scalar(
                    mapping_match.group("value")
                )

    run_script: str | None = None
    run_line = field_lines.get("run")
    if run_line is not None:
        run_value = fields["run"]
        if WORKFLOW_BLOCK_SCALAR_VALUE.fullmatch(run_value):
            body_lines: list[str] = []
            for index in range(run_line + 1, stop):
                raw_line = raw_lines[index]
                line = raw_line.rstrip("\r\n")
                if line.strip():
                    indent = len(line) - len(line.lstrip(" "))
                    if indent <= field_indent:
                        break
                body_lines.append(raw_line)
            run_script = textwrap.dedent("".join(body_lines)).strip()
        else:
            run_script = run_value

    return WorkflowStep(
        position=position,
        steps_block=steps_block,
        fields=fields,
        with_fields=with_fields,
        run_script=run_script,
    )


def yaml_scalar(raw_value: str) -> str:
    """Normalize the small plain or quoted scalar subset used by the workflow policy."""
    value = raw_value.strip()
    if " #" in value:
        value = value.split(" #", maxsplit=1)[0].rstrip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        return value[1:-1]
    return value


def check_ci_go_policy(errors: list[str]) -> None:
    """Require immutable, single-source Go provisioning around the canonical gate."""
    workflow_path = ROOT / ".github" / "workflows" / "check.yml"
    try:
        workflow = workflow_path.read_text(encoding="utf-8")
    except (OSError, UnicodeDecodeError):
        return

    steps = workflow_steps(workflow)
    setup_steps: list[tuple[WorkflowStep, str]] = []
    for step in steps:
        uses = step.fields.get("uses", "")
        prefix = "actions/setup-go@"
        if uses.startswith(prefix):
            setup_steps.append((step, uses.removeprefix(prefix)))

    if len(setup_steps) != 1:
        errors.append("CI must contain exactly one actions/setup-go invocation")

    if not setup_steps or any(
        IMMUTABLE_ACTION_REF.fullmatch(reference) is None for _, reference in setup_steps
    ):
        errors.append("CI setup-go action must use a 40-character commit SHA")

    if len(setup_steps) == 1:
        setup_step, _ = setup_steps[0]
        setup_position = setup_step.position
        if "if" in setup_step.fields:
            errors.append("CI setup-go step must be unconditional")
        if setup_step.with_fields.get("go-version-file") != "go.mod":
            errors.append("CI setup-go must read go-version-file from go.mod")
        if "go-version" in setup_step.with_fields:
            errors.append("CI must not repeat a separate go-version literal")
        if setup_step.with_fields.get("cache", "").lower() != "false":
            errors.append("CI setup-go caching must be disabled for the dependency-free module")
    else:
        setup_position = -1
    preflight_steps = [
        step
        for step in steps
        if "if" not in step.fields and step.run_script == "python3 scripts/check_repository.py"
    ]
    version_steps = [
        step
        for step in steps
        if "if" not in step.fields and step.run_script in EXPECTED_GOVERSION_SCRIPTS
    ]
    gate_steps = [
        step for step in steps if "if" not in step.fields and step.run_script == "./scripts/check"
    ]

    if len(preflight_steps) != 1:
        errors.append("CI must contain exactly one unconditional static policy run step")
    if len(version_steps) != 1:
        errors.append("CI must contain exactly one unconditional GOVERSION verification step")
    if len(gate_steps) != 1:
        errors.append("CI must contain exactly one unconditional canonical gate run step")

    preflight_position = preflight_steps[0].position if len(preflight_steps) == 1 else -1
    version_position = version_steps[0].position if len(version_steps) == 1 else -1
    gate_position = gate_steps[0].position if len(gate_steps) == 1 else -1
    if (
        len(preflight_steps) == 1
        and len(setup_steps) == 1
        and len(version_steps) == 1
        and len(gate_steps) == 1
        and len(
            {
                preflight_steps[0].steps_block,
                setup_steps[0][0].steps_block,
                version_steps[0].steps_block,
                gate_steps[0].steps_block,
            }
        )
        != 1
    ):
        errors.append("CI policy, setup-go, GOVERSION, and gate steps must share one steps block")
    if (
        preflight_position < 0
        or setup_position < 0
        or version_position < 0
        or gate_position < 0
        or not preflight_position < setup_position < version_position < gate_position
    ):
        errors.append(
            "CI static policy, setup-go, GOVERSION verification, and canonical gate stages "
            "are out of order"
        )


def is_environment_file(path: Path) -> bool:
    """Identify environment filenames whose contents the gate must never inspect."""
    name = path.name
    return name == ".env" or name.startswith(".env.") or name.endswith(".env")


def metadata_files(
    directory: Path,
    pattern: re.Pattern[str],
    kind: str,
    errors: list[str],
) -> dict[str, Path]:
    """Index story or lesson files and reject duplicate normalized unit IDs."""
    indexed: dict[str, Path] = {}
    for path in sorted(directory.glob("icgt-*.md")):
        match = pattern.fullmatch(path.name)
        if match:
            unit = match.group(1).upper()
            if unit in indexed:
                first = indexed[unit].relative_to(ROOT)
                second = path.relative_to(ROOT)
                errors.append(f"duplicate {kind} ID: {unit}: {first}, {second}")
                continue
            indexed[unit] = path
    return indexed


def section_body(text: str, heading: str) -> str:
    """Return one second-level Markdown section body, or an empty string."""
    marker = f"## {heading}\n"
    if marker not in text:
        return ""
    body = text.split(marker, maxsplit=1)[1]
    return body.split("\n## ", maxsplit=1)[0]


def check_learning_metadata(errors: list[str]) -> None:
    """Enforce one-to-one story/lesson IDs, status parity, and review evidence fields."""
    stories = metadata_files(ROOT / "user-stories", STORY_FILENAME, "story", errors)
    lessons = metadata_files(ROOT / "docs" / "lessons", LESSON_FILENAME, "lesson", errors)
    for unit in sorted(stories.keys() - lessons.keys()):
        errors.append(f"story has no matching lesson: {unit}")
    for unit in sorted(lessons.keys() - stories.keys()):
        errors.append(f"lesson has no matching story: {unit}")

    for unit in sorted(stories.keys() & lessons.keys()):
        story_path = stories[unit]
        lesson_path = lessons[unit]
        story_relative = story_path.relative_to(ROOT)
        lesson_relative = lesson_path.relative_to(ROOT)
        try:
            story_text = story_path.read_text(encoding="utf-8")
            lesson_text = lesson_path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            # Text hygiene already reports the specific invalid file.
            continue

        story_status_match = STORY_STATUS.search(story_text)
        lesson_status_match = LESSON_STATUS.search(lesson_text)
        if not story_status_match:
            errors.append(f"story status missing: {story_relative}")
        if not lesson_status_match:
            errors.append(f"lesson status missing: {lesson_relative}")
        if story_status_match and lesson_status_match:
            story_status = story_status_match.group(1)
            expected = EXPECTED_LESSON_STATUS.get(story_status)
            if expected is None:
                errors.append(f"unsupported story status: {story_relative}: {story_status}")
            elif lesson_status_match.group(1) != expected:
                errors.append(
                    f"story/lesson status mismatch: {unit}: {story_status} requires {expected}"
                )

        lesson_unit_match = LESSON_UNIT.search(lesson_text)
        if not lesson_unit_match or lesson_unit_match.group(1) != unit:
            errors.append(f"lesson unit metadata mismatch: {lesson_relative}: expected {unit}")

        for label in (
            "- **Lesson:**",
            "- **Review priority:**",
            "## Human review checkpoint",
            "- **Production path:**",
            "- **Failure/test path:**",
            "- **Invariant:**",
            "- **Deferred:**",
        ):
            if label not in story_text:
                errors.append(f"story review metadata missing: {story_relative}: {label}")

        for label in (
            "- **Story:**",
            "- **Review priority:**",
            "## Personal code review map",
            "## Teach-back questions",
        ):
            if label not in lesson_text:
                errors.append(f"lesson review metadata missing: {lesson_relative}: {label}")

        teach_back = section_body(lesson_text, "Teach-back questions")
        question_numbers = re.findall(r"^([1-9]\d*)\. ", teach_back, re.MULTILINE)
        if question_numbers != ["1", "2", "3"]:
            errors.append(f"lesson must contain exactly three teach-back questions: {lesson_relative}")


def main() -> int:
    """Run all dependency-free checks and print actionable failures."""
    errors: list[str] = []
    try:
        files = repository_files()
    except RuntimeError as error:
        print(error, file=sys.stderr)
        return 1
    check_required_paths(errors)
    check_secret_filenames(environment_files(), errors)
    check_go_layout(files, errors)
    check_ci_go_policy(errors)
    for path in files:
        if is_environment_file(path):
            continue
        valid_text = check_text_hygiene(path, errors)
        if valid_text and path.suffix.lower() == ".md":
            check_markdown_links(path, errors)
    check_learning_metadata(errors)
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print(f"Checked {len(files)} repository files.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
