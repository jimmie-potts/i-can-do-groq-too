# TenantPlane

**Status:** Planned. No API, database schema, key format, or policy distribution exists yet.

TenantPlane is the management authority for a multi-tenant inference platform. FastGate remains the
data plane and consumes a bounded, versioned policy projection.

## Planned MVP

- organizations, projects, users, and roles;
- hashed, scoped API keys and revocation;
- model permissions and logical aliases;
- RPM, TPM, connection, and monthly token policy;
- idempotent usage-event ingestion and an append-only ledger;
- audit events; and
- an administrative CLI before an optional UI.

PostgreSQL is the default durable store. Redis is not added until a measured low-latency distributed
enforcement or cache need exists.

## Boundaries

- TenantPlane policy is authoritative; FastGate caches and enforces a versioned projection.
- Code Assist Harness local turn, deadline, output, and tool-call limits remain mandatory and may be
  stricter than platform quota.
- Harness transcripts are local workflow evidence, not billing records.
- Usage events may be delivered more than once and require stable idempotency keys.
- Partial streams, disconnects, and provider-versus-gateway token counts need an explicit accounting
  decision before anything is called billable usage.
