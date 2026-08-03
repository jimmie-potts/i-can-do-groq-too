# Glossary

## Agent loop

The harness-owned cycle that builds model input, consumes model events, validates tool requests,
applies policy, records effects, and chooses the next or terminal state.

## Backpressure

The mechanism that prevents a fast producer from creating unbounded memory or work when a consumer
is slower.

## Capability registry

A reviewed description of behavior a provider actually supports. It is used to reject, emulate, or
reroute a request explicitly rather than silently ignoring fields.

## Control plane

The authority that manages identity, policy, desired state, quota, budget, and configuration. In
this repository, TenantPlane is the primary inference control plane.

## Data plane

The latency-sensitive path that carries inference requests and responses. FastGate is the primary
data-plane component.

## Deterministic fake

A programmable implementation of a real port that verifies exact inputs and emits controlled
outputs, failures, delays, and cancellation points without network or wall-clock uncertainty.

## FastGate model-turn protocol

The versioned, FastGate-owned northbound wire contract for one bounded model interaction. It carries
client-visible request, result, failure, and later streaming meaning without copying a provider API.
It is not the harness's agent loop or authority to choose another workflow turn; Code Assist Harness
uses a separate adapter to translate between those concerns. ICGT-006 commits the exact v1 schema,
fixtures, mapping, strict parse profile, and offline validator.

## KV cache

Model-runtime tensors that retain attention keys and values for already processed tokens. Hosted
providers own their internal KV cache; prompt caching may expose benefits or metrics without raw
tensor control.

## Model alias

A logical purpose such as `review-model` that can map to a reviewed provider/model route without
putting vendor identifiers into workflow code.

## Provider adapter

The boundary translator between provider-specific configuration and wire events and a project-owned
request, event, failure, cancellation, and cleanup contract.

## Reconciliation

An idempotent control loop that repeatedly moves observed state toward declared desired state.

## Time to first token (TTFT)

Elapsed time from request admission to the first accepted output token or text fragment. It should
be separated from queue time, gateway overhead, and total response time when possible.

## Usage event

An append-only observation used for metering or audit. Delivery may be duplicated; ingestion must
be idempotent and billing semantics must cover partial streams and disconnects explicitly.
