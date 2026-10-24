# Roadmap

**Document status:** Roadmap v0.1, prepared 2026-10-24 against approved architecture v0.1; planning only, H2 not granted.

## Purpose

Organize the approved BkpLight architecture into prioritized, independently verifiable milestones. This is a milestone roadmap, not an executable task-by-task implementation plan.

## Authority and baseline

The user approved architecture v0.1/P01-P09 and requested this roadmap with the explicit restriction, "Do not implement." H1 evidence is recorded in ADR-001. The architecture includes R2, Amazon S3 and Google Drive; all three remain required V1 capabilities. Required/optional destination flags control individual job outcomes, not whether an adapter ships.

Use Superpowers writing-plans for decomposition, testable deliverables, dependency review and later detailed plans. Its code-level task format is reserved for the subsequent scoped plans: no speculative code, invented package versions, executable commands or implementation authorization is supplied by this roadmap. No worktree or Git initialization is needed for this documentation task.

**Goal:** Deliver a single-server service that creates, stores, verifies and restores selected database backups with observable, bounded failure handling.

**Architecture:** One Go daemon owns SQLite state, native database processes, encrypted staging and local CLI/IPC. Database, storage and notification adapters sit behind explicit contracts; dedicated targets isolate restores.

**Tech stack:** Go, SQLite, PostgreSQL/MongoDB native tools, age encryption, signed remote commit metadata, Linux Docker Compose and provider SDKs; exact supported versions and libraries remain planning prerequisites.

**Spec:** The six documents starting at [system overview](../02-architecture/system-overview.md), with [accepted ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md).

## Priorities and delivery order

Priority is sequencing, not permission to omit scope. **P0** establishes safe recovery and its prerequisites. **P1** completes required V1 operations/adapters. **P2** qualifies and prepares the complete release. Every milestone is required before claiming V1 release readiness; P2 is not optional polish. All product milestones start in `backlog`; no runtime work is complete.

| ID | Priority | Milestone / deliverable | Depends on | Exit evidence |
|---|---|---|---|---|
| M00 | P0 | Planning prerequisites: deployment inventory, compatibility matrix, acceptance map and first scoped execution plan | H1 (held) | Inputs resolved or dependent tasks explicitly withheld; concrete first plan ready for H2 |
| M01 | P0 | Secure service foundation: strict YAML, local CLI/IPC, SQLite state/outbox, secret references and bounded logs | M00 + scoped H2 | Config/IPC/state failure tests and redaction evidence |
| M02 | P0 | PostgreSQL/MongoDB encrypted local backup pipeline | M01 | Online fixture dumps, truthful consistency labels, cap/cancellation tests |
| M03 | P0 | Recoverable remote backup through S3 and R2 adapters | M02 | Remote integrity, immutable commits and lost-response/restart reconciliation on both providers |
| M04 | P0 | Remote restore verification and safe operator restore | M03 | Isolated end-to-end restores, wrong-target rejection and catalog recovery |
| M05 | P1 | Schedules, multi-database history, retention and overdue health | M04 | Time/DST tests, retention protection/race tests, visible missed/failed jobs |
| M06 | P1 | Google Drive and complete multi-destination behavior | M04 | Drive contract/real-provider tests and mixed-provider failure/recovery |
| M07 | P1 | Optional custom/Discord notifications and operator diagnostics | M05 | No-channel isolation, event routing, bounded retry/unknown-delivery tests |
| M08 | P2 | Reproducible image/Compose, integrated qualification and operational docs | M05, M06, M07 | Integrated candidate tests, least privilege, resource/failure evidence and documented limits |
| M09 | P2 | Release readiness dossier, staging/rollback evidence and production procedure | M08 + H3 for chosen integration | Exact candidate passes release prerequisites; concrete H4 request prepared, no automatic deployment |

Owners and reviewers are assigned when each scoped plan is prepared; TODO: actual names, availability and delivery dates. Milestone order is not a calendar estimate. Estimate only after M00 establishes workload, provider access and execution capacity.

## Delivery strategy

The first recovery slice is M01-M04: configure one source, dump/encrypt locally, upload to a real supported store, retrieve and restore into an isolated target. Both database engines and both S3/R2 adapters must satisfy their milestone evidence before the slice is complete. This exposes recovery and integrity risks before enabling automatic deletion in M05.

M06 can follow M04 while M05 proceeds only if future execution approval permits parallel work and shared interfaces are stable. M07 follows the scheduling/history events in M05. Core logs, durable events, permissions and disk limits start in M01/M02; they are not postponed until notifications or release hardening. Docker fixture tooling may be introduced in the scoped plan that first needs it; M08 consolidates the distributable image and deployment contract.

No interim slice is a production-ready V1. All providers, disabled-webhook behavior, safe retention, recovery and release evidence remain required. Weekly verification is integrated with scheduling in M05 after its manual engine is proven in M04.

## Scope constraints inherited by every milestone

- One operator-owned Linux Docker server; sources remain on private networks with no mandatory published ports or Docker socket access.
- Selected PostgreSQL and standalone MongoDB databases stay online. MongoDB live dumps retain `not_guaranteed` snapshot consistency after a successful restore; no cross-database atomic snapshot or managed RPO.
- YAML and local CLI/Unix IPC; no browser UI or public management API.
- Encrypt before persistent staging; enforce the configurable cap/reserve for partial, staged and downloaded archives. Never evict an undelivered finalized archive just to make space.
- Independent required/optional destinations, qualified integrity evidence and reconciliation; checksum success does not equal restore success.
- Retention is scoped to physical destination binding and database/job, keeps the union of count and age, and honors pins, claims, healthy minimum and latest verified protection.
- New-database restore on a separate allowlisted instance only. No source-instance/in-place overwrite or automatic cutover.
- Optional HTTP/Discord notifications cannot fail a valid backup; local logs/health remain mandatory.
- No new UI, multi-host orchestration, PITR, physical-volume backup, custom shell hooks or managed topology conversion is introduced by this roadmap.

## Requirement and architecture coverage

| Baseline | Milestones |
|---|---|
| REQ-001, REQ-002, P02: Docker/private networks/operator CLI | M01, M02, M08 |
| REQ-003, REQ-013, REQ-014, P04: selected engines, online operation, consistency | M02, M04, M08 |
| REQ-004, REQ-005, P08: YAML/schedules/recovery | M01, M03, M05 |
| REQ-006, P06: history/retention | M01, M05 |
| REQ-007, P07: verification/restore | M04, M05, M06, M09 |
| REQ-008, P03: R2/S3/Drive and multiple destinations | M03, M06 |
| REQ-009, REQ-010, REQ-011, P09: optional notification events | M01 (outbox), M07 |
| REQ-012, P09: logs/health | M01, M05, M07, M08 |
| REQ-015, P05: encrypted bounded staging and reusable retries | M02, M03, M04 |
| P01: Go/SQLite ownership, migrations and catalog state | M01, M03, M04, M09 |
| Security, independent key custody and signed disaster recovery | M01-M04, M06, M08-M09 |

The milestone exit checks in milestones.md are planned verification, not new product features or passing test results. M00 reconciles the PRD's historical unresolved wording and expands its AC-001 through AC-008 into full acceptance traceability using the approved architecture; it does not reopen accepted P01-P09 without cause.

## Detailed-plan and approval boundary

Create separate small execution plans under `implementation-plans/` for each milestone or coherent feature slice. Before H2, each plan must name exact create/modify/test paths, stable consumed/produced interfaces, concrete red/green tests and commands, baseline tooling, review criteria, rollback/cleanup limits and attempt budgets. Use the task template and writing-plans checklist; do not label this roadmap executable while those details are absent.

H1 is held for architecture v0.1. This request authorizes roadmap documentation only. H2 must cover the actual detailed plan and chosen execution mode before worktree setup, packages, implementation or fixture operations. H3 covers the selected integration action. H4 separately approves the exact production artifact/target/procedure after staging, fresh backup/restore, monitoring and rehearsed rollback. Milestone acceptance never substitutes for these gates.

## Status, evidence and replanning

The milestone register below is owned by milestones.md; task-level states later live only in the relevant execution plan. Current status and session handoff summarize/link those records. Keep integration and release states separate from task completion.

On failure, stop dependent work, preserve the last safe candidate and redacted evidence, and use systematic debugging within the task budget. Local fixes repeat affected tests/review. Provider limitations, missing privileges or capacity findings that change approved behavior require an ADR/H1 revision and affected H2 reapproval. Do not drop an adapter, weaken integrity checks or silently change retention to meet an estimate.

TODO: measured performance budgets and delivery estimates after inventory and fixture characterization. Unresolved deployment facts block their dependent execution plans, not this roadmap.

## Related documentation

- [Development lifecycle](development-lifecycle.md)
- [Task record template](task-template.md)
- [Milestones](milestones.md)
- [Task dependencies](task-dependencies.md)
- [Implementation plans](implementation-plans/README.md)
