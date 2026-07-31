# User-story guidelines

These instructions apply under `user-stories/` and extend the root `AGENTS.md`.

## Story size

A story delivers one observable outcome with one primary concept and at least one meaningful failure
path. Split transport, domain, provider, streaming, cancellation, timeout, backpressure, telemetry,
and live-provider behavior unless combining them is necessary to prove one invariant.

An implementation-ready story must name:

- status, milestone, dependencies, lesson, and review priority;
- user value;
- exact scope and locked behavior;
- acceptance criteria;
- deterministic validation;
- documentation impact;
- exclusions; and
- a human review checkpoint with production path, test/failure path, invariant, and deferrals.

Do not promote a roadmap outcome merely to reserve an identifier. Its upstream contracts must exist
or the story must be an explicit, bounded design spike.

## Status honesty

- `Planned` means no implementation evidence is claimed.
- `In progress` means at least one acceptance criterion or review step remains.
- `Blocked` names the exact dependency or decision.
- `Done` requires the full offline gate and an implementation-backed lesson.

Documentation-only stories still require link and policy validation. A planned API, adapter, or
service is never evidence that it works.

## Notes

Notes capture observed constraints, failed approaches, validation evidence, and follow-up questions.
They do not change architecture silently. Update an ADR when a decision changes and the story index
when delivery status changes.
