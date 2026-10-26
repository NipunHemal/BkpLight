# M01 foundation fixture evidence

**Status:** T09 in_review, 2026-10-26. T01-T08 independently reviewed; T09 and whole-branch independent review remain pending. Integration/release not_started; H3/H4 absent.

Candidate: `feature/m01-secure-foundation`, managed worktree `C:\Users\USER\.codex\worktrees\m01-secure-foundation\BkpLight`, T09 based on `50dd7b7bf5dda147948867ccd3310c300b09c2ed`. The task commit is discoverable as `feat(app): compose secure service foundation`; its actual hash must be recorded in subsequent bookkeeping, because this evidence is part of that commit. The [authoritative task register](../03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md#task-register-and-order) records exact earlier task/fix hashes and review outcomes.

| Task | Implementation and reviewed fixes | State entering T09 review |
|---|---|---|
| T01 | `99248f5` | done |
| T02 | `18a2e4c`, `6975a64` | done |
| T03 | `26500db` | done |
| T04 | `1b04bc0`, `14e653a` | done; narrow T09 diagnostic extension pending review |
| T05 | `16bcdfc`, `e8a49c1` | done |
| T06 | `072cd38` | done |
| T07 | `3f6e7cd` | done |
| T08 | `c4e89f0`, `0430582` | done; Minor rooted socket path-length case deferred to whole-branch review |
| T09 | Current task tree, commit subject above | in_review |

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

## Acceptance mapping

| Criteria | Evidence |
|---|---|
| AC01 strict/bounded configuration, AC02 credentials/no startup mutation | Config suite/fuzz and secrets suite; app/integration `TestInvalidConfigCreatesNoState`, app `TestResolvedDirectoryOverlapPreventsStartup`/`TestResolvedSecretRootAliasPreventsStartup`, real binary `TestEnabledMissingSecretPreventsListen` |
| AC03 revisions, AC04 singleton/migrations, AC05 transactional idempotency | Existing state unit/race suite; real binary `TestFoundationLifecycle`, `TestDaemonSingleton`; real-store app `TestCancelIdempotency` |
| AC06 local bounded IPC | Existing control/CLI suites; real binary `TestNoTCPListener` compares child socket inodes with its namespace TCP/TCP6 and Unix tables; lifecycle asserts private modes |
| AC07 logs, AC09 truthful readiness | Existing rotation tests and fixed-code regression; real binary `TestHealthDegradesOnStateWriteFailure`, `TestHealthDegradesOnLogFailure`, `TestHealthProbeCoalescesConcurrentRequests`; degraded cancel creates no command row |
| AC08 process supervision, AC11 shutdown/recovery | Existing process/state unit/race fixtures; app helper cancellation/PID absence; real binary SIGINT/SIGTERM cleanup, crash lock release, stale socket safety and `TestCrashRecoveryDoesNotReplayEffects` |
| AC10 no implicit delivery | Existing state routing/outbox tests; `TestNoNotificationsWithoutChannels`, recovery verifies zero delivery rows; `TestUnsupportedCommandsNoMutations` verifies no runs/events/deliveries/commands |
| AC12 reproducibility | Recorded native tests, fuzz, race, vet/build/module verification, example validation and development instructions; final independent review still required |

## Limits

Synthetic fixtures establish foundation behavior only. No database dump, encrypted archive, provider request, webhook send, restore, scheduled backup, retention execution, Docker image, CI or production release was run or implemented. The integration suite builds a normal daemon binary; internal package race tests exercise the app/state/control implementations with race instrumentation. Network-namespace execution demonstrates these fixtures work without external networking, not a production egress policy or proof against every future code path. Startup probes and periodic requested-health probes are not a backup-protection guarantee. M00 deployment inventory, database/tool/provider qualification and business acceptance remain open. No merge, push or deployment is approved. T09 independent review and the previously deferred T08 Minor long rooted-bind path case remain with the controller.
