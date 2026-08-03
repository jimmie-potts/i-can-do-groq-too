from __future__ import annotations

import contextlib
import importlib.util
import io
import json
import re
import tempfile
import unittest
from decimal import Decimal, InvalidOperation, localcontext
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
        identifier = {
            "type": "string",
            "minLength": 1,
            "maxLength": 128,
            "pattern": CHECKER.IDENTIFIER_PATTERN,
        }
        self.schema = {
            "$schema": CHECKER.SCHEMA_URI,
            "$id": CHECKER.CANONICAL_SCHEMA_METADATA[CHECKER.REQUEST_SCHEMA][0],
            "title": "Test request",
            "type": "object",
            "properties": {
                "version": {"type": "string", "const": "v1"},
                "kind": {"type": "string", "const": "model_turn.request"},
                "request_id": dict(identifier),
                "model_alias": dict(identifier),
                "count": {"type": "integer", "minimum": 0, "maximum": 2},
                "name": {
                    "type": "string",
                    "minLength": 1,
                    "maxLength": 4,
                    "pattern": CHECKER.IDENTIFIER_PATTERN,
                },
                "tags": {
                    "type": "array",
                    "items": {"type": "string", "const": "x"},
                    "minItems": 1,
                    "maxItems": 2,
                },
                "instructions": {
                    "type": "array",
                    "items": {"type": "string"},
                    "minItems": 0,
                    "maxItems": 2,
                },
                "details": {
                    "type": "object",
                    "properties": {"note": {"type": "string"}},
                    "required": [],
                    "additionalProperties": False,
                },
            },
            "required": [
                "version",
                "kind",
                "request_id",
                "model_alias",
                "count",
                "name",
                "instructions",
            ],
            "additionalProperties": False,
        }
        self.success_schema = json.loads(json.dumps(self.schema))
        self.success_schema["$id"] = CHECKER.CANONICAL_SCHEMA_METADATA[CHECKER.SUCCESS_SCHEMA][0]
        self.success_schema["properties"]["kind"]["const"] = "model_turn.completed"
        del self.success_schema["properties"]["model_alias"]
        self.success_schema["required"].remove("model_alias")
        self.failure_schema = json.loads(json.dumps(self.success_schema))
        self.failure_schema["$id"] = CHECKER.CANONICAL_SCHEMA_METADATA[CHECKER.FAILURE_SCHEMA][0]
        self.failure_schema["properties"]["kind"]["const"] = "model_turn.failed"
        original_required_fields = CHECKER.CANONICAL_REQUIRED_FIELDS
        original_root_fields = CHECKER.CANONICAL_ROOT_FIELDS
        self.production_required_fields = original_required_fields
        self.production_root_fields = original_root_fields
        self.addCleanup(setattr, CHECKER, "CANONICAL_REQUIRED_FIELDS", original_required_fields)
        self.addCleanup(setattr, CHECKER, "CANONICAL_ROOT_FIELDS", original_root_fields)
        CHECKER.CANONICAL_REQUIRED_FIELDS = {
            CHECKER.REQUEST_SCHEMA: frozenset(self.schema["required"]),
            CHECKER.SUCCESS_SCHEMA: frozenset(self.success_schema["required"]),
            CHECKER.FAILURE_SCHEMA: frozenset(self.failure_schema["required"]),
        }
        CHECKER.CANONICAL_ROOT_FIELDS = {
            CHECKER.REQUEST_SCHEMA: frozenset(self.schema["properties"]),
            CHECKER.SUCCESS_SCHEMA: frozenset(self.success_schema["properties"]),
            CHECKER.FAILURE_SCHEMA: frozenset(self.failure_schema["properties"]),
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
        self.write_json(self.success_schema_path, self.success_schema)
        self.write_json(self.failure_schema_path, self.failure_schema)
        self.write_json(self.cases_path, self.cases)
        self.write_json(self.valid_path, self.request_document(tags=["x"]))
        self.write_json(self.invalid_path, self.request_document(count=-1))
        self.write_json(self.success_valid_path, self.result_document("model_turn.completed"))
        self.write_json(
            self.success_invalid_path, self.result_document("model_turn.completed", count=-1)
        )
        self.write_json(self.failure_valid_path, self.result_document("model_turn.failed"))
        self.write_json(
            self.failure_invalid_path, self.result_document("model_turn.failed", count=-1)
        )

    @staticmethod
    def write_json(path: Path, document: object) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(document, indent=2) + "\n", encoding="utf-8")

    @staticmethod
    def request_document(**changes: object) -> dict[str, object]:
        document: dict[str, object] = {
            "version": "v1",
            "kind": "model_turn.request",
            "request_id": "req-1",
            "model_alias": "model-1",
            "count": 1,
            "name": "ok",
            "instructions": [],
        }
        document.update(changes)
        return document

    @staticmethod
    def result_document(kind: str, **changes: object) -> dict[str, object]:
        document: dict[str, object] = {
            "version": "v1",
            "kind": kind,
            "request_id": "req-1",
            "count": 1,
            "name": "ok",
            "instructions": [],
        }
        document.update(changes)
        return document

    def check(self) -> list[str]:
        return CHECKER.check_contract(self.root)

    def test_committed_contract_corpus_passes(self) -> None:
        temporary_required_fields = CHECKER.CANONICAL_REQUIRED_FIELDS
        temporary_root_fields = CHECKER.CANONICAL_ROOT_FIELDS
        CHECKER.CANONICAL_REQUIRED_FIELDS = self.production_required_fields
        CHECKER.CANONICAL_ROOT_FIELDS = self.production_root_fields
        try:
            errors = CHECKER.check_contract(CHECKER.ROOT)
        finally:
            CHECKER.CANONICAL_REQUIRED_FIELDS = temporary_required_fields
            CHECKER.CANONICAL_ROOT_FIELDS = temporary_root_fields

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

    def test_schema_profile_requires_closed_objects_and_audited_patterns(self) -> None:
        python_only_pattern = r"^(?P<name>[a-z]+)$"
        self.assertIsNotNone(re.compile(python_only_pattern))
        cases = (
            (
                "open root object",
                lambda schema: schema.update({"additionalProperties": True}),
                "additionalProperties to be false",
            ),
            (
                "open nested object",
                lambda schema: schema["properties"]["details"].update(
                    {"additionalProperties": True}
                ),
                "additionalProperties to be false",
            ),
            (
                "Python-only pattern",
                lambda schema: schema["properties"]["name"].update(
                    {"pattern": python_only_pattern}
                ),
                "audited language-neutral expression",
            ),
        )
        for name, mutate, expected in cases:
            with self.subTest(name=name):
                schema = json.loads(json.dumps(self.schema))
                mutate(schema)
                self.write_json(self.schema_path, schema)

                errors = self.check()

                self.assertTrue(any(expected in error for error in errors), errors)
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

        self.assertTrue(
            any("unexpected file 'schema/unexpected.schema.json'" in error for error in errors)
        )
        self.assertTrue(any("request_id rules must remain identical" in error for error in errors))

    def test_canonical_schema_identity_and_framing_cannot_drift(self) -> None:
        cases = (
            (
                "mistyped ID",
                self.schema_path,
                lambda schema: schema.update({"$id": "urn:fastgate:model-turn:v1:requset"}),
                ("$id must be",),
            ),
            (
                "wrong-version ID",
                self.success_schema_path,
                lambda schema: schema.update({"$id": "urn:fastgate:model-turn:v2:success"}),
                ("$id must be",),
            ),
            (
                "duplicate ID",
                self.failure_schema_path,
                lambda schema: schema.update(
                    {"$id": CHECKER.CANONICAL_SCHEMA_METADATA[CHECKER.REQUEST_SCHEMA][0]}
                ),
                ("$id must be", "canonical $id values must be unique"),
            ),
            (
                "broadened version",
                self.success_schema_path,
                lambda schema: schema["properties"].update(
                    {"version": {"type": "string", "enum": ["v1", "v2"]}}
                ),
                ("version must have type 'string' and const 'v1'",),
            ),
            (
                "broadened kind",
                self.failure_schema_path,
                lambda schema: schema["properties"].update(
                    {
                        "kind": {
                            "type": "string",
                            "enum": ["model_turn.failed", "model_turn.cancelled"],
                        }
                    }
                ),
                ("kind must have type 'string' and const 'model_turn.failed'",),
            ),
            (
                "optional version",
                self.schema_path,
                lambda schema: schema["required"].remove("version"),
                ("version must be required",),
            ),
            (
                "optional kind",
                self.failure_schema_path,
                lambda schema: schema["required"].remove("kind"),
                ("kind must be required",),
            ),
            (
                "optional generic instructions",
                self.schema_path,
                lambda schema: schema["required"].remove("instructions"),
                ("canonical root required fields must remain exact",),
            ),
            (
                "removed generic instructions",
                self.schema_path,
                lambda schema: (
                    schema["required"].remove("instructions"),
                    schema["properties"].pop("instructions"),
                ),
                (
                    "canonical root property names must remain exact",
                    "canonical root required fields must remain exact",
                ),
            ),
            (
                "added optional root field",
                self.schema_path,
                lambda schema: schema["properties"].update(
                    {"temperature": {"type": "number"}}
                ),
                ("canonical root property names must remain exact",),
            ),
        )
        for name, path, mutate, expected_messages in cases:
            with self.subTest(name=name):
                original = json.loads(path.read_text(encoding="utf-8"))
                mutated = json.loads(json.dumps(original))
                mutate(mutated)
                self.write_json(path, mutated)
                try:
                    errors = self.check()

                    for expected in expected_messages:
                        self.assertTrue(any(expected in error for error in errors), errors)
                finally:
                    self.write_json(path, original)

    def test_model_alias_must_match_request_identifier_rule(self) -> None:
        request = json.loads(self.schema_path.read_text(encoding="utf-8"))
        request["properties"]["model_alias"]["maxLength"] = 129
        self.write_json(self.schema_path, request)

        errors = self.check()

        self.assertTrue(
            any("request_id and model_alias rules must remain identical" in error for error in errors),
            errors,
        )

    def test_canonical_schema_non_object_root_fails_without_traceback(self) -> None:
        self.write_json(
            self.schema_path,
            {
                "$schema": CHECKER.SCHEMA_URI,
                "$id": CHECKER.CANONICAL_SCHEMA_METADATA[CHECKER.REQUEST_SCHEMA][0],
                "type": "string",
            },
        )

        errors = self.check()

        self.assertTrue(any("canonical root must be an object" in error for error in errors), errors)

    def test_schema_parity_covers_identifier_usage_and_control_pattern(self) -> None:
        identifier = {"type": "string", "maxLength": 8}
        usage = {"type": "integer", "maximum": 10}
        request = {
            "properties": {
                "request_id": identifier,
                "model_alias": identifier,
                "instructions": {
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
            (self.request_document(count=-1), CHECKER.Violation("minimum", "/count")),
            (self.request_document(count=3), CHECKER.Violation("maximum", "/count")),
            (self.request_document(count=True), CHECKER.Violation("type", "/count")),
            (self.request_document(count=float("inf")), CHECKER.Violation("type", "/count")),
            (self.request_document(name=""), CHECKER.Violation("minLength", "/name")),
            (self.request_document(name="abcde"), CHECKER.Violation("maxLength", "/name")),
            (self.request_document(name="bad!"), CHECKER.Violation("pattern", "/name")),
            (self.request_document(name="ok\n"), CHECKER.Violation("pattern", "/name")),
            (
                self.request_document(tags=[]),
                CHECKER.Violation("minItems", "/tags"),
            ),
            (
                self.request_document(tags=["x", "x", "x"]),
                CHECKER.Violation("maxItems", "/tags"),
            ),
        )
        for document, expected in cases:
            with self.subTest(expected=expected):
                violations = CHECKER.validate_instance(document, self.schema)

                self.assertIn(expected, violations)

        self.assertEqual(CHECKER.validate_instance(self.request_document(count=1.0), self.schema), [])
        self.assertTrue(CHECKER.matches_type(10**10_000, "number"))

    def test_strict_json_classifies_normative_parse_failures(self) -> None:
        malformed = self.root / "malformed.json"
        cases = (
            ("duplicate key", b'{"field": 1, "field": 2}\n'),
            ("NaN", b'{"field": NaN}\n'),
            ("Infinity", b'{"field": Infinity}\n'),
            ("lone surrogate", b'{"field": "\\ud800"}\n'),
            ("invalid syntax", b'{"field": "unterminated}\n'),
            ("invalid UTF-8", b'{"field": "\xff"}\n'),
        )
        for name, content in cases:
            with self.subTest(name=name):
                malformed.write_bytes(content)

                with self.assertRaises(CHECKER.JSONDocumentError) as caught:
                    CHECKER.strict_json(malformed)
                self.assertIs(type(caught.exception), CHECKER.JSONDocumentError)

        malformed.write_text('{"field": 1e999}\n', encoding="utf-8")
        document = CHECKER.strict_json(malformed)
        self.assertEqual(document["field"], Decimal("1e999"))

        malformed.write_text('{"field": "\\ud83d\\ude00"}\n', encoding="utf-8")
        document = CHECKER.strict_json(malformed)
        self.assertEqual(document["field"], "😀")

    def test_exact_decimal_preserves_integer_and_maximum_semantics(self) -> None:
        fraction = Decimal("9007199254740990.5")
        self.assertFalse(CHECKER.matches_type(fraction, "integer"))
        violations = CHECKER.validate_instance(self.request_document(count=fraction), self.schema)
        self.assertIn(CHECKER.Violation("type", "/count"), violations)

        violations = CHECKER.validate_instance(
            self.request_document(count=Decimal("1e999")), self.schema
        )
        self.assertIn(CHECKER.Violation("maximum", "/count"), violations)

    def test_shared_surrogate_pair_fixture_preserves_escaped_source_spelling(self) -> None:
        fixture = (
            CHECKER.ROOT
            / CHECKER.CONTRACT_RELATIVE
            / "fixtures"
            / "valid"
            / "request-escaped-surrogate-pair.json"
        )
        raw = fixture.read_bytes()

        self.assertIn(b"\\ud83d\\ude00", raw)
        self.assertNotIn("😀".encode("utf-8"), raw)

    def test_schema_profile_rejects_enum_and_const_type_mismatches(self) -> None:
        schema = json.loads(json.dumps(self.schema))
        schema["properties"]["count"]["enum"] = ["one"]
        schema["properties"]["name"]["const"] = 1

        issues = CHECKER.schema_issues(schema)

        self.assertTrue(any("enum entries must match type" in issue for issue in issues), issues)
        self.assertTrue(any("const must match type" in issue for issue in issues), issues)

    def test_schema_profile_bounds_enum_uniqueness_work(self) -> None:
        schema = json.loads(json.dumps(self.schema))
        schema["properties"]["name"]["enum"] = [
            f"value-{index}" for index in range(CHECKER.MAX_ENUM_ITEMS + 1)
        ]

        issues = CHECKER.schema_issues(schema)

        self.assertTrue(any("enum must contain at most" in issue for issue in issues), issues)

    def test_invalid_json_fixture_can_expect_json_failure(self) -> None:
        self.invalid_path.write_text('{"count":\n', encoding="utf-8")
        self.cases["cases"][1]["expected"] = {"keyword": "json", "instance_path": ""}
        self.write_json(self.cases_path, self.cases)

        self.assertEqual(self.check(), [])

    def test_artifact_size_guard_cannot_satisfy_json_expectation(self) -> None:
        self.cases["cases"][1]["expected"] = {"keyword": "json", "instance_path": ""}
        self.write_json(self.cases_path, self.cases)
        oversized = self.request_document(
            details={"note": "x" * CHECKER.MAX_JSON_BYTES},
        )
        self.write_json(self.invalid_path, oversized)

        errors = self.check()

        self.assertTrue(any("fixture JSON document exceeds the checker size limit" in error for error in errors))

    def test_artifact_nesting_guard_cannot_satisfy_json_expectation(self) -> None:
        self.cases["cases"][1]["expected"] = {"keyword": "json", "instance_path": ""}
        self.write_json(self.cases_path, self.cases)
        depth = CHECKER.MAX_NESTING_DEPTH + 1
        self.invalid_path.write_text("[" * depth + "0" + "]" * depth + "\n", encoding="utf-8")

        errors = self.check()

        self.assertTrue(any("fixture JSON document exceeds the checker nesting limit" in error for error in errors))

    def test_exact_number_range_guard_cannot_satisfy_json_expectation(self) -> None:
        self.cases["cases"][1]["expected"] = {"keyword": "json", "instance_path": ""}
        self.write_json(self.cases_path, self.cases)
        raw = json.dumps(self.request_document()).replace(
            '"count": 1', '"count": 0e9999999999999999999'
        )
        self.invalid_path.write_text(raw + "\n", encoding="utf-8")

        with localcontext() as context:
            context.traps[InvalidOperation] = False
            errors = self.check()

        self.assertTrue(
            any("fixture JSON number exceeds the checker exact-number range" in error for error in errors)
        )

    def test_schema_nesting_guard_returns_bounded_issue(self) -> None:
        schema: dict[str, object] = {"type": "string"}
        for _ in range(CHECKER.MAX_NESTING_DEPTH + 1):
            schema = {"type": "array", "items": schema}

        issues = CHECKER.schema_issues(schema)

        self.assertTrue(any("schema exceeds the checker nesting limit" in issue for issue in issues))

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

    def test_orphan_diagnostic_escapes_control_characters_in_path(self) -> None:
        orphan = self.contract / "fixtures" / "valid" / "orphan\nINJECT\x1b[31m.json"
        self.write_json(orphan, {})

        errors = self.check()
        diagnostic = next(error for error in errors if "orphaned file" in error)

        self.assertIn(r"\n", diagnostic)
        self.assertIn(r"\x1b", diagnostic)
        self.assertNotIn("\n", diagnostic)
        self.assertNotIn("\x1b", diagnostic)

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
        self.write_json(self.invalid_path, self.request_document(count=-1, name=""))

        self.assertEqual(self.check(), [])

    def test_diagnostics_do_not_echo_fixture_values(self) -> None:
        sentinel = "private-prompt-sentinel"
        self.write_json(self.valid_path, self.request_document(name=sentinel))

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
