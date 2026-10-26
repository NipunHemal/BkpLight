# M01 foundation fixture evidence

**Status:** M01 T01-T09 done on the isolated feature branch, 2026-10-26. Whole-branch review and one scoped fix re-review closed. Integration/release not_started; H3/H4 absent.

Candidate: `feature/m01-secure-foundation`, managed worktree `C:\Users\USER\.codex\worktrees\m01-secure-foundation\BkpLight`. T09 implementation `07635a2ce01e0e71caa75cb9134a49f14baa31f2`, task-review fix `dd8fae9b102994092d14a6aa2a352ab63c927b64`, whole-branch review fix `02e84c3b267a534bb6e74bcbd1fab6bd51730b94`. The [task register](../03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md#task-register-and-order) records earlier task/fix hashes.

| Task | Implementation and reviewed fixes | Final state |
|---|---|---|
| T01 | `99248f5` | done |
| T02 | `18a2e4c`, `6975a64` | done |
| T03 | `26500db` | done |
| T04 | `1b04bc0`, `14e653a` | done; T09 diagnostic extension reviewed |
| T05 | `16bcdfc`, `e8a49c1` | done |
| T06 | `072cd38` | done |
| T07 | `3f6e7cd` | done |
| T08 | `c4e89f0`, `0430582` | done; final-review path-length fix in `02e84c3` |
| T09 | `07635a2`, `dd8fae9`, `02e84c3` | done; independent task, whole-branch and fix re-review closed |

## Scope and execution rulings

The daemon decodes/validates before mutation, rejects resolved directory overlaps, checks active secret references and clears returned bytes, acquires singleton SQLite ownership/migrations, opens the safe log sink, recovers only supported interrupted local runs, activates the immutable config, probes durable writes and publishes local Unix control. Shutdown stops readiness/admission, drains HTTP for at most five seconds, removes only the owned socket, closes logs and closes state with the singleton lock released last.

Health reports `scope: foundation`, `scheduler: not_implemented`, `backup_engine: not_implemented`. It serializes and caches state/log probes for at most one second. Cancel checks readiness before the transactional state method. A failed sink or write probe makes readiness false; diagnostics use fixed codes.

Controller ruling on 2026-10-26 explicitly extends the T09 file map by `internal/observability/logger.go` and `logger_test.go`: allowlist only the fixed internal `service.health_checked` diagnostic, emitted at most once per second when health is requested. Existing allowed codes would misdescribe a health probe. No arbitrary message/attribute or external event was added. The daemon starts **zero native helpers**; `TestShutdownReapsOwnedHelper` launches a fixture-owned test executable with the same parent cancellation context as Serve, verifies cancellation/join and PID absence, and does not claim a production helper registry was exercised.

## Native commands and observed results

All commands ran 2026-10-26 UTC as UID 1000 in the private native WSL snapshot with `GOTOOLCHAIN=local TMPDIR=/tmp`; the environment snapshot was captured at 00:26:25Z and final verification by 00:35Z. The approved ignored sync helper ran successfully before each Go command (95 files at the source-complete test phase, 97 after adding the evidence/development documents). Tool environment: Go 1.26.10 linux/amd64, CGO enabled, GCC 13.3.0, kernel `6.6.87.2-microsoft-standard-WSL2`, filesystem reported `ext2/ext3`. SQLite runtime 3.54.1 is asserted by store startup and the existing state tests. Exact setup and repeatable commands are in the [development environment](../03-planning/development-environment.md).

| Command | Observed result |
|---|---|
| `go test ./internal/app -run TestInvalidConfigCreatesNoState -count=1 -v` before implementation | RED: missing Serve, then behavioral RED `invalid config started` against an empty boundary |
| `go test ./tests/integration -run TestFoundationLifecycle -count=1 -v` before composition | RED: actual binary exited 2 with `invalid_config: command` |
| `go test ./internal/observability -run TestHealthCheckedDiagnosticUsesClosedSchema -count=1 -v` before allowlist extension | RED: `safe health diagnostic rejected invalid_config` |
| `go test ./internal/app ./internal/observability ./internal/cli ./tests/integration -count=1 -timeout 120s -v` | Initial run exposed fixture mistakes: explicit YAML min_successful zero, incorrect client error-body/route expectations; corrected fixtures, no production contract changes |
| `go test ./internal/app -count=1 -timeout 30s -v` | PASS: six tests, 0.132s; helper initially lacked required trusted working directory, corrected fixture and verified actual PID absence |
| `go test ./internal/... -count=1 -timeout 120s` | PASS all nine internal packages; control 7.086s, state 2.470s |
| `CGO_ENABLED=1 go test -race ./internal/... -count=1 -timeout 180s` | PASS all nine packages; no race reports |
| `go test ./internal/config -run '^$' -fuzz FuzzDecode -fuzztime 10s` | PASS, 25,083 executions, 49 new interesting inputs, 11.202s |
| `go test ./tests/integration -count=1 -timeout 120s -v` | PASS all 12 named tests plus stale-socket subtests, 4.237s |
| `unshare --user --map-current-user --net sh -c 'GOTOOLCHAIN=local TMPDIR=/tmp go test ./tests/integration -count=1 -timeout 120s'` | PASS 4.075s; isolated namespace, UID 1000 maps to host UID 1000, no configured routes |
| `go mod verify` | PASS: `all modules verified` |
| `go list -m all` | PASS; reviewed direct pins yaml v3.0.5, cron v3.0.1, SQLite v1.61.0 and locked transitive modules; no dependency changes |
| `go vet ./...` | PASS, no output |

Self-review found that a resolved secret-root alias to `/` was not covered by the ordinary descendant-prefix predicate. `go test ./internal/app -run TestResolvedSecretRootAliasPreventsStartup -count=1 -v` directly reproduced RED (`resolved secret root accepted`). The predicate now recognizes filesystem root as containing every absolute path, so startup rejects the alias before mutation. The final suite includes the GREEN regression. Fixture cleanup was also hardened to join Serve/Runner and close seed stores even on failure.

| Final source check | Result after self-review correction |
|---|---|
| `go test ./... -count=1 -timeout 120s` | PASS all nine internal packages and integration; final app 0.164s, control 7.093s, state 2.179s, integration 4.098s. Seven app and 12 real-binary integration named tests execute; `cmd/bkplight` has no direct test files and is exercised by reexec fixtures |
| `CGO_ENABLED=1 go test -race ./internal/... -count=1 -timeout 180s` | PASS all nine internal packages; app 1.268s, control 8.292s, state 4.329s |
| `go vet ./...` | PASS, no output |
| `go build -trimpath -o bin/bkplight ./cmd/bkplight` | PASS |
| `./bin/bkplight version` | PASS: `bkplight dev` |
| `./bin/bkplight config validate --config examples/bkplight.yaml` | PASS: `valid 6b2ffc6559840bb121e6f1c69b3dabe49a0deebb46acb87969539292b4583b9c` |
| Isolated-network integration command above | PASS after correction, 3.949s |
| `gofmt -l cmd internal tests`, `git diff --check`, `git diff -- go.mod go.sum` | Empty output; no formatting/whitespace errors or dependency changes |
| Owned Markdown local-link check | PASS, 152 resolving path links across seven task-owned documents (anchors not separately validated) |

The exact staged name/stat/full-diff, secrets and whitespace review is recorded in the ignored task report before commit. Review fixes require fresh affected verification and a subsequent evidence entry; these results do not establish independent review approval.

## T09 independent review fix round 1 - 2026-10-26

Review identified an Important cross-state startup race: final stale-inode checking and removal were separate, so two daemons could validate one stale socket and a delayed removal could unlink the other's newly bound listener. Separate SQLite state locks did not serialize that path. The fix adds a private, same-parent `.<socket-name>.lock` inode with nonblocking flock, owned mode 0600, regular-file/single-link/no-symlink checks and an inode identity check after acquisition. The file is never unlinked. The held runtime parent and claim span stale recovery, bind, serving and `control.Server.Close`; state ownership is still acquired first and released last. Stale operations remain descriptor-relative. Startup verifies the configured parent and the created socket match the held runtime directory before serving; replacement aborts safely. No T08 control-package contract changed.

Deterministic RED: `go test ./internal/app -run TestConcurrentStaleRecoveryKeepsWinnerListener -count=1 -v` failed `second claimant acquired stale socket before winner bound <nil>` before the lock was implemented. GREEN passed 0.038s. The test pauses the winning claim before stale recovery/bind, attempts a concurrent claim, proves the stale inode is preserved, binds/requests the winner, repeats a losing claim without losing liveness, verifies the claim survives socket cleanup, then proves the next startup can acquire it after owner close. Additional app tests reject symlink/hardlink/public-mode lock files without altering them and reject a replaced runtime parent. `TestConcurrentDaemonsRecoverStaleSocket` starts two real binaries with separate state directories and a shared stale socket; exactly one wins, stays reachable, shuts down and leaves a private reusable lock inode; the loser configuration can start afterward.

| Fix verification command | Observed result |
|---|---|
| `go test ./internal/app ./tests/integration -run 'TestConcurrent\|TestRuntimeClaim\|TestStaleSocketSafety\|TestFoundationLifecycle' -count=1 -timeout 60s -v` | PASS: deterministic ownership, unsafe-lock subtests, parent replacement, lifecycle/stale-safety and actual competing daemons; app 0.060s, integration 0.883s. The command used regex alternation without backslash escaping in the shell |
| `go test ./... -count=1 -timeout 120s` | PASS all internal packages/integration; app 0.262s, control 7.086s, state 2.748s, integration 4.344s |
| `CGO_ENABLED=1 go test -race ./internal/app ./tests/integration -count=1 -timeout 120s` | PASS: app 1.325s, integration 5.276s; no race reports |
| `go test ./internal/app ./tests/integration -run TestConcurrent -count=20 -timeout 60s` | PASS: app 0.495s, integration 1.803s; 20 repetitions of each concurrent regression |
| `go vet ./...`, `go build -trimpath -o bin/bkplight ./cmd/bkplight` | PASS, no output |
| Isolated-network integration command recorded above | PASS 4.238s after the runtime ownership fix |
| `gofmt -l cmd internal tests`, `git diff --check` | Empty output |

All fix commands ran in the same non-root native WSL snapshot, with the approved source sync before each Go command. Scoped re-review remains pending. The reviewer's separate Minor coalescing-test weakness is deferred to final whole-branch review along with the existing T08 rooted-path-length Minor; this fix round addresses the Important runtime ownership race.

## Acceptance mapping

| Criteria | Evidence |
|---|---|
| AC01 strict/bounded configuration, AC02 credentials/no startup mutation | Config suite/fuzz and secrets suite; app/integration `TestInvalidConfigCreatesNoState`, app `TestResolvedDirectoryOverlapPreventsStartup`/`TestResolvedSecretRootAliasPreventsStartup`, real binary `TestEnabledMissingSecretPreventsListen` |
| AC03 revisions, AC04 singleton/migrations, AC05 transactional idempotency | Existing state unit/race suite; real binary `TestFoundationLifecycle`, `TestDaemonSingleton`; real-store app `TestCancelIdempotency` |
| AC06 local bounded IPC | Existing control/CLI suites; real binary `TestNoTCPListener` compares child socket inodes with its namespace TCP/TCP6 and Unix tables; lifecycle asserts private modes |
| AC07 logs, AC09 truthful readiness | Existing rotation tests and fixed-code regression; real binary `TestHealthDegradesOnStateWriteFailure`, `TestHealthDegradesOnLogFailure`, `TestHealthProbeCoalescesConcurrentRequests`; degraded cancel creates no command row |
| AC08 process supervision, AC11 shutdown/recovery | Existing process/state unit/race fixtures; app helper cancellation/PID absence; real binary SIGINT/SIGTERM cleanup, crash lock release, stale socket safety and `TestCrashRecoveryDoesNotReplayEffects` |
| AC10 no implicit delivery | Existing state routing/outbox tests; `TestNoNotificationsWithoutChannels`, recovery verifies zero delivery rows; `TestUnsupportedCommandsNoMutations` verifies no runs/events/deliveries/commands |
| AC12 reproducibility | Recorded native tests, fuzz, race, vet/build/module verification, example validation and development instructions; whole-branch and scoped fix re-review closed |

## Limits

Synthetic fixtures establish foundation behavior only. No database dump, encrypted archive, provider request, webhook send, restore, scheduled backup, retention execution, Docker image, CI or production release was run or implemented. The integration suite builds a normal daemon binary; internal package race tests exercise the app/state/control implementations with race instrumentation. Network-namespace execution demonstrates these fixtures work without external networking, not a production egress policy or proof against every future code path. Startup probes and periodic requested-health probes are not a backup-protection guarantee. M00 deployment inventory, database/tool/provider qualification and business acceptance remain open. No merge, push or deployment is approved.


## Final whole-branch review fix wave - 2026-10-26

One Important and two Minor findings were reproduced and fixed. Logger acquisition now creates/opens each directory with retained `O_DIRECTORY|O_NOFOLLOW` descriptors and `openat`/`mkdirat`, then acquires `os.Root` through the held `/proc/self/fd` directory. A deterministic regression replaces an ancestor immediately before creation and proves the replacement tree remains untouched. Existing inode, ownership, private-mode and closed-log-schema checks remain active.

Control validates the actual descriptor-relative bind path before inspecting an existing inode or binding: 107-byte paths bind; 108-byte paths return safe invalid. The app also checks its rooted path before lock creation or stale recovery. The health-coalescing integration fixture now expires startup cache, bounds a barrier-released 24-client burst below the next cache interval and requires exactly one new write probe.

All commands below ran in the private non-root native WSL fixture snapshot with `GOTOOLCHAIN=local TMPDIR=/tmp`, with the approved sync helper before every Go command:

| Command | Observed result |
|---|---|
| `go test ./internal/observability -run TestLogDirectoryCreationAnchoredBeforeAncestorSubstitution -count=1 -v` before fix | RED: replacement tree mutated; acquired tree lacked child |
| `go test ./internal/observability -count=1 -v` after fix | PASS all 16 tests |
| Focused rooted-path/app/coalescing tests before fix | RED: unavailable/conflict instead of invalid; runtime lock creation; health delta zero |
| Corrected app path fixture with guard removed | RED: accepted oversize rooted path and created runtime lock |
| Same three focused regressions with `-count=3 -v` | PASS all three repetitions |
| `go test ./... -count=1` | PASS all nine internal packages and real-binary integration (6.441s) |
| `go test -race ./... -count=1` | PASS all nine internal packages and integration (7.959s), no race reports |
| `go vet ./...` | PASS, empty output |
| `go build ./cmd/bkplight` | PASS, empty output |

Fixture corrections are preserved in the ignored final-fix report: the initial ancestor swap happened before validation rather than at mkdir; an initial app temporary parent was too long to guarantee actual rooted-path overflow. Both were corrected and behavioral RED was directly observed before final verification. These results establish fixture behavior, not production inventory, integration or release readiness. The scoped independent re-review closed all three findings with no new breakage; H3/H4 are absent.

## Final review and controller verification - 2026-10-26

Whole-branch review covered base `7a5ba9e3020ecc2bd9e3af8687cbdd7183195669` through `dd8fae9b102994092d14a6aa2a352ab63c927b64` and found the log-directory ancestor race (Important) plus two deferred Minor cases. One fix wave committed `02e84c3b267a534bb6e74bcbd1fab6bd51730b94`; one scoped independent re-review verified all three addressed, no new Critical/Important breakage and no out-of-scope observation. The reviewer did not rerun suites. The initial attempted whole-branch reviewer hit a usage limit before returning findings; a second available reviewer completed the review.

The controller reran commands on the committed fix in the private native WSL snapshot, syncing before each Go command. `go test ./... -count=1 -timeout 120s` passed all nine internal packages and the actual-binary integration package. `CGO_ENABLED=1 go test -race ./... -count=1 -timeout 180s` passed all nine plus integration with no race reports. `go mod verify` reported `all modules verified`; `go list -m all` retained the reviewed direct pins (yaml v3.0.5, cron v3.0.1, SQLite v1.61.0). `go vet ./...` and `go build -trimpath -o bin/bkplight ./cmd/bkplight` passed. Rebuilding and immediately running `./bin/bkplight version` returned `bkplight dev`; example config validation returned `valid 6b2ffc6559840bb121e6f1c69b3dabe49a0deebb46acb87969539292b4583b9c`. One initial version check failed because a later source sync removed ignored `bin/bkplight`; rebuilding and running both binary checks before another sync resolved the verification workflow issue, with no source change. Bounded `FuzzDecode` passed after 27,392 executions and 43 new interesting inputs. Network-isolated integration passed in an unprivileged WSL namespace. `gofmt -l cmd internal tests` was empty.

These results verify the M01 synthetic Linux foundation. They do not establish production readiness, actual database backup/recovery, or H3/H4 authorization.
