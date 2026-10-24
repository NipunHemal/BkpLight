# BkpLight product requirements document

**Document status:** Confirmed scope plus architecture resolutions P01-P09 accepted under H1; inventory and detailed acceptance synchronization remain open.

## Current disposition - 2026-10-24

The user approved architecture v0.1/P01-P09 and requested roadmap documentation only; [ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md) records the exact evidence. Earlier discovery and architecture-handoff wording below is retained as history: claims that P01-P09 still require confirmation are superseded by this approval. C01-C05 remain the original user answers. Deployment facts and detailed acceptance traceability are not invented; [roadmap M00](../03-planning/roadmap.md) tracks their completion. H2/H3/H4 remain ungranted.

## Purpose

Specify the first release of a Docker-hosted database backup service using the user's requirements, explicit clarification decisions and separately labeled recommendations. This document does not claim that any feature is implemented.

## Authorization and evidence

On 2026-10-24 the user explicitly requested Superpowers brainstorming, requirements analysis, clarifying questions and a complete PRD in this directory, with no code implementation. This grants lifecycle H0 for discovery and documentation. H1/H2/H4 are not granted.

The earlier conversation is the primary requirements source. Explicit choices in the clarification log below take precedence over earlier assistant proposals. The repository contains documentation and skill instructions, with no application source or verified runtime. The implementation stack, database tools, state store and concrete YAML schema are not selected by this PRD draft.

## Problem and product intent

### Architecture handoff - 2026-10-24

The user subsequently asked to design architecture based on the approved PRD and prior conversation, then ask for approval. Treat this as authorization to draft against the confirmed scope below. The file still contains unanswered details; preserve them rather than inventing approval evidence. [Architecture proposal v0.1](../02-architecture/system-overview.md) maps these details to proposed decisions P01-P09 for H1 review. The original requirements and C01-C05 answers remain unchanged. TODO: record the human disposition and synchronize accepted proposals into requirements after review.

The user needs to operate backups for multiple databases running in Docker on one server without exposing database ports outside Docker networks. They want configurable storage destinations, backup history and retention, restore verification, a restore process, optional notifications and local logs, managed through YAML configuration.

## Users and operating context

- Primary operator: the user managing their own Docker server; additional roles and access requirements are TODO.
- Version-one host scope: one server, with multiple database containers/databases accessible over private Docker networks.
- Database engines: PostgreSQL and MongoDB.
- Current MongoDB deployment: standalone container, confirmed by the user; actual engine/tool versions are TODO.
- Workload: applications must remain online and continue reading and writing during backups.
- Expected database count, size, growth, server resources and deployment platform details: TODO.

## Confirmed scope from the conversation

| ID | Requirement intent | Source / clarification |
|---|---|---|
| REQ-001 | Run the backup service in Docker on the user's own server | User restricted first version to their own server |
| REQ-002 | Connect to databases through private Docker networks without requiring externally exposed database ports | User explicitly specified Docker-inside networks |
| REQ-003 | Support multiple PostgreSQL and MongoDB databases | User explicitly added PostgreSQL and multiple databases |
| REQ-004 | Configure the service using YAML | User explicitly selected YAML configuration |
| REQ-005 | Run daily, weekly and other configurable scheduled backups | Clarification C01; exact schedule syntax and timezone policy TODO |
| REQ-006 | Maintain backup history and configurable retention, including latest 3/5 backups or one week | User gave these retention examples; composition/scope rules TODO |
| REQ-007 | Provide backup restore verification and an operator restore process | User explicitly requested both |
| REQ-008 | Support storage destinations through adapters, including Google Drive and Cloudflare R2 | User named these destinations and adapter model; original 'S2' interpreted as S3 remains to confirm |
| REQ-009 | Provide optional custom HTTP and Discord webhook notifications configured in YAML | User explicitly requested these channels and optional behavior |
| REQ-010 | When webhook configuration is absent/disabled, backups work without sending webhook notifications | User explicitly requested optional webhooks |
| REQ-011 | Notify on backup start/completion and validation completion; other event details to clarify | User examples; complete event and filtering policy TODO |
| REQ-012 | Maintain local log files | User explicitly requested local logs; rotation/retention policy TODO |
| REQ-013 | Backups must run while applications remain online with reads and writes continuing | Clarification C02 |
| REQ-014 | Support existing standalone MongoDB and disclose live-backup consistency limits | Clarifications C03-C04 |
| REQ-015 | Allow temporary local backup archives with a configurable disk limit, so failed uploads can retry without repeating the database dump | Clarification C05 |

## Clarification decisions

| ID | Question / topic | Explicit user decision | Consequence |
|---|---|---|---|
| C01 | Data-loss objectives versus scheduled backups | V1 schedules backups daily, weekly, etc.; management of an affordable loss-time duration is outside V1 | Do not impose a guaranteed RPO; schedule choice is configurable. Maximum restore duration remains unspecified. |
| C02 | Availability during backup | Applications stay online during backups | Do not require a write-maintenance window as the default backup procedure |
| C03 | Current MongoDB topology | Standalone MongoDB container | Do not assume a replica set or automatically reconfigure MongoDB |
| C04 | MongoDB consistency guarantee | Support current standalone MongoDB and clearly disclose live-backup consistency limits | Backup metadata, history and verification reports must not imply a consistent snapshot or point-in-time recovery |
| C05 | Temporary archives versus direct streaming | Temporary local files with a configurable disk limit | Stage completed backups locally for upload/retry; direct-stream-only behavior is not required. Exact cap, cleanup and disk-full policies remain to be specified. |

## Backup semantics and consistency

The release provides scheduled recovery copies; it does not guarantee a maximum data-loss duration. A configured schedule is distinct from evidence that a backup succeeded. **Recommendation pending confirmation:** show last successful remote backup time and detect failed/overdue jobs.

For a standalone MongoDB dump while writes continue, the data may not represent a single instant across collections. An archive that restores successfully may still reflect that limitation. The product must distinguish archive/upload success, restore verification and snapshot consistency. MongoDB documents that oplog capture requires an eligible replica-set member and a full dump; the user has not authorized a topology change. See [MongoDB mongodump documentation](https://www.mongodb.com/docs/database-tools/mongodump/) (checked 2026-10-24).

PostgreSQL backup scope and engine/version guarantees will be specified after clarification and technical validation. No shared consistency guarantee across independently backed-up databases is approved.

Temporary local archives are permitted with a configurable staging disk limit. A completed archive can be reused for upload retries without another database dump. The product must distinguish local staging from a successful remote backup. TODO: define the limit's scope, behavior when space is insufficient, failed-upload retention and cleanup after successful delivery.

## Operator journeys to complete

1. Configure named sources, destinations and schedules in YAML; validate connections and permissions. Configuration validation details: TODO.
2. Execute a scheduled backup and inspect per-database/destination results. Manual execution and operator interface: TODO.
3. Apply latest-count or age-based retention and inspect retained/deleted backups. Combination rules, pinning and minimum copies: TODO.
4. Verify a backup by restoring into a safe target and reviewing the result. Automation cadence, target ownership and depth of checks: TODO.
5. Select a backup and restore through an explicit operator workflow. New-target versus overwrite behavior and production approval: TODO.
6. Optionally configure notification channels/events and inspect local logs. Retry, delivery history, rotation and retention: TODO.

## Recommendations requiring confirmation

- Manual backup/restore commands and YAML as the first-release operator interface.
- Encryption before remote upload, secret-file references and bounded disk/resource usage.
- Independent destination retries and a distinction between required and optional copies.
- Safe retention minimums, pinned backups and protection of the latest verified copy.
- Automated isolated restore verification at a configurable cadence.
- Durable job/notification state, bounded retries and explicit ambiguous-delivery handling.
- Last-success, failure and overdue monitoring even without a guaranteed RPO.

These are proposals, not accepted architecture decisions or final requirements.

## Acceptance criteria - confirmed portion

| ID | Requirement | Observable acceptance condition | Verification status |
|---|---|---|---|
| AC-001 | REQ-001/002/003 | A deployment can reach configured PostgreSQL/MongoDB databases through Docker networks without requiring public database ports | Planned; not run |
| AC-002 | REQ-004/005 | YAML expresses daily/weekly schedules and the service executes jobs according to the approved timezone/missed-run policy | Pending policy clarification; not run |
| AC-003 | REQ-006 | Retention can be configured to keep the latest 3 or 5 copies, or one week, according to the clarified retention scope | Pending policy clarification; not run |
| AC-004 | REQ-009/010 | With no webhook configuration, backup operation performs no notification delivery and does not fail due to missing webhook settings | Planned; not run |
| AC-005 | REQ-013/014 | A supported standalone MongoDB backup does not require stopping application writes; its result clearly identifies snapshot consistency as not guaranteed | Planned; not run |
| AC-006 | REQ-007/014 | Restore-verification status is separate from consistency status; successful restore validation does not relabel a standalone live dump as a consistent snapshot | Planned; not run |
| AC-007 | REQ-015 | Following a failed upload, retry can reuse the completed local archive without starting a new database dump | Planned; not run |
| AC-008 | REQ-015 | The configured staging disk limit is enforced with observable outcomes when capacity is insufficient | Pending disk-full and concurrent-job policy clarification; not run |

TODO: complete traceable criteria for every requirement and approved recommendation after clarifications.

## Non-functional requirements

Confirmed constraint: no required application read/write outage during backups. No guaranteed maximum data-loss duration is in scope for V1. TODO: database count/size limits, resource budgets, supported engine/platform versions, restore expectations, encryption policy, disk/log retention, secret handling, retry budgets, audit depth and configuration compatibility.

## Release scope and exclusions

Confirmed first-release boundary: one server and scheduled database backups. No automatic MongoDB topology conversion is authorized. TODO: finalize storage-provider availability, UI/CLI scope, point-in-time recovery exclusion, multi-server exclusions, physical/volume backups, database roles/globals and advanced restore scope. Earlier assistant lists do not settle these choices.

## Risks and dependencies

- Live standalone MongoDB consistency limits are accepted and must remain visible, including after restore verification.
- Storage capacity, credentials, provider restrictions and network reliability are not yet characterized.
- Verification and restore can modify data; safe target behavior and authorization must be defined before implementation.
- Schedule execution, upload success and restorability need distinct evidence and statuses.
- Existing application, build and runtime evidence is absent; PRD completeness is not production readiness.

## Completion and approval

Discovery remains in progress. The PRD becomes review-ready after material questions are resolved or explicitly accepted as deferred, recommendations are dispositioned, each requirement has acceptance criteria, and a reader review checks contradictions and gaps. Product-specific unknowns remain TODO until answered. Final PRD/design approval does not itself authorize implementation or deployment.

## Related documentation

- [Project brief](project-brief.md)
- [Functional requirements](functional-requirements.md)
- [Non-functional requirements](non-functional-requirements.md)
- [Acceptance criteria](acceptance-criteria.md)
- [Open questions](open-questions.md)
- [Development lifecycle](../03-planning/development-lifecycle.md)
- [Current status](../05-progress/current-status.md)
