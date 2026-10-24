# Open questions

**Document status:** Confirmed scope plus architecture resolutions P01-P09 accepted under H1; inventory and detailed acceptance synchronization remain open.

## Current disposition - 2026-10-24

The user approved architecture v0.1/P01-P09 and requested roadmap documentation only; [ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md) records the exact evidence. Earlier discovery and architecture-handoff wording below is retained as history: claims that P01-P09 still require confirmation are superseded by this approval. C01-C05 remain the original user answers. Deployment facts and detailed acceptance traceability are not invented; [roadmap M00](../03-planning/roadmap.md) tracks their completion. H2/H3/H4 remain ungranted.

## Purpose

Track material missing requirements, user decisions and unresolved recommendations for the PRD.

## Question register

| ID | Topic / missing detail | Impact | State / evidence |
|---|---|---|---|
| Q01 | Maximum acceptable data-loss duration | Recovery scope | Resolved C01: V1 schedules daily/weekly/etc.; no managed loss-window guarantee |
| Q02 | Application writes during backup | Availability and consistency | Resolved C02: applications stay online, reads/writes continue |
| Q03 | Current MongoDB topology | Supported backup mode | Resolved C03: standalone MongoDB container |
| Q04 | Standalone live-backup consistency guarantee | Correctness and operator expectations | Resolved C04: support existing standalone with explicit consistency limitation |
| Q05 | Temporary complete local archive versus direct streaming | Disk budget, retry cost and failure recovery | Resolved C05: temporary local files with a configurable disk limit; detailed cap/cleanup policy tracked in Q08/Q15 |
| Q06 | Required V1 destinations; confirm original 'S2' means Amazon S3; one or multiple destinations per job | Adapter/release scope | Policy resolved by H1: P03 (R2/S3/Drive and required/optional copies); deployment values/credentials remain inventory |
| Q07 | Daily/weekly/custom schedule syntax, timezone, manual runs, missed schedules and overlap | Predictable scheduling | Policy resolved by H1: P08 (schedule semantics); deployment values/credentials remain inventory |
| Q08 | Database count/size/growth, engine versions, host OS/architecture, available CPU/RAM/disk | Supported scale and constraints | TODO; no credentials needed |
| Q09 | Selected databases versus full-instance backup; PostgreSQL roles/globals/extensions | Backup completeness and restore scope | Policy resolved by H1: P04 (selected databases and opt-in globals); deployment values/credentials remain inventory |
| Q10 | Retention per database/destination, count+age combination, pinned copies and safe minimum | Deletion semantics | Policy resolved by H1: P06 (retention protections); deployment values/credentials remain inventory |
| Q11 | Manual/automatic restore verification, cadence, target provisioning and validation depth | Restorability and resource usage | Policy resolved by H1: P07 (manual/weekly isolated verification); deployment values/credentials remain inventory |
| Q12 | Restore into new targets, overwrite permission, selective restore and recovery after server loss | Recovery safety and usability | Policy resolved by H1: P07 (new-target restore); deployment values/credentials remain inventory |
| Q13 | YAML+CLI versus additional UI/API | Operator experience and V1 boundary | Policy resolved by H1: P02 (CLI/local IPC); deployment values/credentials remain inventory |
| Q14 | Encryption requirement, key ownership, secret sources and credential permissions | Confidentiality and recovery dependencies | Policy resolved by H1: P05 (encryption and key custody); deployment values/credentials remain inventory |
| Q15 | Upload retries, partial multi-destination success, restart recovery and disk-full behavior | Reliability policy | Policy resolved by H1: P05/P08 (bounded staging and retries); deployment values/credentials remain inventory |
| Q16 | Notification events/routing, delivery retries/duplicates, local log format/rotation/retention | Observability and bounded storage | Policy resolved by H1: P09 (notifications and logs); deployment values/credentials remain inventory |
| Q17 | Last-success/failure/overdue visibility and alert behavior when webhooks are disabled | Detection of missed protection | Policy resolved by H1: P08 (health and overdue visibility); deployment values/credentials remain inventory |
| Q18 | Definition of V1 done, deferred features and PRD approval | Release acceptance | TODO |

## Resolution policy

### Architecture resolution mapping - 2026-10-24

The user initially authorized drafting and then explicitly approved architecture v0.1/P01-P09. The register above records the later resolution; remaining inventory facts are still open. Use these accepted choices in the system overview:

| Open questions | Accepted resolution / remaining evidence |
|---|---|
| Q06 | P03: R2, S3 and Drive with required/optional destinations; S3 interpretation accepted with P03 |
| Q07/Q15/Q17 | P08: durable schedules/retries, overlap/missed-slot policy, disk-full stop and last-success visibility |
| Q08 | P02 proposes Linux Compose; actual versions, resources, data sizes and accounts remain inventory TODOs |
| Q09 | P04: explicitly selected databases, per-database logical dumps, opt-in PG globals |
| Q10 | P06: count OR age retention plus minimum/pinned/latest verified protection |
| Q11/Q12 | P07: manual/weekly isolated verification, new-target-only operator restore and remote catalog recovery |
| Q13 | P02: YAML and CLI/local IPC; public API/UI deferred |
| Q14 | P05 plus security design: encrypted staging, independent key custody; online decrypt key needed for automatic verification |
| Q16 | P09: optional HTTP/Discord, bounded delivery state and rotating local logs |
| Q18 | H1 reviews the proposed V1 boundary; complete acceptance traceability during subsequent authorized planning before H2 |

These defaults were accepted under H1 on 2026-10-24. Operational inventory and measured limits cannot be inferred from approval of the design.

Record user answers and sources. Earlier proposals are not automatically approved requirements.

## Next-phase authorization

H1 for architecture v0.1/P01-P09 is granted; the current request authorizes roadmap documentation only. H2/H3/H4 remain ungranted. Resolve inventory and exact task-plan prerequisites before requesting execution approval; accepted design policies need no repeat confirmation.

## Related documentation

- [Working PRD and clarification decisions](PRD.md)
- [Project brief](project-brief.md)
- [Decision register](../04-decisions/README.md)
- [Architecture proposal and P01-P09](../02-architecture/system-overview.md)
