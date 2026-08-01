# Lesson guidelines

These instructions apply under `docs/lessons/` and extend the root `AGENTS.md`.

## Status and evidence

Match the linked story status. Planned lessons may explain concepts and use clearly labeled
pseudocode. Never call a package, endpoint, adapter, test, metric, or behavior implemented until the
repository contains it and validation proves it.

When code exists, replace speculative paths with exact relative links and focused excerpts copied
from the implementation. Include one important production path and one meaningful failure, test,
cancellation, timeout, or cleanup path. Explain why each part exists rather than paraphrasing syntax.

## Audience

Assume the reader can program but has not built inference infrastructure. Define the junior-level
foundation before relying on terms such as port, adapter, SSE, backpressure, idempotency,
reconciliation, TTFT, control plane, or KV cache. Include a small example and at least one common
misconception.

## Required ending

Every lesson ends with:

- practical exercises;
- key takeaways;
- a local glossary;
- exactly three teach-back questions; and
- further reading that prefers repository docs and official references.

## Production comparisons

Production tools are comparisons, not implicit dependencies. Describe capability, benefit,
operational cost, and the signal that would justify graduation. Do not add a tool merely because it
appears in a lesson.

## Visual evidence

Create a required visual companion only after source, tests, and written lesson stabilize. Use the
presentation workflow available in the current environment, render every slide, inspect the output,
and record the exact validation in the story note.
