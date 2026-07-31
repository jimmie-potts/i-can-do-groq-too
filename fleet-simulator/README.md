# FleetSim

**Status:** Planned. No simulator, scenario schema, or scheduler exists yet.

FleetSim is a deterministic discrete-event simulator for inference capacity and scheduling. It
lets us compare policies without real accelerators or production traffic.

## Planned MVP

- Model regions, clusters, racks, nodes, model deployments, request classes, reservations, queues,
  network latency, and failures.
- Implement simple schedulers independently: first fit, best fit, least utilized, latency weighted,
  reservation aware, and SLO/fairness scoring.
- Replay the same versioned workload and random seed across strategies.
- Report placement, queueing, rejection, latency, utilization, fairness, and capacity fragmentation.

## Boundaries

- Scheduling logic is ordinary deterministic Go, not LangChain or LangGraph.
- Inputs are synthetic or aggregated and carry units and provenance.
- LatencyLab measurements may calibrate scenarios, but prompts and repository content do not enter
  the simulator.
- FleetSim is advisory and offline. It does not become a live FastGate router without a new reviewed
  architecture boundary.
