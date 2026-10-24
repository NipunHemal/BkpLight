# API and interface design

**Document status:** Architecture v0.1 approved under H1 on 2026-10-24; no implementation.

## Approval record

The human approved architecture v0.1 and P01-P09 on 2026-10-24; see [H1 evidence in ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md). Proposal/default wording below records the approved design rationale, not a pending repeat approval. Unknown deployment facts remain unresolved; H2/H3/H4 are not granted.

## Purpose

Specify the operator interface, configuration contract, internal adapters, storage commit protocol and outgoing event payloads.

## Interface inventory

Propose a local CLI and versioned HTTP/JSON over `/run/bkplight/control.sock`, restricted to the daemon's OS account/group. Commands run through `docker compose exec` or inside the managed container. No published TCP port, browser UI, remote management API or inbound webhook command execution is included in V1. Local IPC is an implementation boundary, not an internet API.

| CLI family (proposed) | Behavior / local endpoint |
|---|---|
| `config validate`, `serve --config PATH` | Offline syntax/reference validation; daemon startup applies one immutable revision |
| `source check ID`, `destination check ID` | `/v1/checks`; explicit connection test with bounded side effects; no automatic destructive tests |
| `jobs list`, `backup run JOB` | GET `/v1/jobs`; POST `/v1/runs` with job_id and idempotency request_id |
| `history list/show`, `operation show/cancel` | Read run/artifact/copy/operation records; cancellation requests normal process-group shutdown |
| `backup pin/unpin`, `retention preview/apply` | Pinning and deletion plans bounded to managed copies and configured protections |
| `verify run COPY --target ID` | POST `/v1/verifications`; starts remote-copy restore verification |
| `restore plan COPY --target ID --new-db NAME` | POST `/v1/restore-plans`; returns manifest, target identity, requirements and plan digest |
| `restore execute PLAN --confirm DIGEST` | POST `/v1/restores`; consumes a valid confirmed plan and returns operation_id |
| `notifications test CHANNEL`, `deliveries list/retry` | Explicit test sends and delivery inspection; ambiguous retry warns about duplicates |
| `catalog import --destination ID --preview` | Discover signed commit manifests; apply requires explicit import request; retention stays disabled until reviewed |
| `health` | GET `/v1/health`; liveness, scheduler/state/storage status, overdue jobs and delivery backlog |

Return structured JSON with `request_id`, `operation_id`, `status`, `data` or `{code,message,retryable,details}` error. List operations use bounded pagination. Long operations return accepted IDs; callers poll records. Repeated mutating request IDs with the same payload return the existing operation; a changed payload conflicts. HTTP transport statuses distinguish accepted, validation error, conflict, denied, unavailable and internal failure. CLI wait mode exits nonzero on terminal failure; accepted without wait does not claim completion.

## Contracts

### YAML contract (proposed schema v1)

Top-level groups: `version`, `service`, `sources`, `destinations`, `jobs`, `restore_targets`, `verification_targets`, `encryption`, `notifications`, `logging`. Reject unknown/duplicate keys, invalid identifiers, dangling references, duplicate output locations, unsupported schedules and invalid retention combinations. Do not execute shell strings or interpolate arbitrary environment variables. Explicit secret-file fields are resolved with bounded reads and redacted errors; fail closed for missing secrets needed by enabled jobs.

Illustrative fragment, not a deployable complete configuration or an implemented schema:

```yaml
version: 1
service:
  timezone: Asia/Colombo
  staging_max_bytes: 21474836480 # Example 20 GiB; size for actual workload
  min_free_bytes: 2147483648   # Example reserve
  max_heavy_operations: 1
sources:
  application_pg:
    type: postgres
    host: application-postgres
    port: 5432
    credentials_file: /run/secrets/application_pg
    databases: [app, billing]
  application_mongo:
    type: mongodb
    host: application-mongo
    port: 27017
    credentials_file: /run/secrets/application_mongo
    databases: [crm]
    consistency: live_not_guaranteed
jobs:
  nightly_pg:
    source: application_pg
    schedule: {cron: "0 1 * * *"}
    destinations:
      - {id: primary_r2, required: true}
      - {id: secondary_drive, required: false}
    retention:
      keep_last: 5
      keep_days: 7
      min_successful: 1
      protect_last_verified: true
    verification:
      schedule: {cron: "0 4 * * 0"}
      destination: primary_r2
      target: verification_pg
      profile: portable_data
notifications:
  channels: {} # Valid: no notification deliveries
```

The referenced destinations, verification target and encryption settings must exist in a complete config. Disk values above are examples, not capacity claims. Daily/weekly shortcuts normalize to the schedule policy in system overview. At least one destination must be required for each enabled job. No source is implicitly included merely because it is discoverable. PostgreSQL globals export is opt-in with explicit permissions, separate artifact/retention scope and no automatic role mutation on restore.

### Internal adapters

| Boundary | Required operations / result |
|---|---|
| DatabaseAdapter | `Preflight`, `Dump`, `PlanRestore`, `Restore`, `InspectRestored`; return version/scope/consistency and structured errors; caller owns process limits |
| StorageAdapter | `Capabilities`, `BeginUpload`, `Upload/Resume`, `Inspect/Reconcile`, `Download`, `ListManaged`, `Delete`; return stable provider receipt, size and qualified checksum evidence |
| NotificationAdapter | `Format`, `Send`, optional `Reconcile`; distinguish definitely-not-sent, delivered, retryable-rejected, ambiguous and permanent failure |
| SecretProvider | Resolve explicit local secret references without persisting plaintext into job/config snapshots |

Interfaces remain package-level contracts; exact Go signatures belong in implementation plans. Adapters cannot mark a run complete themselves: the orchestrator persists outcomes and events.

### Remote artifact commit protocol

Use an immutable logical path such as `server_id/job_id/source_id/database_scope/artifact_id/`; sanitize components and use IDs rather than raw uncontrolled database names. Store `archive.age`, `manifest.age`, detached `commit.sig` and `commit.json`. On Drive, persist actual folder/file IDs and app-owned artifact metadata; names are not unique identifiers.

1. Persist upload intent and a stable provider identifier/key before the request. For Drive, persist a generated file ID where supported and a unique artifact/copy marker; recover ambiguous creates by exact ID/marker lookup before another create.
2. Upload/resume the archive. Verify size and SHA-256 by reading the remote ciphertext back by default. An adapter may use an authoritative provider-validated SHA-256 only after capability tests prove its semantics. A copied user metadata hash or generic multipart ETag is not evidence of content integrity.
3. Upload the encrypted manifest and detached Ed25519 signature. The signature covers exact stored `commit.json` bytes; the commit lists artifact identity, minimal metadata, archive/manifest hashes and lengths, signing key ID and provider object references. Verify uploaded component hashes.
4. Publish `commit.json` last, then reconcile and mark the copy committed in SQLite. Readers require matching signature and all referenced objects; a visible commit without those checks is incomplete. Signatures are verified with a separately trusted public key, never one obtained solely from the same untrusted backup location.

There is no cross-object atomic transaction. Retry/reconciliation must handle missing components and lost responses. Multipart/session resources are reconciled and aborted when safe; retention deletes only known committed owned copies. Unknown objects are reported, not swept by name prefix alone.

### Webhook events

Common envelope: `schema_version`, `event_id`, `event_type`, `occurred_at` (UTC), `server_id`, `job_id`, `run_id`, optional `artifact_id/copy_id`, `sequence`, `outcome`, `summary`. Include consistency and verification status where relevant, bounded redacted error codes and timing/size; never include credentials or backup contents.

Events: `backup.started/completed/degraded/failed`, `upload.completed/failed`, `archive_validation.completed/failed`, `verification.started/completed/failed`, `restore.started/completed/failed`, `retention.completed/failed`, `backup.overdue`, `storage.low_space`. Archive validation means producer/format/integrity checks at that stage, not a restore guarantee. `backup.completed` follows the system-overview aggregate rule. Restore verification emits an independent later event.

Omitted notifications or empty channels are valid and generate zero deliveries. Enabled channels require valid settings. Per-channel event filters and job routes apply to future events; a config change does not replay all historical events. Persist a redacted channel revision for queued delivery and resolve its secret reference at send time. Removing/disabling a channel cancels its pending delivery attempts visibly; it does not reroute them silently.

Proposed bounds: five send attempts, maximum delivery age 24 hours, and at most 10,000 nonterminal deliveries globally, configurable. Expired deliveries retain compact outcome evidence. At capacity, retain the core event and a delivery-suppressed audit count, raise degraded health and avoid recursively notifying about queue overflow. A notification outage must not block database backups or cause unlimited SQLite growth; detailed delivery/history cleanup has a separate configured budget.

Custom HTTP uses JSON POST, optional authorization and an HMAC over timestamp plus exact body bytes; include event ID for receiver deduplication. Receiver-idempotent mode may use at-least-once retry with the same event ID. Otherwise, ambiguous post-send outcomes become unknown. Discord uses `wait=true`, suppresses automatic mentions, obeys message limits and records returned IDs when available. A lost response can remain unknown; no exactly-once guarantee is made. Honor provider rate-limit timing. [Discord webhook reference](https://docs.discord.com/developers/resources/webhook).

## Access and limits

IPC callers are trusted local operators authorized by filesystem permissions; no multi-user RBAC is claimed. Restore plans expire after a proposed 10 minutes, bind exact backup/copy, target engine identity, destination name and profile, and revalidate at execution. Reject production/verification target identity collisions, source-equivalent instances for both verification and operator restore, existing destination databases and changed/expired plans. A separate restore container on the same host is permitted. Manual destructive operations not described here are out of scope for V1.

Use bounded payloads, request timeouts, path validation, safe subprocess argument arrays and no caller-supplied SQL/shell hooks. Configured verification SQL is operator-authored, allowlisted and run with a read-only role and time limit against the isolated restored target. PostgreSQL restore may execute source-defined code: target isolation is mandatory.

## Evidence

Implementation must exercise config rejection, schedule/DST uniqueness, idempotent commands, process failures, storage reconciliation, checksum mismatch, retention/restore races, disabled notifications and ambiguous webhook sends. Exact API schema types and supported native-tool versions remain TODO for approved implementation planning. No API server or command is implemented by this document.

## Related documentation

- [System overview](system-overview.md)
- [Database design](database-design.md)
- [Security design](security-design.md)
- [Functional requirements](../01-requirements/functional-requirements.md)
