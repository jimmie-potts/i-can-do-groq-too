from __future__ import annotations

import contextlib
import importlib.util
import io
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "check_repository.py"
SPEC = importlib.util.spec_from_file_location("check_repository", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


class RepositoryCheckerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory(prefix="icgt-policy-")
        self.root = Path(self.temporary_directory.name)
        self.original_root = CHECKER.ROOT
        CHECKER.ROOT = self.root
        self.addCleanup(self._cleanup)
        for relative_path in CHECKER.REQUIRED_PATHS:
            path = self.root / relative_path
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(f"# {path.stem}\n", encoding="utf-8")
        (self.root / ".gitignore").write_text(
            """.env
.env.*
*.env
!.env.example
.agents/
.codex/
.idea/
.npm/
.vscode/
node_modules/
""",
            encoding="utf-8",
        )
        subprocess.run(
            ["git", "init", "--quiet", "--initial-branch=main"],
            cwd=self.root,
            check=True,
        )

    def _cleanup(self) -> None:
        CHECKER.ROOT = self.original_root
        self.temporary_directory.cleanup()

    def run_checker(self) -> tuple[int, str]:
        error_output = io.StringIO()
        with contextlib.redirect_stderr(error_output):
            result = CHECKER.main()
        return result, error_output.getvalue()

    def test_invalid_utf8_markdown_fails_without_traceback(self) -> None:
        (self.root / "invalid.md").write_bytes(b"# Invalid\n\xff\n")

        result, errors = self.run_checker()

        self.assertEqual(result, 1)
        self.assertIn("invalid UTF-8: invalid.md", errors)

    def test_commonmark_link_title_is_ignored(self) -> None:
        (self.root / "target.md").write_text("# Target\n", encoding="utf-8")
        (self.root / "source.md").write_text(
            '# Source\n\n[target](target.md "Optional title")\n', encoding="utf-8"
        )

        errors: list[str] = []
        CHECKER.check_markdown_links(self.root / "source.md", errors)

        self.assertEqual(errors, [])

    def test_link_escape_is_rejected(self) -> None:
        (self.root / "source.md").write_text(
            "# Source\n\n[outside](../outside.md)\n", encoding="utf-8"
        )

        errors: list[str] = []
        CHECKER.check_markdown_links(self.root / "source.md", errors)

        self.assertEqual(len(errors), 1)
        self.assertIn("link escapes repository", errors[0])

    def test_environment_file_is_rejected_without_reading_contents(self) -> None:
        environment_file = self.root / ".env.local"
        environment_file.write_text("SECRET=sentinel\n", encoding="utf-8")
        environment_file.chmod(0)
        self.addCleanup(environment_file.chmod, 0o600)

        result, errors = self.run_checker()

        self.assertEqual(result, 1)
        self.assertIn(
            "local environment file is not allowed in the repository tree: .env.local", errors
        )
        self.assertNotIn("sentinel", errors)

    def test_ignored_dependency_directory_is_not_scanned(self) -> None:
        module = self.root / "node_modules" / "fixture" / "go.mod"
        module.parent.mkdir(parents=True)
        module.write_text("module ignored.example\n", encoding="utf-8")

        files = CHECKER.repository_files()

        self.assertNotIn(module, files)

    def test_ignored_editor_file_is_not_scanned(self) -> None:
        settings = self.root / ".vscode" / "settings.json"
        settings.parent.mkdir(parents=True)
        settings.write_text('{"ignored": true}   \n', encoding="utf-8")

        files = CHECKER.repository_files()

        self.assertNotIn(settings, files)

    def test_story_and_lesson_status_must_match(self) -> None:
        story = self.root / "user-stories" / "icgt-999-example.md"
        lesson = self.root / "docs" / "lessons" / "icgt-999-example.md"
        story.write_text(
            """# ICGT-999 - Example

- **Status:** Done
- **Lesson:** [Example](../docs/lessons/icgt-999-example.md)
- **Review priority:** Normal

## Human review checkpoint

- **Production path:** Example.
- **Failure/test path:** Example.
- **Invariant:** Example.
- **Deferred:** Example.
""",
            encoding="utf-8",
        )
        lesson.write_text(
            """# ICGT-999 lesson: Example

- **Unit:** ICGT-999
- **Lesson status:** Planned
- **Story:** [Example](../../user-stories/icgt-999-example.md)
- **Review priority:** Normal

## Personal code review map

Example.

## Teach-back questions

1. One?
2. Two?
3. Three?
""",
            encoding="utf-8",
        )

        errors: list[str] = []
        CHECKER.check_learning_metadata(errors)

        self.assertIn(
            "story/lesson status mismatch: ICGT-999: Done requires Verified against implementation",
            errors,
        )

    def test_lesson_requires_exactly_three_teach_back_questions(self) -> None:
        story = self.root / "user-stories" / "icgt-999-example.md"
        lesson = self.root / "docs" / "lessons" / "icgt-999-example.md"
        story.write_text(
            """# ICGT-999 - Example

- **Status:** Planned
- **Lesson:** [Example](../docs/lessons/icgt-999-example.md)
- **Review priority:** Normal

## Human review checkpoint

- **Production path:** Example.
- **Failure/test path:** Example.
- **Invariant:** Example.
- **Deferred:** Example.
""",
            encoding="utf-8",
        )
        lesson.write_text(
            """# ICGT-999 lesson: Example

- **Unit:** ICGT-999
- **Lesson status:** Planned
- **Story:** [Example](../../user-stories/icgt-999-example.md)
- **Review priority:** Normal

## Personal code review map

Example.

## Teach-back questions

1. One?
2. Two?
""",
            encoding="utf-8",
        )

        errors: list[str] = []
        CHECKER.check_learning_metadata(errors)

        self.assertIn(
            "lesson must contain exactly three teach-back questions: "
            "docs/lessons/icgt-999-example.md",
            errors,
        )

    def test_duplicate_story_ids_are_rejected(self) -> None:
        first = self.root / "user-stories" / "icgt-999-first.md"
        second = self.root / "user-stories" / "icgt-999-second.md"
        first.write_text("# First\n", encoding="utf-8")
        second.write_text("# Second\n", encoding="utf-8")

        errors: list[str] = []
        CHECKER.check_learning_metadata(errors)

        self.assertTrue(any(error.startswith("duplicate story ID: ICGT-999:") for error in errors))

    def test_duplicate_lesson_ids_are_rejected(self) -> None:
        first = self.root / "docs" / "lessons" / "icgt-999-first.md"
        second = self.root / "docs" / "lessons" / "icgt-999-second.md"
        first.write_text("# First\n", encoding="utf-8")
        second.write_text("# Second\n", encoding="utf-8")

        errors: list[str] = []
        CHECKER.check_learning_metadata(errors)

        self.assertTrue(any(error.startswith("duplicate lesson ID: ICGT-999:") for error in errors))

    def test_critical_foundation_artifacts_are_required(self) -> None:
        for relative_path in (
            ".github/workflows/check.yml",
            "tests/test_check_repository.py",
            "docs/lessons/AGENTS.md",
            "user-stories/AGENTS.md",
        ):
            with self.subTest(relative_path=relative_path):
                path = self.root / relative_path
                original = path.read_text(encoding="utf-8")
                path.unlink()
                errors: list[str] = []

                CHECKER.check_required_paths(errors)

                self.assertIn(f"missing required file: {relative_path}", errors)
                path.write_text(original, encoding="utf-8")


if __name__ == "__main__":
    unittest.main()
