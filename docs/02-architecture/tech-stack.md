# Technology stack

**Document status:** Architecture v0.1 approved under H1 on 2026-10-24; no implementation.

## Approval record

The human approved architecture v0.1 and P01-P09 on 2026-10-24; see [H1 evidence in ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md). Proposal/default wording below records the approved design rationale, not a pending repeat approval. Unknown deployment facts remain unresolved; H2/H3/H4 are not granted.

## Purpose

Compare implementation technologies for the proposed single-server architecture and define version-selection constraints.

## Decision register

| Layer | Recommendation | Alternatives / tradeoff | Status |
|---|---|---|---|
| Application | Go daemon and CLI | Python is concise for automation but adds runtime packaging; TypeScript fits web/API work but adds Node/runtime dependencies. Go suits bounded streaming, process supervision and one deployable application. | Proposed P01 |
| Metadata | SQLite on local disk, foreign keys, WAL, short transactions | External PostgreSQL is appropriate if multiple workers/hosts need shared state, but is another service to protect on this one-server release. JSON files lack transaction/concurrency guarantees for history and outbox. | Proposed P01 |
| PostgreSQL backup | Native `pg_dump -Fc`, `pg_restore`; optional separate globals export | Physical/WAL backup supports different recovery objectives and is outside this scheduled-logical V1 proposal. | Proposed P04 |
| MongoDB backup | Native `mongodump --archive --gzip`, `mongorestore` | Filesystem copies of live data are not adopted. Replica-set/oplog mode is not assumed for the confirmed standalone deployment. | Proposed P04 |
| S3/R2 | AWS SDK for Go v2 behind a capability-aware adapter | rclone reduces provider code but introduces another configuration/process surface and less direct control of receipts/reconciliation. | Proposed P03 |
| Google Drive | Official Google API Go client, resumable binary-file uploads | rclone remains a future adapter option; Drive is modeled by folder/file IDs rather than bucket semantics. | Proposed P03 |
| Encryption | Maintained age Go library using recipient encryption | Managed KMS adds external dependencies; custom cryptography is excluded. | Proposed P05 |
| Configuration | Strict versioned YAML and file-based secret references | Environment-only configuration becomes unwieldy for multiple sources/jobs. Exact parser/validator package chosen during implementation planning. | Proposed |
| Control interface | CLI + HTTP/JSON over Unix domain socket | Public REST/UI expands access control and attack surface without a confirmed V1 need. | Proposed P02 |
| Notifications | Standard HTTP client + custom/Discord format adapters | A message broker is unnecessary for one daemon with a durable outbox. | Proposed P09 |
| Packaging | Linux Docker image and Compose | Kubernetes/multi-host agents defer until scope expands. | Proposed P02 |

## Compatibility and lifecycle

Pin the application, base image, native database tools and Go dependencies to reviewed versions/digests. Do not choose floating `latest` tags in a production manifest. Maintain a tested source-tool-restore matrix and fail preflight for unsupported combinations rather than running the wrong client silently.

PostgreSQL custom-format dumps support selective/sequential restore; each dump covers one database. Roles/tablespaces need a separately managed global-object export. A newer unsupported server must not be backed up by an older incompatible `pg_dump`. MongoDB source/target major/feature compatibility and tool support must be validated for the actual deployment. Logical backup scope is suited to the requested scheduled V1; scale/recovery benchmarks determine when a different backup method is required. [PostgreSQL documentation](https://www.postgresql.org/docs/current/app-pgdump.html), [MongoDB documentation](https://www.mongodb.com/docs/database-tools/mongodump/).

SQLite WAL supports local concurrent readers with one writer; do not put state on NFS or share it between hosts. Select a currently patched SQLite build, test the chosen Go driver against that build and exercise checkpoint/restart behavior. Exact driver and dependency versions remain TODO; no dependency installation is authorized by design work. [SQLite WAL documentation](https://www.sqlite.org/wal.html).

R2 supports a subset of S3 features, so compatibility must be tested against R2 itself. Drive supports resumable uploads but session state, authentication and quota behavior differ. Capability tests determine integrity checks and retry/reconciliation behavior rather than treating providers as interchangeable. [R2 compatibility](https://developers.cloudflare.com/r2/api/s3/api/), [Drive uploads](https://developers.google.com/workspace/drive/api/guides/manage-uploads).

## Library and build selection left to planning

Select maintained Go packages for SQLite, strict YAML decoding, five-field cron with IANA timezone handling, CLI parsing and log rotation after H1. Build the compatibility matrix, license/security checks and native-tool subprocess tests before pinning releases. The architecture requires behavior and ownership boundaries; it does not prescribe unverified package versions.

## Commands

TODO: exact build, lint, unit/integration and image-validation commands after implementation planning approval. No build system, packages, Dockerfile or executable code is created in this design task.

## Related documentation

- [System overview](system-overview.md)
- [Infrastructure design](infrastructure-design.md)
- [Security design](security-design.md)
- [Decision register](../04-decisions/README.md)
