# Milestones

**Document status:** Milestone register v0.1, 2026-10-24; planned against approved architecture, no execution authorized.

## Purpose

Define milestone entry criteria, work packages, deliverables and measurable exit evidence. Detailed task plans will refine these into exact files, interfaces and TDD steps before H2.

## Milestone register

| ID | Priority | Owner / reviewer | Status | Completion evidence |
|---|---|---|---|---|
| M00 | P0 | TODO: planning owner / operator | backlog | Not produced |
| M01 | P0 | TODO: implementation owner / reviewer | backlog | [Detailed plan v0.1](implementation-plans/2026-10-24-m01-secure-service-foundation.md); H2 pending; runtime checks not run |
| M02 | P0 | TODO: database owner / reviewer | backlog | Not run |
| M03 | P0 | TODO: storage owner / reviewer | backlog | Not run |
| M04 | P0 | TODO: recovery owner / reviewer | backlog | Not run |
| M05 | P1 | TODO: orchestration owner / reviewer | backlog | Not run |
| M06 | P1 | TODO: Drive owner / reviewer | backlog | Not run |
| M07 | P1 | TODO: notifications owner / reviewer | backlog | Not run |
| M08 | P2 | TODO: DevOps owner / reviewer | backlog | Not run |
| M09 | P2 | TODO: release owner / human approver | backlog | Not run |

Roles are responsibilities, not assumed staffing or delegation permission. These rows are the authoritative milestone states; every checkbox below is future work.

## M00 - Planning prerequisites (P0)

**Entry:** H1 approval recorded; roadmap available. Subsequent work remains subject to the user's documentation-only boundary.

- [ ] M00.1 Collect non-secret inventory: source versions/topology/databases/sizes/growth, network aliases, Linux/CPU platform, free disk/RAM/CPU and expected backup windows.
- [ ] M00.2 Establish source-tool-target compatibility and least-privilege requirements; identify S3/R2 test prefixes and Drive account/scope/recovery-access model. No credentials in documentation or chat.
- [ ] M00.3 Reconcile requirements with accepted P01-P09; expand acceptance mapping, fix resource/timeout/history limits for the target and assign owners. Separate unresolved deployment facts from approved design.
- [ ] M00.4 Prepare the first scoped plan: proposed file boundaries, exact library versions/commands, meaningful tests, recovery budget and execution mode; record Git/worktree prerequisites without initializing Git.

**Deliverables:** completed inventory/compatibility and acceptance records, decision list for genuine remaining choices, first reviewable plan under implementation-plans/.

**Exit:** every first-plan dependency is resolved; each later unknown has an owner and affected milestone. H2 remains a separate explicit approval before execution. Unsupported deployment facts trigger replanning, not an invented compatibility claim.

## M01 - Secure service foundation (P0)

**Entry:** M00 exit, scoped H2, authorized workspace/bootstrap and known baseline.

- [ ] M01.1 Implement strict versioned YAML, secret references and redacted immutable config revisions with safe startup validation.
- [ ] M01.2 Provide local CLI/Unix IPC and bounded idempotent operation handling; establish SQLite migrations, singleton ownership, state transitions, event outbox and history queries.
- [ ] M01.3 Add structured rotating/redacted logs, basic health, bounded native-process supervision and graceful stop primitives needed by later tasks.

**Deliverables:** testable daemon/CLI foundation, metadata migration baseline, config examples, shared error/operation contracts and development fixture harness.

**Exit:** reject duplicate/unknown YAML fields and missing enabled secrets; no TCP control listener; repeated request IDs do not duplicate operations; singleton collision is rejected; transaction rollback creates neither half-state nor half-event; newer schema and full state disk fail safely; secrets do not appear in logs/errors. Review covers state/outbox ownership and process cancellation.

## M02 - Online encrypted database archives (P0)

**Entry:** M01; tested engine/tool matrix and source credentials for isolated fixtures.

- [ ] M02.1 Add selected-database PostgreSQL custom dumps and standalone MongoDB archive/gzip dumps; explicit opt-in globals and backup scope metadata.
- [ ] M02.2 Stream into age encryption, finalize only after successful producers and durable file completion, bind manifest/version/scope/consistency metadata.
- [ ] M02.3 Enforce aggregate staging reservations/cap/free-space reserve, source locks, process timeouts/cancellation and restart handling for partial artifacts.

**Deliverables:** encrypted reusable artifacts for both engines, native-tool preflight/results, independently recoverable key instructions and disk-accounting tests.

**Exit:** fixture writes continue during dump; PostgreSQL per-database and MongoDB live-limit labels remain distinct; selected DBs only; incomplete/RLS-inadequate/unsupported exports fail visibly; encryption failure never yields a staged success; growing/parallel writers cannot exceed budget; killed workers are reaped and partials reconciled. A source failure does not erase another database's successful artifact. All local archive checks remain distinct from restore verification.

## M03 - S3/R2 remote recovery slice (P0)

**Entry:** M02; independent storage contract and signing/key custody specified; scoped provider test access authorized.

- [ ] M03.1 Implement capability-aware S3 and R2 adapters and independent required/optional delivery results using immutable artifact identity.
- [ ] M03.2 Upload archive/manifest/signature with commit-last publication; verify remote size/hash through full ciphertext readback unless qualified provider SHA-256 evidence is proven.
- [ ] M03.3 Persist upload intent/receipts, retry the same staged artifact, reconcile ambiguous responses and cleanup owned multipart resources safely.

**Deliverables:** one remotely retrievable committed backup path for each provider, qualified integrity evidence and crash/retry tests.

**Exit:** exercise both real S3 and real R2 in authorized isolated prefixes; mocks alone cannot establish provider acceptance. Inject failures at each commit boundary and prove no partial copy becomes successful, lost responses reconcile, optional failure yields degraded, required failure yields failed and committed peers survive. Retry invokes no new dump. Missing objects/tampering fail validation; ETag/copied metadata alone never passes integrity.

## M04 - Verification, operator restore and catalog recovery (P0)

**Entry:** M03; separately provisioned/allowlisted fixture targets, independent keys and target identity checks available.

- [ ] M04.1 Download committed copies, verify/decrypt, restore each engine into an operation-owned new database and record profile-specific structure/assertion evidence.
- [ ] M04.2 Provide preview/expiring confirmation bound to target/copy/name/profile; reject source-equivalent, changed or existing targets, with cleanup limited to operation-owned resources.
- [ ] M04.3 Import authenticated remote catalog records with an empty local state and retention disabled; document key loss/rotation and state backup/recovery boundaries.

**Deliverables:** manual remote verification, safe restore CLI flow, portable PostgreSQL profile and explicit faithful-profile prerequisites, new-host recovery procedure and evidence format.

**Exit:** restore remote copies for each engine/provider combination supported by M03; missing keys, modified signatures, corrupt data, changed targets, insufficient privileges and interrupted restore fail safely. MongoDB restore success never changes its consistency label. Compare archive-derived structure rather than current source counts. Import cannot fabricate historical verification or enable deletion. Wrong-target and cleanup-ownership tests pass; valid backup cannot be deleted while claimed by a restore.

## M05 - Scheduling, history and safe retention (P1)

**Entry:** M04; reliable copy/verification/claim states available.

- [ ] M05.1 Add daily/weekly/cron scheduling, timezone/DST semantics, durable slot identity, restart coalescing, bounded pending work and per-source overlap exclusion.
- [ ] M05.2 Schedule weekly/configurable verification with newest-eligible-copy semantics; expose per-database/copy history, failure, skipped-slot, last-success and verification age.
- [ ] M05.3 Implement retention preview/apply with capture-time ordering, union of count/age, protected copies and transactional deletion claims; add overdue/disk health and metadata compaction controls.

**Deliverables:** unattended job orchestration, readable history, retention audit/tombstones, verification schedules and operational health output.

**Exit:** deterministic clock tests cover DST repeats/gaps and restart/missed slots; unrelated config edits cannot replay ticks; source overlap is skipped and worker backlog bounded. Verify latest 3/5 and rolling 7-day rules alone/combined; preserve pins, active claims, healthy minimum and latest verified copies. Changed physical destination cannot authorize old-location deletion. Test pin/restore/delete races, corruption, lost delete responses and full-disk backlog; explain protected-count excess. Disabled webhooks do not hide overdue state.

## M06 - Drive and multi-destination completion (P1)

**Entry:** M04; generic copy/recovery contracts stable; actual Drive account/scopes and isolated test folder authorized. M05 is not a build prerequisite; integrated retention qualification waits for both.

- [ ] M06.1 Add Drive resumable uploads, stable file IDs/markers, protected mutable token/session state, scoped listing and ambiguous-create reconciliation.
- [ ] M06.2 Apply existing signed-commit/integrity, download, catalog import and exact-ID deletion contracts to Drive.
- [ ] M06.3 Exercise mixed R2/S3/Drive required/optional jobs and restore both engines from Drive without changing common orchestration semantics.

**Deliverables:** all three V1 adapters, account bootstrap/rotation/recovery guide and shared adapter conformance evidence.

**Exit:** real authorized Drive tests cover refresh/revocation, quota/rate limits, resumable-session expiry and lost responses without blind duplicate creation; selected OAuth scope can recover/list files after state loss. Full readback detects corrupted content. One destination outage preserves successful copies and the retryable local archive. Drive retention is qualified with M05 during M08. No provider is silently deferred to a later release.

## M07 - Optional notifications and diagnostics (P1)

**Entry:** M05; persisted events and history/health are stable. M06 provider-specific combined scenarios are qualified in M08.

- [ ] M07.1 Add custom JSON/HMAC and Discord formatting/delivery, YAML filters/routes and explicit notification test command.
- [ ] M07.2 Implement bounded outbox delivery retries, expiry/cancellation/unknown states, rate-limit timing and redacted delivery history.
- [ ] M07.3 Document backup/upload/archive-validation/verification/restore/retention events and operator response to missed backups, disk pressure and queue exhaustion.

**Deliverables:** optional provider adapters, sanitized webhook examples and diagnostic procedures, with no inbound command execution.

**Exit:** no channels means zero outbound notification requests; notification outage cannot fail a completed backup. Test events independently, signatures, duplicate handling, disabled channels, queue limits, ambiguous Discord outcomes and mention suppression. Validate HTTPS/redirect/address restrictions and secret redaction. Use authorized isolated real-provider tests separately from mocked contracts; never send a production webhook as a test by default.

## M08 - Integrated V1 qualification (P2)

**Entry:** M05/M06/M07 exits; all baseline capabilities complete under approved plans.

- [ ] M08.1 Consolidate pinned reproducible image and Compose templates: non-root/read-only root, no socket/public source ports, explicit networks, volumes/secret ownership, native-tool compatibility and resource/log limits.
- [ ] M08.2 Run the full engine/provider/config matrix, multi-source workloads, restart/crash/full-disk tests, retention races, key rotation, catalog import and state upgrade/recovery against one candidate.
- [ ] M08.3 Measure backup/restore time, archive/verification expansion, bandwidth/readback cost and production-fixture impact; document supported limits, install/config/monitoring/recovery guides and CI evidence.

**Deliverables:** immutable candidate image, Compose/config examples, automated checks, operator docs, acceptance/evidence matrix and reviewed support limits.

**Exit:** fresh security/functional/integration checks on that candidate pass; all REQ-001 through REQ-015 and accepted decisions have evidence; no secrets in image/config/logs; resource isolation demonstrated with realistic fixtures. Missing real-provider access is blocked evidence, not a mocked pass. Native tool/OS/CPU support claims match measured fixtures. Close review blockers; readiness is assessed separately in M09.

## M09 - Release readiness and deployment handoff (P2)

**Entry:** M08, authorized integration action under H3 and a verified integrated candidate; explicit staging scope/targets available.

- [ ] M09.1 Promote the same immutable candidate to isolated staging and validate the exact target-relevant compatibility/configuration, migrations and failure/recovery paths.
- [ ] M09.2 Produce applicable fresh backup IDs and isolated restore evidence, independent recovery material, a compatible image/state rollback rehearsal and retention-disabled reconciliation procedure.
- [ ] M09.3 Verify monitoring/overdue ownership, log rotation, alert/no-webhook choice and external observation; assemble the concrete production procedure, stop conditions and rollback scope for H4 review.

**Deliverables:** release-readiness dossier with exact candidate/target, evidence timestamps, runbooks and explicit production approval request. Production deployment is outside this roadmap's execution scope.

**Exit:** production-readiness review passes with no missing mandatory staging/backup/rollback/monitoring evidence; authorized operator can identify the candidate and recovery procedure. H4 must be granted separately before deployment. First deployment documents why no previous BkpLight image/state exists and supplies a rehearsed safe stop/removal path that preserves backups and source data; it does not waive backup/restore evidence. No release date is promised.

## Evidence and approval

For every completed milestone link the exact candidate, commands/procedures, expected/actual results, review disposition and acceptance owner/date. Evidence states are pass/fail/blocked/not_run/not_applicable with rationale; every check above is currently not_run. Detailed plans use per-task TDD and review gates, not one test phase after all coding. TODO: record actual owners and evidence only when available.

## Related documentation

- [Roadmap](roadmap.md)
- [Dependencies and prerequisite register](task-dependencies.md)
- [Implementation plan boundaries](implementation-plans/README.md)
- [Approved architecture](../02-architecture/system-overview.md)
- [Acceptance criteria](../01-requirements/acceptance-criteria.md)
- [Completed milestones](../05-progress/completed-milestones.md)
