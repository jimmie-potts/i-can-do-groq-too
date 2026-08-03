from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import tempfile
import unittest
from decimal import Decimal
from pathlib import Path


SCRIPT = Path(__file__).resolve().parents[1] / "scripts" / "check_contract.py"
SPEC = importlib.util.spec_from_file_location("check_contract", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
CHECKER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKER)


class ContractCheckerTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temporary_directory = tempfile.TemporaryDirectory(prefix="icgt-contract-")
        self.root = Path(self.temporary_directory.name)
        self.addCleanup(self.temporary_directory.cleanup)
        self.contract = self.root / CHECKER.CONTRACT_RELATIVE
        self.schema_path = self.contract / "schema" / "request.schema.json"
        self.success_schema_path = self.contract / "schema" / "success.schema.json"
        self.failure_schema_path = self.contract / "schema" / "failure.schema.json"
        self.cases_path = self.contract / "fixtures" / "cases.json"
        self.valid_path = self.contract / "fixtures" / "valid" / "minimal.json"
        self.invalid_path = self.contract / "fixtures" / "invalid" / "count-too-small.json"
        self.success_valid_path = self.contract / "fixtures" / "valid" / "success.json"
        self.success_invalid_path = self.contract / "fixtures" / "invalid" / "success-count.json"
        self.failure_valid_path = self.contract / "fixtures" / "valid" / "failure.json"
        self.failure_invalid_path = self.contract / "fixtures" / "invalid" / "failure-count.json"
        self.schema = {
            "$schema": CHECKER.SCHEMA_URI,
            "$id": "https://example.invalid/fastgate/model-turn/v1/request.schema.json",
            "title": "Test request",
            "type": "object",
            "properties": {
                "count": {"type": "integer", "minimum": 0, "maximum": 2},
                "name": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 4,
                    "pattern": "^[a-z]+(?![\\s\\S])",
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string", "const": "x"},
                    "minItems": 1,
                    "maxItems": 2,
                },
            },
            "required": ["count", "name"],
            "additionalProperties": False,
        }
        self.cases = {
            "version": "v1",
            "cases": [
                {
                    "name": "minimal-valid-request",
                    "schema": "schema/request.schema.json",
                    "fixture": "fixtures/valid/minimal.json",
                    "valid": True,
                    "meaning": "A compact valid request.",
                },
                {
                    "name": "count-too-small",
                    "schema": "schema/request.schema.json",
                    "fixture": "fixtures/invalid/count-too-small.json",
                    "valid": False,
                    "expected": {"keyword": "minimum", "instance_path": "/count"},
                },
                {
                    "name": "minimal-valid-success",
                    "schema": "schema/success.schema.json",
                    "fixture": "fixtures/valid/success.json",
                    "valid": True,
                },
                {
                    "name": "success-count-too-small",
                    "schema": "schema/success.schema.json",
                    "fixture": "fixtures/invalid/success-count.json",
                    "valid": False,
                    "expected": {"keyword": "minimum", "instance_path": "/count"},
                },
                {
                    "name": "minimal-valid-failure",
                    "schema": "schema/failure.schema.json",
                    "fixture": "fixtures/valid/failure.json",
                    "valid": True,
                },
                {
                    "name": "failure-count-too-small",
                    "schema": "schema/failure.schema.json",
                    "fixture": "fixtures/invalid/failure-count.json",
                    "valid": False,
                    "expected": {"keyword": "minimum", "instance_path": "/count"},
                },
            ],
        }
        self.write_json(self.schema_path, self.schema)
        self.write_json(self.success_schema_path, self.schema)
        self.write_json(self.failure_schema_path, self.schema)
        self.write_json(self.cases_path, self.cases)
        self.write_json(self.valid_path, {"count": 1, "name": "ok", "tags": ["x"]})
        self.write_json(self.invalid_path, {"count": -1, "name": "ok"})
        self.write_json(self.success_valid_path, {"count": 1, "name": "ok"})
        self.write_json(self.success_invalid_path, {"count": -1, "name": "ok"})
        self.write_json(self.failure_valid_path, {"count": 1, "name": "ok"})
        self.write_json(self.failure_invalid_path, {"count": -1, "name": "ok"})

    @staticmethod
    def write_json(path: Path, document: object) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")

    def check(self) -> list[str]:
        return CHECKER.check_contract(self.root)

    def test_committed_contract_corpus_passes(self) -> None:
        errors = CHECKER.check_contract(CHECKER.ROOT)

        self.assertEqual(errors, [])

    def test_complete_temporary_corpus_passes(self) -> None:
        self.assertEqual(self.check(), [])

    def test_schema_profile_rejects_unknown_and_malformed_keywords(self) -> None:
        cases = (
            ("unknown", lambda schema: schema.update({"$ref": "elsewhere.json"})),
            ("malformed", lambda schema: schema.update({"maxItems": 1})),
            ("type array", lambda schema: schema.update({"type": ["object"]})),
            ("bad bound", lambda schema: schema["properties"]["name"].update({"maxLength": True})),
            ("bad pattern", lambda schema: schema["properties"]["name"].update({"pattern": "["})),
        )
        for name, mutate in cases:
            with self.subTest(name=name):
                schema = json.loads(json.dumps(self.schema))
                mutate(schema)
                self.write_json(self.schema_path, schema)

                errors = self.check()

                self.assertTrue(any("schema" in error for error in errors), errors)
        self.write_json(self.schema_path, self.schema)

    def test_unreferenced_canonical_schema_is_still_validated_and_requires_coverage(self) -> None:
        self.cases["cases"] = [
            case
            for case in self.cases["cases"]
            if case["schema"] != "schema/failure.schema.json"
        ]
        malformed = json.loads(json.dumps(self.schema))
        malformed["$ref"] = "ignored.json"
        self.write_json(self.failure_schema_path, malformed)
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(
            any("schema schema/failure.schema.json" in error and "$ref" in error for error in errors),
            errors,
        )
        self.assertTrue(
            any(
                "schema/failure.schema.json requires at least one valid fixture" in error
                for error in errors
            ),
            errors,
        )
        self.assertTrue(
            any(
                "schema/failure.schema.json requires at least one invalid fixture" in error
                for error in errors
            ),
            errors,
        )

    def test_schema_inventory_and_duplicated_public_rules_cannot_drift(self) -> None:
        unexpected = self.contract / "schema" / "unexpected.schema.json"
        self.write_json(unexpected, self.schema)

        shared_identifier = {
            "type": "string",
            "minLength": 1,
            "maxLength": 8,
        }
        for path in (self.schema_path, self.success_schema_path, self.failure_schema_path):
            schema = json.loads(path.read_text(encoding="utf-8"))
            schema["properties"]["request_id"] = shared_identifier
            self.write_json(path, schema)
        success = json.loads(self.success_schema_path.read_text(encoding="utf-8"))
        success["properties"]["request_id"]["maxLength"] = 9
        self.write_json(self.success_schema_path, success)

        errors = self.check()

        self.assertTrue(any("unexpected file schema/unexpected.schema.json" in error for error in errors))
        self.assertTrue(any("request_id rules must remain identical" in error for error in errors))

    def test_schema_parity_covers_identifier_usage_and_control_pattern(self) -> None:
        identifier = {"type": "string", "maxLength": 8}
        usage = {"type": "integer", "maximum": 10}
        request = {
            "properties": {
                "request_id": identifier,
                "repository_instructions": {
                    "items": {
                        "properties": {
                            "source": {"pattern": "control-safe"},
                        }
                    }
                },
            }
        }
        success = {"properties": {"request_id": identifier, "usage": usage}}
        failure = {
            "properties": {
                "request_id": identifier,
                "usage": usage,
                "error": {
                    "properties": {
                        "message": {"pattern": "control-safe"},
                    }
                },
            }
        }
        schemas = {
            self.contract / CHECKER.REQUEST_SCHEMA: request,
            self.contract / CHECKER.SUCCESS_SCHEMA: success,
            self.contract / CHECKER.FAILURE_SCHEMA: failure,
        }
        errors: list[str] = []

        CHECKER.check_schema_parity(self.contract, schemas, errors)

        self.assertEqual(errors, [])

        success["properties"]["request_id"] = {"type": "string", "maxLength": 9}
        failure["properties"]["usage"] = {"type": "integer", "maximum": 11}
        failure["properties"]["error"]["properties"]["message"]["pattern"] = "different"

        CHECKER.check_schema_parity(self.contract, schemas, errors)

        self.assertTrue(any("request_id rules must remain identical" in error for error in errors))
        self.assertTrue(any("usage rules must remain identical" in error for error in errors))
        self.assertTrue(any("control-safe pattern rules must remain identical" in error for error in errors))

    def test_bounds_pattern_and_boolean_integer_are_enforced(self) -> None:
        cases = (
            ({"count": -1, "name": "ok"}, CHECKER.Violation("minimum", "/count")),
            ({"count": 3, "name": "ok"}, CHECKER.Violation("maximum", "/count")),
            ({"count": True, "name": "ok"}, CHECKER.Violation("type", "/count")),
            ({"count": float("inf"), "name": "ok"}, CHECKER.Violation("type", "/count")),
            ({"count": 1, "name": ""}, CHECKER.Violation("minLength", "/name")),
            ({"count": 1, "name": "abcde"}, CHECKER.Violation("maxLength", "/name")),
            ({"count": 1, "name": "UP"}, CHECKER.Violation("pattern", "/name")),
            ({"count": 1, "name": "ok\n"}, CHECKER.Violation("pattern", "/name")),
            (
                {"count": 1, "name": "ok", "tags": []},
                CHECKER.Violation("minItems", "/tags"),
            ),
            (
                {"count": 1, "name": "ok", "tags": ["x", "x", "x"]},
                CHECKER.Violation("maxItems", "/tags"),
            ),
        )
        for document, expected in cases:
            with self.subTest(expected=expected):
                violations = CHECKER.validate_instance(document, self.schema)

                self.assertIn(expected, violations)

        self.assertEqual(CHECKER.validate_instance({"count": 1.0, "name": "ok"}, self.schema), [])
        self.assertTrue(CHECKER.matches_type(10**10_000, "number"))

    def test_strict_json_rejects_duplicate_keys_and_non_finite_numbers(self) -> None:
        malformed = self.root / "malformed.json"
        cases = (
            '{"field": 1, "field": 2}\n',
            '{"field": NaN}\n',
            '{"field": Infinity}\n',
            '{"field": "\\ud800"}\n',
            '{"field": "unterminated}\n',
        )
        for content in cases:
            with self.subTest(content=content):
                malformed.write_text(content, encoding="utf-8")

                with self.assertRaises(CHECKER.JSONDocumentError):
                    CHECKER.strict_json(malformed)

        malformed.write_text('{"field": 1e999}\n', encoding="utf-8")
        document = CHECKER.strict_json(malformed)
        self.assertEqual(document["field"], Decimal("1e999"))

        malformed.write_text('{"field": "\\ud83d\\ude00"}\n', encoding="utf-8")
        document = CHECKER.strict_json(malformed)
        self.assertEqual(document["field"], "😀")

    def test_exact_decimal_preserves_integer_and_maximum_semantics(self) -> None:
        fraction = Decimal("9007199254740990.5")
        self.assertFalse(CHECKER.matches_type(fraction, "integer"))
        violations = CHECKER.validate_instance({"count": fraction, "name": "ok"}, self.schema)
        self.assertIn(CHECKER.Violation("type", "/count"), violations)

        violations = CHECKER.validate_instance(
            {"count": Decimal("1e999"), "name": "ok"}, self.schema
        )
        self.assertIn(CHECKER.Violation("maximum", "/count"), violations)

    def test_schema_profile_rejects_enum_and_const_type_mismatches(self) -> None:
        schema = json.loads(json.dumps(self.schema))
        schema["properties"]["count"]["enum"] = ["one"]
        schema["properties"]["name"]["const"] = 1

        issues = CHECKER.schema_issues(schema)

        self.assertTrue(any("enum entries must match type" in issue for issue in issues), issues)
        self.assertTrue(any("const must match type" in issue for issue in issues), issues)

    def test_invalid_json_fixture_can_expect_json_failure(self) -> None:
        self.invalid_path.write_text('{"count":\n', encoding="utf-8")
        self.cases["cases"][1]["expected"] = {"keyword": "json", "instance_path": ""}
        self.write_json(self.cases_path, self.cases)

        self.assertEqual(self.check(), [])

    def test_manifest_rejects_duplicate_case_names_and_fixture_paths(self) -> None:
        duplicate = dict(self.cases["cases"][0])
        duplicate["valid"] = False
        duplicate["expected"] = {"keyword": "minimum", "instance_path": "/count"}
        duplicate.pop("meaning")
        duplicate["fixture"] = "fixtures/invalid/count-too-small.json"
        self.cases["cases"].append(duplicate)
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("duplicate case name" in error for error in errors), errors)
        self.assertTrue(any("duplicate fixture path" in error for error in errors), errors)

    def test_manifest_rejects_non_string_expected_keyword_without_traceback(self) -> None:
        self.cases["cases"][1]["expected"]["keyword"] = []
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("expected keyword or instance_path is invalid" in error for error in errors))

    def test_valid_manifest_case_must_omit_even_null_expected_metadata(self) -> None:
        self.cases["cases"][0]["expected"] = None
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("valid cases must omit expected" in error for error in errors))

    def test_manifest_rejects_missing_escaping_and_orphaned_fixture_paths(self) -> None:
        orphan = self.contract / "fixtures" / "valid" / "orphan.json"
        self.write_json(orphan, {})
        self.cases["cases"][0]["fixture"] = "../outside.json"
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("normalized contract-relative" in error for error in errors), errors)
        self.assertTrue(any("orphaned file" in error for error in errors), errors)
        self.cases["cases"][0]["fixture"] = "fixtures/valid/missing.json"
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("existing regular file" in error for error in errors), errors)

    def test_manifest_rejects_noncanonical_relative_paths(self) -> None:
        for fixture in ("fixtures/valid//minimal.json", "fixtures/valid/./minimal.json"):
            with self.subTest(fixture=fixture):
                self.cases["cases"][0]["fixture"] = fixture
                self.write_json(self.cases_path, self.cases)

                errors = self.check()

                self.assertTrue(any("normalized contract-relative" in error for error in errors))

    def test_manifest_rejects_symlinked_fixture(self) -> None:
        linked = self.contract / "fixtures" / "valid" / "linked.json"
        linked.symlink_to(self.valid_path)
        self.cases["cases"][0]["fixture"] = "fixtures/valid/linked.json"
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("symlink" in error for error in errors), errors)

    def test_invalid_fixture_must_match_its_intended_rule(self) -> None:
        self.cases["cases"][1]["expected"] = {
            "keyword": "maximum",
            "instance_path": "/count",
        }
        self.write_json(self.cases_path, self.cases)

        errors = self.check()

        self.assertTrue(any("intended rule mismatch" in error for error in errors), errors)

    def test_invalid_fixture_can_have_additional_failures(self) -> None:
        self.write_json(self.invalid_path, {"count": -1, "name": ""})

        self.assertEqual(self.check(), [])

    def test_diagnostics_do_not_echo_fixture_values(self) -> None:
        sentinel = "private-prompt-sentinel"
        self.write_json(self.valid_path, {"count": 1, "name": sentinel})

        errors = self.check()

        self.assertTrue(errors)
        self.assertNotIn(sentinel, "\n".join(errors))

    def test_cli_rejects_arguments_without_running_the_check(self) -> None:
        error_output = io.StringIO()
        with contextlib.redirect_stderr(error_output):
            result = CHECKER.main(["unexpected"])

        self.assertEqual(result, 2)
        self.assertEqual(error_output.getvalue(), "Usage: python3 scripts/check_contract.py\n")


if __name__ == "__main__":
    unittest.main()
