#!/usr/bin/env python3
"""Run dependency-free repository hygiene and local Markdown-link checks."""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path
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
    "scripts/check_repository.py",
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
