# M02 local execution evidence

**Document status:** Partial verified evidence, 2026-11-03. T02-T03 local primitives only; M02 milestone incomplete.

## Purpose and scope

Record commands observed on the isolated M02 branch. Evidence uses synthetic local data in a private native Ubuntu WSL snapshot. No PostgreSQL/MongoDB tool, server, live write, remote destination, restore or production operation was exercised.

## Baseline and access

The managed checkout was already a linked Git worktree on `feature/m02-backup-plan`; no new worktree was created. The existing native WSL sync helper refreshed `/home/hemal/.bkplight-m01-fixture-eba7f1c` before each Go command. Baseline `GOTOOLCHAIN=local TMPDIR=/tmp go test ./... -count=1 -timeout 120s` passed all existing packages and integration before M02 edits.

The user confirmed a disposable test server and authorized temporary database containers/data there. A read-only, password-free SSH probe was denied. The chat-disclosed credential was not used. Local Docker did not run; `pg_dump` and `mongodump` were absent from WSL. No remote server mutation occurred. T01 compatibility and online-write evidence remain blocked.

## T02 bounded encrypted staging — `0d42a51`

Observed RED tests: `NewEncryptWriter` undefined; `OpenStager`/`Options` undefined; duplicate final ID accepted; ancestor-swap acquisition unsupported; cancellation API missing; a synthetic post-rename directory-sync failure had no seam. One initial GREEN run exposed a real mutex deadlock because age writes its header during writer construction while the budget mutex was held; the stack trace identified the cycle, and the fix released the mutex before constructing the age writer.

Observed GREEN: `go test ./internal/archive -count=1 -timeout 120s`, `go test -race ./internal/archive -count=1 -timeout 180s`, and full `go test ./... -count=1 -timeout 120s` passed. `go vet ./...`, `go build ./cmd/bkplight` and `go mod verify` passed; the last reported `all modules verified`. Tests cover ciphertext-only persistence/decryption, wrong identity, private file/directory modes, cap and free-space refusal, aggregate reservations and parallel writers, cancellation, duplicate ID, restart accounting, symlink/ancestor substitution, aborted partials and post-rename sync failure. Official age v1.3.2 is pinned in `go.mod`.

## T03 artifact catalog and recovery — `a1cbc5d`

Observed RED tests: missing artifact intent/stage APIs; missing ciphertext verifier and verified-receipt stage contract; missing tamper reconciliation and partial/orphan handling; manifest writer accepted a nonexistent archive receipt; database trigger failure was mislabeled conflict. GREEN regressions followed each correction.

SQLite migration `002_artifacts.sql` adds one artifact per run/source/scope, distinct globals scope, explicit consistency label and ciphertext/manifest receipts. A real v1 schema fixture upgraded to v2 with a pre-upgrade state backup. Staging requires both actual encrypted files to match size/hash; missing or tampered staged files lose eligibility on reconciliation. Versioned manifest JSON is encrypted separately and binds scope, consistency, capture interval and archive hash/size. Incomplete `.partial` files are removed after interruption; uncataloged final ciphertext is renamed `.orphan` and retained under the disk cap. The M01 run history contract remains unchanged. T06 must call recovery in daemon startup before backup capability is enabled.

Final observed checks after T03: `go test ./internal/archive ./internal/state -count=1 -timeout 120s`, `go test -race ./internal/archive ./internal/state -count=1 -timeout 180s`, full `go test ./... -count=1 -timeout 120s`, `go vet ./...`, `go build ./cmd/bkplight` and `go mod verify` passed. Staged diffs passed `git diff --cached --check` and a scoped scan for the supplied server address/password and obvious embedded credentials before each task commit. The official `task-done` script recorded passing focused tests and commit ranges in the ignored M02 ledger.

## Limits and next checks

These checks prove local encrypted artifact and metadata behavior only. They do not prove native dump compatibility, online-write consistency, least-privilege access, end-to-end backup/restore or production readiness. T01 must establish a real engine/tool matrix and test fixtures before T04-T07 execute. An independent whole-branch review remains required after all M02 tasks; no merge or deployment approval exists.

## Related documentation

- [M02 implementation plan](../03-planning/implementation-plans/2026-11-03-m02-encrypted-local-backups.md)
- [Current status](../05-progress/current-status.md)
- [Quality gates](quality-gates.md)
