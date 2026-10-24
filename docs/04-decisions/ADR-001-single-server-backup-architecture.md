# ADR-001: Single-server backup service architecture

**Document status:** Accepted v0.1 under H1 on 2026-10-24; implementation unapproved.

## Purpose

Record the architecture, alternatives, tradeoffs and subsequent explicit human approval. The initial drafting request alone did not grant acceptance.

## Context

BkpLight must back up selected MongoDB/PostgreSQL databases on one operator-owned Docker server, while applications remain online, using private networks, YAML schedules, remote storage adapters, history/retention, restore verification, optional webhooks and local logs. The user accepts bounded temporary local archives and standalone MongoDB live-backup consistency limits. Outstanding PRD choices are addressed as explicit proposals P01-P09, not fabricated user answers.

## Alternatives

1. Cron/scripts, native tools and rclone: simple initial backup path, but durable state, safe retention, provider reconciliation and restore evidence require substantial additional orchestration.
2. One modular daemon with SQLite and native tools: simpler single-host operations with a consistent owner for job state, retries and recovery. Recommended.
3. Distributed API, workers, message broker and external state database: useful for future multi-server scaling; unjustified operational cost for current scope.

## Accepted decision

Adopt option 2: Go daemon, SQLite on local disk, native database backup/restore tools, direct storage adapters, CLI over a restricted Unix socket, and Linux Compose deployment without Docker-socket access. Use encrypted bounded local staging, immutable artifact/copy identity, signed remote commit records for catalog recovery, protected retention and isolated remote-copy restore verification. All detailed choices P01-P09 in the system overview are covered by H1 below; no exceptions were stated.

## Consequences

- One state owner simplifies transactions and scheduling; no queue/database service is needed for the application.
- One host remains a failure domain. Off-host archives, independently held recovery keys and remote catalog recovery are essential.
- Native tool and provider compatibility need a tested matrix; SDK selection is not proof of interoperability.
- Full remote readback and weekly verification consume bandwidth, storage and CPU; measure costs before production.
- Automatic verification requires online decryption access. Manual verification is available if the operator declines that tradeoff.
- V1 restores create a new database on a separate allowlisted instance (which may be another container on the same host); source-instance restores and overwrite are deferred. Production cutover remains an operator action.
- Standalone MongoDB restores can succeed without proving point-in-time consistency. No multi-database atomic snapshot or RPO guarantee is claimed.

## Approval and evidence

### H1-2026-10-24 - Architecture approval

- **Approver:** the human project owner in this conversation.
- **Date:** 2026-10-24 (Asia/Colombo).
- **Artifact/scope:** architecture v0.1, all six documents in `docs/02-architecture/`, P01-P09 and this ADR. This includes Amazon S3 as the interpretation of the earlier "S2", alongside R2 and Google Drive.
- **Evidence reference:** the user's immediate response to the assistant's question asking approval of "architecture v0.1 and proposals P01-P09" including the S3 interpretation: "ok i aprued." The same message asks for a milestone roadmap using Superpowers writing-plans and ends "Do not implement."
- **Disposition:** accepted with no stated design exceptions. Original proposal language in the architecture documents is retained as design rationale; this record establishes its acceptance. Historical C01-C05 answers remain distinct from this later approval.
- **Limits:** approval does not establish unknown database versions, account capabilities, host resources, exact package choices, measured limits or passing tests. Those remain prerequisite work, not reasons to ask for the same H1 again.
- **Next authority:** roadmap documentation is authorized. H2 execution/mode, H3 integration and H4 production are not granted. TODO: record those approvals only when actually received.

Documentation review is not runtime/security/performance verification. Any material design change needs a scoped ADR update and renewed H1 rather than silently altering this accepted baseline.

## Related documentation

- [System overview, P01-P09 and requirement coverage](../02-architecture/system-overview.md)
- [Technology alternatives](../02-architecture/tech-stack.md)
- [Database and state](../02-architecture/database-design.md)
- [API and configuration](../02-architecture/api-design.md)
- [Security](../02-architecture/security-design.md)
- [Infrastructure](../02-architecture/infrastructure-design.md)
- [Decision register](README.md)
- [PRD](../01-requirements/PRD.md)
