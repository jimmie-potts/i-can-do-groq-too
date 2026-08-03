#!/usr/bin/env python3
"""Validate the versioned FastGate model-turn contract without dependencies."""

from __future__ import annotations

import hashlib
import json
import math
import os
import re
import sys
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any, NamedTuple


ROOT = Path(__file__).resolve().parent.parent
CONTRACT_RELATIVE = Path("gateway/contracts/model-turn/v1")
REQUEST_SCHEMA = Path("schema/request.schema.json")
SUCCESS_SCHEMA = Path("schema/success.schema.json")
FAILURE_SCHEMA = Path("schema/failure.schema.json")
CANONICAL_SCHEMA_METADATA = {
    REQUEST_SCHEMA: ("urn:fastgate:model-turn:v1:request", "model_turn.request"),
    SUCCESS_SCHEMA: ("urn:fastgate:model-turn:v1:success", "model_turn.completed"),
    FAILURE_SCHEMA: ("urn:fastgate:model-turn:v1:failure", "model_turn.failed"),
}
CANONICAL_REQUIRED_FIELDS = {
    REQUEST_SCHEMA: frozenset(
        {
            "version",
            "kind",
            "request_id",
            "model_alias",
            "conversation",
            "instructions",
            "required_capabilities",
        }
    ),
    SUCCESS_SCHEMA: frozenset({"version", "kind", "request_id", "output_text"}),
    FAILURE_SCHEMA: frozenset({"version", "kind", "request_id", "error"}),
}
CANONICAL_ROOT_FIELDS = {
    REQUEST_SCHEMA: CANONICAL_REQUIRED_FIELDS[REQUEST_SCHEMA],
    SUCCESS_SCHEMA: CANONICAL_REQUIRED_FIELDS[SUCCESS_SCHEMA] | {"usage"},
    FAILURE_SCHEMA: CANONICAL_REQUIRED_FIELDS[FAILURE_SCHEMA] | {"usage"},
}
V1_SCHEMA_VALIDATION_FINGERPRINTS = {
    REQUEST_SCHEMA: "e08098dd1c8ef35c63b177720145caeb927304added8ec5b9e993c49bbcd0fb9",
    SUCCESS_SCHEMA: "31d2b6f7892a4b67c7122bace115d2614535f1e3745a4f3905d27f7c164d73f1",
    FAILURE_SCHEMA: "1be52b33e19688e818175dc8ae4e336be9fb6ba56e5f0d7885864d6913856686",
}
CANONICAL_SCHEMAS = tuple(CANONICAL_SCHEMA_METADATA)
SCHEMA_URI = "https://json-schema.org/draft/2020-12/schema"
MAX_JSON_BYTES = 1_000_000
MAX_NESTING_DEPTH = 128
MAX_ENUM_ITEMS = 64
MAX_ERRORS = 50
MAX_VIOLATIONS = 25
MAX_PATH_LENGTH = 240
CASE_NAME = re.compile(r"[a-z0-9](?:[a-z0-9-]{0,78}[a-z0-9])?")
JSON_POINTER = re.compile(r"(?:/(?:[^~/]|~[01])*)*")
IDENTIFIER_PATTERN = r"^[A-Za-z0-9][A-Za-z0-9._:-]*(?![\s\S])"
CONTROL_SAFE_PATTERN = r"^[^\u0000-\u001F\u007F-\u009F\u2028\u2029]+(?![\s\S])"
AUDITED_PATTERN_MATCHERS = {
    pattern: re.compile(pattern) for pattern in (IDENTIFIER_PATTERN, CONTROL_SAFE_PATTERN)
}
SUPPORTED_KEYWORDS = {
    "$schema", "$id", "title", "description", "type", "properties", "required",
    "additionalProperties", "items", "minItems", "maxItems", "minLength", "maxLength",
    "minimum", "maximum", "enum", "const", "pattern",
}
JSON_TYPES = {"object", "array", "string", "integer", "number", "boolean", "null"}
SCHEMA_ANNOTATIONS = frozenset({"title", "description"})
SCHEMA_UNORDERED_ARRAYS = frozenset({"required", "enum"})


class JSONDocumentError(ValueError):
    """A bounded, content-free strict JSON parsing failure."""


class JSONArtifactError(JSONDocumentError):
    """A repository-artifact failure that is not a protocol JSON violation."""


class Violation(NamedTuple):
    """One machine-comparable instance-validation failure."""

    keyword: str
    instance_path: str


def strict_json(path: Path) -> Any:
    """Load bounded strict JSON with unique keys, exact numbers, and Unicode scalars."""
    try:
        with path.open("rb") as source:
            raw = source.read(MAX_JSON_BYTES + 1)
    except OSError as error:
        raise JSONArtifactError("unable to read JSON document") from error
    if len(raw) > MAX_JSON_BYTES:
        raise JSONArtifactError("JSON document exceeds the checker size limit")
    try:
        text = raw.decode("utf-8")
    except UnicodeDecodeError as error:
        raise JSONDocumentError("JSON document is not UTF-8") from error

    def unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in pairs:
            if key in result:
                raise JSONDocumentError("JSON document contains a duplicate object key")
            result[key] = value
        return result

    def reject_constant(_value: str) -> None:
        raise JSONDocumentError("JSON document contains a non-finite number")

    def exact_decimal(value: str) -> Decimal:
        try:
            parsed = Decimal(value)
        except InvalidOperation as error:
            raise JSONArtifactError("JSON number exceeds the checker exact-number range") from error
        if not parsed.is_finite():
            raise JSONArtifactError("JSON number exceeds the checker exact-number range")
        return parsed

    try:
        document = json.loads(
            text, object_pairs_hook=unique_object, parse_constant=reject_constant,
            parse_float=exact_decimal, parse_int=exact_decimal,
        )
        require_unicode_scalars(document)
        return document
    except JSONDocumentError:
        raise
    except json.JSONDecodeError as error:
        raise JSONDocumentError(
            f"invalid JSON syntax at line {error.lineno}, column {error.colno}"
        ) from error
    except RecursionError as error:
        raise JSONArtifactError("JSON document exceeds the checker nesting limit") from error
    except ValueError as error:
        raise JSONArtifactError("JSON document exceeds checker implementation limits") from error


def require_unicode_scalars(document: Any) -> None:
    """Reject decoded object keys or values containing an unpaired surrogate."""
    pending = [(document, 0)]
    while pending:
        value, depth = pending.pop()
        if depth > MAX_NESTING_DEPTH:
            raise JSONArtifactError("JSON document exceeds the checker nesting limit")
        if isinstance(value, str):
            if any(0xD800 <= ord(character) <= 0xDFFF for character in value):
                raise JSONDocumentError("JSON document contains a lone Unicode surrogate")
        elif isinstance(value, list):
            pending.extend((item, depth + 1) for item in value)
        elif isinstance(value, dict):
            pending.extend((key, depth) for key in value)
            pending.extend((item, depth + 1) for item in value.values())


def matches_type(value: Any, schema_type: str) -> bool:
    """Apply JSON types while keeping Python bool distinct from integer."""
    if schema_type == "object":
        return isinstance(value, dict)
    if schema_type == "array":
        return isinstance(value, list)
    if schema_type == "string":
        return isinstance(value, str)
    if schema_type == "integer":
        return not isinstance(value, bool) and (
            isinstance(value, int)
            or isinstance(value, Decimal) and value.is_finite() and value == value.to_integral_value()
            or isinstance(value, float) and math.isfinite(value) and value.is_integer()
        )
    if schema_type == "number":
        return (
            isinstance(value, int) and not isinstance(value, bool)
            or isinstance(value, Decimal) and value.is_finite()
            or isinstance(value, float) and math.isfinite(value)
        )
    if schema_type == "boolean":
        return isinstance(value, bool)
    return value is None


def json_equal(left: Any, right: Any) -> bool:
    """Compare JSON values without treating booleans as numbers."""
    if isinstance(left, bool) or isinstance(right, bool):
        return type(left) is type(right) and left == right
    numeric = (int, float, Decimal)
    if isinstance(left, numeric) and isinstance(right, numeric):
        return left == right
    if type(left) is not type(right):
        return False
    if isinstance(left, list):
        return len(left) == len(right) and all(json_equal(a, b) for a, b in zip(left, right))
    if isinstance(left, dict):
        return left.keys() == right.keys() and all(json_equal(left[key], right[key]) for key in left)
    return left == right


def pointer_join(base: str, token: str | int) -> str:
    """Append one escaped token to an RFC 6901 JSON pointer."""
    escaped = str(token).replace("~", "~0").replace("/", "~1")
    return f"{base}/{escaped}"


def schema_validation_fingerprint(schema: dict[str, Any]) -> str:
    """Fingerprint all validation-affecting schema content, excluding annotations."""

    def number_key(value: int | Decimal) -> str:
        decimal = value if isinstance(value, Decimal) else Decimal(value)
        if decimal == 0:
            return "0"
        sign, digits_tuple, exponent = decimal.as_tuple()
        digits = list(digits_tuple)
        while digits and digits[-1] == 0:
            digits.pop()
            exponent += 1
        return f"{sign}:{''.join(str(digit) for digit in digits)}:{exponent}"

    def encode(value: Any, *, schema_node: bool = False) -> Any:
        if isinstance(value, dict):
            entries: list[list[Any]] = []
            for key in sorted(value):
                if schema_node and key in SCHEMA_ANNOTATIONS:
                    continue
                child = value[key]
                if schema_node and key == "properties":
                    encoded = [
                        "object",
                        [[name, encode(child[name], schema_node=True)] for name in sorted(child)],
                    ]
                elif schema_node and key == "items":
                    encoded = encode(child, schema_node=True)
                elif schema_node and key in SCHEMA_UNORDERED_ARRAYS:
                    encoded_items = [encode(item) for item in child]
                    encoded_items.sort(
                        key=lambda item: json.dumps(
                            item, ensure_ascii=True, separators=(",", ":")
                        )
                    )
                    encoded = ["array", encoded_items]
                else:
                    encoded = encode(child)
                entries.append([key, encoded])
            return ["object", entries]
        if isinstance(value, list):
            return ["array", [encode(item) for item in value]]
        if isinstance(value, str):
            return ["string", value]
        if isinstance(value, bool):
            return ["boolean", value]
        if isinstance(value, (int, Decimal)):
            return ["number", number_key(value)]
        if value is None:
            return ["null"]
        raise TypeError(f"unsupported schema value type: {type(value).__name__}")

    canonical = json.dumps(
        encode(schema, schema_node=True), ensure_ascii=True, separators=(",", ":")
    )
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()


def schema_issues(schema: Any) -> list[str]:
    """Validate the closed JSON Schema 2020-12 profile used by this contract."""
    issues: list[str] = []

    def add(path: str, message: str) -> None:
        if len(issues) < MAX_ERRORS:
            bounded_path = path
            if len(bounded_path) > MAX_PATH_LENGTH:
                bounded_path = bounded_path[: MAX_PATH_LENGTH - 3] + "..."
            issues.append(f"at {bounded_path!r}: {message}"[:500])

    def natural(value: Any) -> bool:
        return matches_type(value, "integer") and value >= 0

    def number(value: Any) -> bool:
        return (
            isinstance(value, int) and not isinstance(value, bool)
            or isinstance(value, Decimal) and value.is_finite()
            or isinstance(value, float) and math.isfinite(value)
        )

    def bounds(node: dict[str, Any], path: str, low: str, high: str) -> None:
        for keyword in (low, high):
            if keyword in node and not natural(node[keyword]):
                add(path, f"{keyword} must be a non-negative integer")
        if natural(node.get(low)) and natural(node.get(high)) and node[low] > node[high]:
            add(path, f"{low} must not exceed {high}")

    def walk(node: Any, path: str, root: bool = False, depth: int = 0) -> None:
        if depth > MAX_NESTING_DEPTH:
            add(path, "schema exceeds the checker nesting limit")
            return
        if not isinstance(node, dict):
            add(path, "a schema node must be an object")
            return
        for keyword in sorted(node.keys() - SUPPORTED_KEYWORDS):
            add(path, f"unsupported keyword {keyword!r}")
        if root:
            if node.get("$schema") != SCHEMA_URI:
                add(path, f"$schema must be {SCHEMA_URI!r}")
            if not isinstance(node.get("$id"), str) or not node.get("$id"):
                add(path, "$id must be a non-empty string")
        elif "$schema" in node or "$id" in node:
            add(path, "$schema and $id are root-only in this profile")
        for annotation in ("title", "description"):
            if annotation in node and not isinstance(node[annotation], str):
                add(path, f"{annotation} must be a string")

        schema_type = node.get("type")
        valid_type = isinstance(schema_type, str) and schema_type in JSON_TYPES
        if not valid_type:
            add(path, "type must be one supported JSON type string")
        keyword_groups = (
            ({"properties", "required", "additionalProperties"}, {"object"}, "object"),
            ({"items", "minItems", "maxItems"}, {"array"}, "array"),
            ({"minLength", "maxLength", "pattern"}, {"string"}, "string"),
            ({"minimum", "maximum"}, {"integer", "number"}, "numeric"),
        )
        for keywords, allowed_types, label in keyword_groups:
            if keywords & node.keys() and (not isinstance(schema_type, str) or schema_type not in allowed_types):
                add(path, f"{label} keywords do not match type")

        if schema_type == "object":
            properties, required = node.get("properties"), node.get("required")
            if not isinstance(properties, dict):
                add(path, "object schemas require a properties object")
                properties = {}
            if not isinstance(required, list) or any(not isinstance(item, str) for item in required):
                add(path, "object schemas require a required string array")
                required = []
            elif len(required) != len(set(required)):
                add(path, "required entries must be unique")
            if node.get("additionalProperties") is not False:
                add(path, "object schemas require additionalProperties to be false")
            for name, child in properties.items():
                walk(child, pointer_join(path, name), depth=depth + 1)
            if any(name not in properties for name in required):
                add(path, "every required entry must name a property")
        elif schema_type == "array":
            if "items" not in node:
                add(path, "array schemas require items")
            else:
                walk(node["items"], pointer_join(path, "items"), depth=depth + 1)
            bounds(node, path, "minItems", "maxItems")
        elif schema_type == "string":
            bounds(node, path, "minLength", "maxLength")
            if "pattern" in node:
                if not isinstance(node["pattern"], str):
                    add(path, "pattern must be a string")
                elif node["pattern"] not in AUDITED_PATTERN_MATCHERS:
                    add(path, "pattern must be an audited language-neutral expression")
        elif schema_type in ("integer", "number"):
            for keyword in ("minimum", "maximum"):
                if keyword in node and not number(node[keyword]):
                    add(path, f"{keyword} must be a finite number")
            if number(node.get("minimum")) and number(node.get("maximum")):
                if node["minimum"] > node["maximum"]:
                    add(path, "minimum must not exceed maximum")

        enum = node.get("enum")
        if "enum" in node:
            if not isinstance(enum, list) or not enum:
                add(path, "enum must be a non-empty array")
            elif len(enum) > MAX_ENUM_ITEMS:
                add(path, f"enum must contain at most {MAX_ENUM_ITEMS} entries")
            elif any(json_equal(a, b) for index, a in enumerate(enum) for b in enum[index + 1 :]):
                add(path, "enum entries must be unique")
            elif valid_type and any(not matches_type(item, schema_type) for item in enum):
                add(path, "enum entries must match type")
        if "const" in node and valid_type:
            if not matches_type(node["const"], schema_type):
                add(path, "const must match type")
        if "enum" in node and "const" in node:
            add(path, "enum and const must not be combined in this profile")

    walk(schema, "", root=True)
    return issues


def validate_instance(instance: Any, schema: dict[str, Any]) -> list[Violation]:
    """Validate one instance against the supported profile."""
    violations: list[Violation] = []

    def add(keyword: str, path: str) -> None:
        if len(violations) < MAX_VIOLATIONS:
            violations.append(Violation(keyword, path))

    def walk(value: Any, node: dict[str, Any], path: str) -> None:
        if len(violations) >= MAX_VIOLATIONS:
            return
        schema_type = node["type"]
        if not matches_type(value, schema_type):
            add("type", path)
            return
        if "enum" in node and not any(json_equal(value, item) for item in node["enum"]):
            add("enum", path)
        if "const" in node and not json_equal(value, node["const"]):
            add("const", path)
        if schema_type == "object":
            properties = node["properties"]
            for name in node["required"]:
                if name not in value:
                    add("required", path)
            for name, child in value.items():
                if name in properties:
                    walk(child, properties[name], pointer_join(path, name))
                else:
                    add("additionalProperties", path)
        elif schema_type == "array":
            for keyword, fails in (
                ("minItems", lambda: len(value) < node["minItems"]),
                ("maxItems", lambda: len(value) > node["maxItems"]),
            ):
                if keyword in node and fails():
                    add(keyword, path)
            for index, item in enumerate(value):
                walk(item, node["items"], pointer_join(path, index))
        elif schema_type == "string":
            checks = (
                ("minLength", lambda: len(value) < node["minLength"]),
                ("maxLength", lambda: len(value) > node["maxLength"]),
            )
            for keyword, fails in checks:
                if keyword in node and fails():
                    add(keyword, path)
            if "pattern" in node:
                matcher = AUDITED_PATTERN_MATCHERS.get(node["pattern"])
                if matcher is None or matcher.search(value) is None:
                    add("pattern", path)
        elif schema_type in {"integer", "number"}:
            if "minimum" in node and value < node["minimum"]:
                add("minimum", path)
            if "maximum" in node and value > node["maximum"]:
                add("maximum", path)

    walk(instance, schema, "")
    return violations


def safe_file(contract: Path, raw_path: Any, prefix: Path) -> tuple[Path | None, str | None]:
    """Resolve one manifest path without allowing aliases, escapes, or symlinks."""
    if not isinstance(raw_path, str) or not raw_path or len(raw_path) > MAX_PATH_LENGTH:
        return None, "path must be a bounded non-empty string"
    relative = Path(raw_path)
    if relative.is_absolute() or relative.as_posix() != raw_path or ".." in relative.parts:
        return None, "path must be a normalized contract-relative path"
    try:
        relative.relative_to(prefix)
    except ValueError:
        return None, f"path must be under {prefix.as_posix()}"
    candidate, current = contract / relative, contract
    for part in relative.parts:
        current /= part
        if current.is_symlink():
            return None, "path must not contain a symlink"
    if not candidate.is_file():
        return None, "path must name an existing regular file"
    return candidate, None


def load_canonical_schemas(
    contract: Path, errors: list[str]
) -> dict[Path, dict[str, Any] | None]:
    """Load every canonical schema independently and reject schema-inventory drift."""
    schemas: dict[Path, dict[str, Any] | None] = {}
    for relative in CANONICAL_SCHEMAS:
        path, path_error = safe_file(contract, relative.as_posix(), Path("schema"))
        if path_error:
            errors.append(f"schema {relative}: {path_error}")
            continue
        assert path is not None
        try:
            loaded = strict_json(path)
        except JSONDocumentError as error:
            errors.append(f"schema {relative}: {error}")
            schemas[path] = None
            continue
        issues = schema_issues(loaded)
        errors.extend(f"schema {relative} {issue}" for issue in issues)
        schemas[path] = None if issues else loaded

    discovered: set[Path] = set()
    schema_root = contract / "schema"
    if schema_root.is_dir():
        for directory, directory_names, file_names in os.walk(schema_root, followlinks=False):
            base = Path(directory)
            for name in directory_names:
                if (base / name).is_symlink():
                    errors.append("contract schemas: symlinked directories are not allowed")
            for name in file_names:
                entry = base / name
                relative = entry.relative_to(contract)
                if entry.is_symlink():
                    errors.append("contract schemas: symlinked files are not allowed")
                else:
                    discovered.add(relative)
    for unexpected in sorted(discovered - set(CANONICAL_SCHEMAS)):
        errors.append(f"contract schemas: unexpected file {unexpected.as_posix()!r}")

    check_canonical_schema_invariants(contract, schemas, errors)
    check_schema_parity(contract, schemas, errors)
    return schemas


def check_canonical_schema_invariants(
    contract: Path,
    schemas: dict[Path, dict[str, Any] | None],
    errors: list[str],
) -> None:
    """Pin each canonical file to its complete v1 contract and readable framing rules."""
    if set(V1_SCHEMA_VALIDATION_FINGERPRINTS) != set(CANONICAL_SCHEMA_METADATA):
        errors.append(
            "contract schemas: v1 validation fingerprint inventory must match canonical schemas"
        )

    loaded: list[tuple[Path, dict[str, Any], str, str]] = []
    for relative, (expected_id, expected_kind) in CANONICAL_SCHEMA_METADATA.items():
        schema = schemas.get(contract / relative)
        if not isinstance(schema, dict):
            return
        loaded.append((relative, schema, expected_id, expected_kind))

    actual_ids = [schema["$id"] for _relative, schema, _expected_id, _kind in loaded]
    if len(set(actual_ids)) != len(actual_ids):
        errors.append("contract schemas: canonical $id values must be unique")

    for relative, schema, expected_id, expected_kind in loaded:
        if schema["$id"] != expected_id:
            errors.append(f"schema {relative}: $id must be {expected_id!r}")
        expected_fingerprint = V1_SCHEMA_VALIDATION_FINGERPRINTS.get(relative)
        if schema_validation_fingerprint(schema) != expected_fingerprint:
            errors.append(
                f"schema {relative}: validation rules must match the frozen v1 contract"
            )
        required = schema.get("required")
        properties = schema.get("properties")
        if (
            schema.get("type") != "object"
            or not isinstance(required, list)
            or not isinstance(properties, dict)
        ):
            errors.append(
                f"schema {relative}: canonical root must be an object with properties and required"
            )
            continue
        if set(properties) != CANONICAL_ROOT_FIELDS[relative]:
            errors.append(
                f"schema {relative}: canonical root property names must remain exact"
            )
        if set(required) != CANONICAL_REQUIRED_FIELDS[relative]:
            errors.append(
                f"schema {relative}: canonical root required fields must remain exact"
            )
        for field, expected_const in (("version", "v1"), ("kind", expected_kind)):
            rule = properties.get(field)
            if field not in required:
                errors.append(f"schema {relative}: {field} must be required")
            if (
                not isinstance(rule, dict)
                or rule.get("type") != "string"
                or rule.get("const") != expected_const
            ):
                errors.append(
                    f"schema {relative}: {field} must have type 'string' "
                    f"and const {expected_const!r}"
                )


def check_schema_parity(
    contract: Path,
    schemas: dict[Path, dict[str, Any] | None],
    errors: list[str],
) -> None:
    """Keep deliberately duplicated public field rules identical across documents."""
    request = schemas.get(contract / REQUEST_SCHEMA)
    success = schemas.get(contract / SUCCESS_SCHEMA)
    failure = schemas.get(contract / FAILURE_SCHEMA)
    if not all(isinstance(schema, dict) for schema in (request, success, failure)):
        return

    def nested(schema: dict[str, Any], *keys: str) -> Any:
        value: Any = schema
        for key in keys:
            if not isinstance(value, dict):
                return None
            value = value.get(key)
        return value

    def require_equal(label: str, values: tuple[Any, ...]) -> None:
        if all(value is None for value in values):
            return
        if any(value is None for value in values) or not all(
            json_equal(values[0], value) for value in values[1:]
        ):
            errors.append(f"contract schemas: {label} rules must remain identical")

    require_equal(
        "request_id",
        tuple(nested(schema, "properties", "request_id") for schema in (request, success, failure)),
    )
    require_equal(
        "request_id and model_alias",
        (
            nested(request, "properties", "request_id"),
            nested(request, "properties", "model_alias"),
        ),
    )
    require_equal(
        "usage",
        (
            nested(success, "properties", "usage"),
            nested(failure, "properties", "usage"),
        ),
    )
    require_equal(
        "control-safe pattern",
        (
            nested(
                request,
                "properties",
                "instructions",
                "items",
                "properties",
                "source",
                "pattern",
            ),
            nested(failure, "properties", "error", "properties", "message", "pattern"),
        ),
    )


def check_contract(repository_root: Path = ROOT) -> list[str]:
    """Validate schema syntax, manifest integrity, and the complete fixture corpus."""
    contract = repository_root / CONTRACT_RELATIVE
    errors: list[str] = []
    try:
        manifest = strict_json(contract / "fixtures" / "cases.json")
    except JSONDocumentError as error:
        return [f"contract cases: {error}"]
    if not isinstance(manifest, dict) or set(manifest) != {"version", "cases"}:
        return ["contract cases: root must contain only version and cases"]
    if manifest["version"] != "v1":
        errors.append("contract cases: version must be v1")
    cases = manifest["cases"]
    if not isinstance(cases, list) or not cases:
        return errors + ["contract cases: cases must be a non-empty array"]

    names: set[str] = set()
    fixture_paths: set[Path] = set()
    schemas = load_canonical_schemas(contract, errors)
    coverage = {relative: set() for relative in CANONICAL_SCHEMAS}
    for index, case in enumerate(cases):
        label = f"case[{index}]"
        required = {"name", "schema", "fixture", "valid"}
        allowed = required | {"meaning", "expected"}
        if not isinstance(case, dict) or not required <= set(case) or not set(case) <= allowed:
            errors.append(f"{label}: case fields are missing or unsupported")
            continue
        name = case["name"]
        if not isinstance(name, str) or CASE_NAME.fullmatch(name) is None:
            errors.append(f"{label}: name must be a bounded lowercase slug")
        else:
            label = name
            if name in names:
                errors.append(f"{label}: duplicate case name")
            names.add(name)
        if type(case["valid"]) is not bool:
            errors.append(f"{label}: valid must be boolean")
            continue
        if "meaning" in case and (not isinstance(case["meaning"], str) or not case["meaning"]):
            errors.append(f"{label}: meaning must be a non-empty string when present")

        expected: Violation | None = None
        expected_data = case.get("expected")
        if case["valid"] and "expected" in case:
            errors.append(f"{label}: valid cases must omit expected")
        elif not case["valid"]:
            if not isinstance(expected_data, dict) or set(expected_data) != {"keyword", "instance_path"}:
                errors.append(f"{label}: invalid cases require strict expected metadata")
            elif (
                not isinstance(expected_data["keyword"], str)
                or expected_data["keyword"] not in SUPPORTED_KEYWORDS | {"json"}
                or not isinstance(expected_data["instance_path"], str)
                or JSON_POINTER.fullmatch(expected_data["instance_path"]) is None
            ):
                errors.append(f"{label}: expected keyword or instance_path is invalid")
            else:
                expected = Violation(expected_data["keyword"], expected_data["instance_path"])

        schema_path, schema_error = safe_file(contract, case["schema"], Path("schema"))
        fixture_prefix = Path("fixtures/valid" if case["valid"] else "fixtures/invalid")
        fixture_path, fixture_error = safe_file(contract, case["fixture"], fixture_prefix)
        if schema_error:
            errors.append(f"{label}: schema {schema_error}")
        if fixture_error:
            errors.append(f"{label}: fixture {fixture_error}")
        if schema_path is None or fixture_path is None:
            continue
        relative_schema = schema_path.relative_to(contract)
        if relative_schema not in coverage:
            errors.append(f"{label}: schema must be one of the three canonical schemas")
            continue
        coverage[relative_schema].add(case["valid"])
        relative_fixture = fixture_path.relative_to(contract)
        if relative_fixture in fixture_paths:
            errors.append(f"{label}: duplicate fixture path")
        fixture_paths.add(relative_fixture)

        schema = schemas.get(schema_path)
        if schema is None:
            continue
        try:
            fixture = strict_json(fixture_path)
        except JSONArtifactError as error:
            errors.append(f"{label}: fixture {error}")
            continue
        except JSONDocumentError:
            violations = [Violation("json", "")]
        else:
            violations = validate_instance(fixture, schema)
        if case["valid"] and violations:
            first = violations[0]
            errors.append(
                f"{label}: expected valid but failed {first.keyword} at {first.instance_path!r}"
            )
        elif not case["valid"] and not violations:
            errors.append(f"{label}: expected invalid but was accepted")
        elif not case["valid"] and expected is not None and expected not in violations:
            first = violations[0]
            errors.append(
                f"{label}: intended rule mismatch; observed {first.keyword} "
                f"at {first.instance_path!r}"
            )

    discovered: set[Path] = set()
    fixture_root = contract / "fixtures"
    if fixture_root.is_dir():
        for directory, directory_names, file_names in os.walk(fixture_root, followlinks=False):
            base = Path(directory)
            for name in directory_names:
                if (base / name).is_symlink():
                    errors.append("contract fixtures: symlinked directories are not allowed")
            for name in file_names:
                entry = base / name
                relative = entry.relative_to(contract)
                if entry.is_symlink():
                    errors.append("contract fixtures: symlinked files are not allowed")
                elif relative != Path("fixtures/cases.json"):
                    discovered.add(relative)
    for orphan in sorted(discovered - fixture_paths):
        errors.append(f"contract fixtures: orphaned file {orphan.as_posix()!r}")
    for relative, outcomes in coverage.items():
        if True not in outcomes:
            errors.append(f"contract cases: {relative} requires at least one valid fixture")
        if False not in outcomes:
            errors.append(f"contract cases: {relative} requires at least one invalid fixture")
    return [message[:500] for message in errors[:MAX_ERRORS]]


def main(arguments: list[str] | None = None) -> int:
    """Run the offline contract check and print bounded diagnostics."""
    if (sys.argv[1:] if arguments is None else arguments):
        print("Usage: python3 scripts/check_contract.py", file=sys.stderr)
        return 2
    errors = check_contract()
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print("FastGate model-turn v1 contract fixtures passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
