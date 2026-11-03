# M02 Encrypted Local Backups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. **Draft: do not execute until the M02 fixture matrix, exact dependency versions, acceptance checks and H2 plan/mode approval are recorded.**

**Goal:** Produce bounded, encrypted local PostgreSQL and standalone MongoDB archives for selected databases while applications continue writing.

**Architecture:** Reuse the M01 daemon, private SQLite owner, strict YAML and supervised native-tool runner. Each database produces a separate age-encrypted `.partial` artifact; successful producer exit, durable finalize and a committed catalog record are required before it becomes staged. Remote upload, restore, schedules, retention and notifications remain later milestones.

**Tech Stack:** Go on Linux, SQLite, native `pg_dump` and `mongodump`, maintained age Go library, native WSL and disposable Docker fixture server. Exact source/tool/library/container versions: TODO after non-secret inventory and compatibility review.

**Spec:** [Approved architecture](../../02-architecture/system-overview.md), [database design](../../02-architecture/database-design.md), [security design](../../02-architecture/security-design.md), [M02 milestone](../milestones.md#m02---online-encrypted-database-archives-p0), [PRD](../../01-requirements/PRD.md).

**Document status:** Draft, 2026-11-03. The user authorized continuing M02 and confirmed a disposable test server with permission to create temporary database containers and data. M01's H2 does not approve M02 application code. No server access has been attempted; a credential shared in chat must be rotated and replaced with private access outside chat. No credentials, host address or customer data belong in this plan.

## Global constraints and entry gates

- Preserve the M01 public behavior and its truthful `not_implemented` response for unfinished endpoints. Enable only M02 functionality that has an end-to-end test.
- One artifact per selected database; optional PostgreSQL globals are a separate explicit scope. Never silently turn a partial dump into success.
- PostgreSQL consistency label is per database. Standalone MongoDB live dumps carry `mongo_live_not_guaranteed` through history and later verification; no oplog or point-in-time claim.
- Stream plaintext from the native producer to age encryption in memory. Persist encrypted bytes only, under a private staging directory and a configured aggregate cap plus free-space reserve.
- The existing `process.Runner` requires canonical allowlisted executable paths, a complete explicit environment and a cancellation-aware stdout writer. Keep credentials out of arguments, logs, events, manifests and errors.
- Keep source/database locks and global heavy-work ownership until the producer exits and is reaped. A failed database must not erase a sibling's completed artifact.
- Before execution: record tested PostgreSQL/MongoDB server and tool versions, image digests, architecture, fixture network, resource budget, least-privilege accounts and exact age library pin. Production versions/sizes remain TODO and are not inferred from disposable fixtures.
- H2 must name this plan revision and select subagent-driven-development or executing-plans. Use a fresh M02 worktree and baseline verification. No push, merge or deployment is covered by H2.

## Review focus

1. A producer exits zero after emitting truncated or semantically incomplete data: archive checks and privilege/RLS evidence must reject false success.
2. A writer reaches the byte cap or filesystem free-space threshold mid-stream: cancellation and reaping must finish without retaining plaintext or marking staged.
3. A crash falls between file rename and catalog commit: restart reconciles by verified identity/hash and never creates an untracked success.
4. A secret or URI appears in native stderr or an exception: public error, event and log schemas expose only safe codes and IDs.
5. Two jobs address the same source/database or share the staging filesystem: serialization and accounting keep the aggregate cap valid.

## File map and task register

| Task | Status | Dependencies | Deliverable | Commit |
|---|---|---|---|---|
| T01 Fixture and compatibility gate | blocked | non-secret inventory; private access | tested engine/tool matrix and synthetic fixture harness | TODO |
| T02 Encrypted staging and budget | backlog | T01 | bounded ciphertext writer, private partial/finalize lifecycle | TODO |
| T03 Artifact catalog and recovery | backlog | T02 | SQLite migration, atomic state transitions, restart reconciliation | TODO |
| T04 PostgreSQL selected-database producer | backlog | T01-T03 | custom-format dumps with version/permission/scope evidence | TODO |
| T05 Standalone MongoDB producer | backlog | T01-T03 | archive/gzip dumps with explicit live-consistency limit | TODO |
| T06 M02 run orchestration and local control | backlog | T04-T05 | manual backup run, per-database outcomes and history | TODO |
| T07 Integrated acceptance and review | backlog | T06 | end-to-end, fault and resource evidence | TODO |

## Task 01 — Fixture and compatibility gate

**Files:** Create `tests/integration/backup_fixture_linux_test.go` and `docs/06-testing/m02-compatibility-matrix.md`; update `docs/03-planning/development-environment.md` with repeatable non-secret fixture commands. No database installer or production Docker files are part of this task.

**Interfaces:** Fixture helper returns source endpoints, disposable credentials-file paths, exact server/tool versions, and cleanup handles. It must not return connection strings containing secrets to logs.

- [ ] Record PostgreSQL and MongoDB fixture image digests, their server and dump-tool versions, architecture, Docker networks, database sizes and sample data shape. Compare against official tool compatibility documentation.
- [ ] Write failing tests for private-only network reachability, online writes during dump, unsupported client/server combinations, and required permissions. Establish safe failure signatures before production adapter code.
- [ ] Build the fixture harness using the disposable server only after the rotated credential and private SSH access exist; use local native WSL fixtures meanwhile. The fixture must create uniquely named temporary containers/volumes and remove only resources it created.
- [ ] Run: `go test ./tests/integration -run TestBackupFixture -count=1 -timeout 120s` from the native Linux snapshot after sync. Record actual output and skip reasons; a skipped fixture is not a pass.
- [ ] Review the exact staged diff and secret scan; commit `test(m02): qualify database fixture matrix`.

**Exit:** A reviewed, reproducible matrix exists. Unknown production versions stay explicitly unknown; fixture compatibility does not imply production support.

## Task 02 — Encrypted staging and aggregate budget

**Files:** Create `internal/archive/staging_linux.go`, `internal/archive/staging_linux_test.go`, `internal/archive/encrypt.go`, `internal/archive/encrypt_test.go`. Modify `go.mod`/`go.sum` only for the reviewed age library pin. Extend the existing config tests only if a new field is truly needed.

**Interfaces:** `Stager.Begin(ctx context.Context, artifactID string, reserveBytes int64) (io.WriteCloser, error)` returns a cancellation-aware writer; `Finalize(ctx context.Context, artifactID string) (ArtifactFile, error)` returns relative path, ciphertext size and SHA-256 only after file sync and atomic rename; `Abort(ctx context.Context, artifactID string) error` records/cleans only that partial. Exact type signatures and capacity reservation ownership must be finalized in the H2 revision after reviewing the M01 process runner.

- [ ] Write failing tests for ciphertext-only persistence, valid age decryption with a fixture identity, wrong-recipient failure, producer-write error propagation, cap/free-space exhaustion, concurrent writers, private permissions, symlink/ancestor substitution and rename/fsync interruption.
- [ ] Run: `go test ./internal/archive -count=1 -timeout 120s`; expect RED for missing behavior.
- [ ] Implement streaming age encryption and byte accounting with a single owner for all active and retained staging files. Treat disk free-space checks as an additional limit, not a substitute for actual byte accounting.
- [ ] Rerun focused tests and `go test -race ./internal/archive -count=1 -timeout 180s`; expect PASS.
- [ ] Review scoped diff and staged bytes for plaintext/secrets; commit `feat(archive): add bounded encrypted staging`.

**Exit:** No plaintext staging file exists, failure cannot finalize, and concurrent writers cannot exceed the configured aggregate budget.

## Task 03 — Artifact catalog and restart reconciliation

**Files:** Create `internal/state/migrations/002_artifacts.sql`, `internal/state/artifacts.go`, `internal/state/artifacts_test.go`, `internal/archive/reconcile_linux.go`, `internal/archive/reconcile_linux_test.go`; modify `internal/state/types.go` and startup recovery only where required.

**Interfaces:** Catalog methods persist writing intent, one database scope, tool version, consistency label, capture interval, key ID, relative path, size and ciphertext hash. A transition to `staged` requires successful file finalization. Preserve M01 `runs` and `events` contracts.

- [ ] Write failing migration and state tests for one artifact per selected database, separate globals scope, stable IDs, invalid transitions, rollback on disk/database errors, unknown newer schema and restart with partial/orphan/renamed files.
- [ ] Run: `go test ./internal/state ./internal/archive -count=1 -timeout 120s`; expect RED.
- [ ] Add migration and reconciliation logic. A surviving completed file is reusable only after manifest, size and hash checks; otherwise mark interrupted/quarantine within the managed directory.
- [ ] Rerun focused and race tests; verify migration backup/recovery from v1 state. Review and commit `feat(state): catalog encrypted backup artifacts`.

**Exit:** Catalog success and on-disk state agree after crash recovery; neither a `.partial` nor a missing file is reported staged.

## Task 04 — PostgreSQL producer

**Files:** Create `internal/backup/postgres.go`, `internal/backup/postgres_test.go`; extend `tests/integration/backup_fixture_linux_test.go` and `docs/06-testing/m02-compatibility-matrix.md`.

**Interfaces:** Producer consumes an immutable source/job/database selection and the approved `process.Runner`; it streams `pg_dump -Fc` output into Task 02's writer and returns version, selected scope, capture interval and a safe failure code. Globals export is a separate opt-in artifact with its own privilege check.

- [ ] Write failing tests for selected databases only, custom-format output, exact version preflight, live writes, inadequate privileges including RLS, globals disabled by default, lost connection, nonzero exit, cancellation and redacted stderr.
- [ ] Run: `go test ./internal/backup -run TestPostgres -count=1 -timeout 120s`; expect RED.
- [ ] Implement allowlisted native args and a complete minimal environment using private credential files. Never put a password or URI with credentials in args or messages.
- [ ] Rerun unit and fixture integration tests, then review and commit `feat(backup): export selected PostgreSQL databases`.

**Exit:** A valid per-database custom archive is staged only after the producer succeeds; unsupported/incomplete exports fail visibly.

## Task 05 — Standalone MongoDB producer

**Files:** Create `internal/backup/mongo.go`, `internal/backup/mongo_test.go`; extend `tests/integration/backup_fixture_linux_test.go` and the compatibility matrix.

**Interfaces:** Producer streams `mongodump --archive --gzip` for one configured database into the encrypted writer. Result consistency is always `mongo_live_not_guaranteed` for the confirmed standalone topology.

- [ ] Write failing tests for selected databases only, online writes, archive/gzip output, absence of oplog mode, unsupported topology/tool versions, auth failure, cancellation, redacted diagnostics and persistent consistency label.
- [ ] Run: `go test ./internal/backup -run TestMongo -count=1 -timeout 120s`; expect RED.
- [ ] Implement native invocation via the M01 runner; keep credentials out of arguments and persistent output. Do not claim point-in-time consistency or full server recovery.
- [ ] Rerun unit and fixture integration tests, then review and commit `feat(backup): export selected standalone MongoDB databases`.

**Exit:** The local archive can be opened by the matching fixture tool; metadata still discloses live-backup limits.

## Task 06 — Manual run orchestration and local control

**Files:** Create `internal/backup/service.go`, `internal/backup/service_test.go`; modify `internal/app/serve_linux.go`, `internal/app/health_linux.go`, `internal/control/server_linux.go`, `internal/control/types.go`, `internal/cli/run.go` and their tests only for the M02 manual backup path.

**Interfaces:** `POST /v1/runs` and `bkplight backup run` accept a configured job and idempotency request ID, then expose one run and separate per-database artifact outcomes. Existing GET/list/cancel contracts remain compatible. No scheduler or remote delivery is implied by a locally staged result.

- [ ] Write failing tests for disabled/unknown job, replayed request ID, two selected databases with one failure, source lock, global heavy-work limit, cancellation, shutdown/restart, aggregate cap and truthful health/history.
- [ ] Run: `go test ./internal/backup ./internal/app ./internal/control ./internal/cli -count=1 -timeout 120s`; expect RED.
- [ ] Wire only validated manual backup execution. Decide in the H2 revision whether M01's `dumping -> delivering` state needs a compatible new local-staged state or an explicit local-only terminal result; add an ADR if the public state contract changes materially. Do not label local staging as remotely delivered success.
- [ ] Rerun focused tests and race tests, review diff/API compatibility, commit `feat(app): run local encrypted backups`.

**Exit:** Manual requests yield truthful per-database local artifact history, and all unfinished operations fail or interrupt visibly.

## Task 07 — Integrated acceptance, review and verification

**Files:** Update `docs/06-testing/m02-foundation-evidence.md`, `docs/05-progress/current-status.md`, `docs/08-agent-memory/session-handoff.md`, `README.md` and only necessary acceptance documentation.

- [ ] Use disposable fixtures for PostgreSQL and MongoDB with ongoing writes. Exercise success, wrong permissions, lost network, producer crash, encryption error, full disk, concurrent jobs, restart with partials, and sibling success after one database failure.
- [ ] Verify encrypted archives with a private fixture identity and native archive inspection. This is archive validation, not an isolated restore-verification claim; M04 owns restore verification.
- [ ] Run native Linux `go test ./... -count=1 -timeout 120s`, `go test -race ./... -count=1 -timeout 180s`, `go vet ./...`, `go mod verify`, `go build ./cmd/bkplight`, plus bounded fault tests. Increase timeouts only with recorded cause and observed limits.
- [ ] Perform independent architecture/security/code review, fix findings with scoped regression tests and re-review. Update task hashes, exact evidence, known limits and handoff. Commit `docs(m02): record encrypted local backup evidence`.

**Exit:** M02 milestone exit criteria have observed evidence for the tested matrix. This is neither production inventory nor authorization to merge or deploy.

## Current blockers and approval

1. **Private access:** rotate the chat-disclosed password and configure a private SSH key outside chat before using the disposable server. The server is confirmed disposable; no remote connection or mutation has yet occurred.
2. **Compatibility:** record actual production source versions/sizes separately from disposable fixture versions. Select and verify exact native tool, age library and image pins before H2.
3. **Contract decisions:** resolve the M02 local-staged run state and artifact manifest schema in a reviewed H2 revision; keep later M03 remote commit semantics separate.
4. **Human gate:** approve the exact revised M02 plan and execution mode under H2. The instruction to continue M02 authorizes planning and preparation but does not waive the repo's explicit plan/mode gate.

## Related documentation

- [Development lifecycle](../development-lifecycle.md)
- [Git workflow](../git-workflow.md)
- [Milestone register](../milestones.md)
- [Open questions](../../01-requirements/open-questions.md)
- [Current status](../../05-progress/current-status.md)
