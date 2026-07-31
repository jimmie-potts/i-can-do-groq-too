# LatencyLab

**Status:** Planned. No load generator, proxy, workload schema, or dashboard exists yet.

LatencyLab is an inference-aware workload and failure lab. It measures FastGate, direct providers,
or another reviewed endpoint without becoming the source of production routing policy.

## Planned MVP

- A Go CLI reads a versioned YAML workload.
- A load generator controls concurrency, prompt profile, output expectation, and streaming mode.
- A fault proxy can add latency, rate limits, disconnects, and bounded malformed behavior.
- Results include P50/P95/P99 TTFT and total latency, stream duration, successes, categorized
  failures, retries, dropped work, and bounded token/cost observations.
- Output is available as terminal summary and machine-readable JSON. Prometheus and an optional
  TypeScript dashboard come only after the result contract is stable.

## Boundaries

- LatencyLab measures operational behavior; Code Assist Harness evaluates coding-task correctness.
- Test prompts and outputs are redacted or synthetic by default.
- The lab does not automatically change live FastGate routing.
- Fault injection is explicit, reproducible, and isolated from default repository checks.

LatencyLab begins only after FastGate has a stable, observable request path.
