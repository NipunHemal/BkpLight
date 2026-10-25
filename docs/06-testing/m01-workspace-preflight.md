# Milestone 1 workspace preflight

**Document status:** WSL toolchain installed and smoke-tested 2026-10-25; native test arrangement and fixture scope remain unresolved. Earlier preflight evidence is historical.

## WSL execution choice confirmed - 2026-10-25

The user said "ok can you use wsl". Use Ubuntu WSL for M01 commands, retaining the managed worktree as the source/Git authority. Fresh non-root checks report Go 1.26.10 linux/amd64, CGO_ENABLED=1 and GCC 13.3.0. An initial probe used the wrong conventional Go path; the documented `/usr/local/bin/go` resolves correctly to `/opt/bkplight/toolchains/go1.26.10/bin/go`. No reinstall is needed.

Use native `/tmp` for Go temporary files and permission-sensitive fixture data (`TMPDIR=/tmp`, `GOTOOLCHAIN=local`). Source may be read from the mounted managed checkout; tests requiring restrictive modes for source-adjacent files must use a native snapshot with recorded source hashes. Do not weaken checks to accept mounted Windows permissions. Fresh self-cleaning probes passed 0700 directory/0600 file and socket modes, flock contention/release and Unix-socket exchange as UID 1000. These are environment checks, not product tests.

Execution remains stopped before T01: the explicit M01-only synthetic-fixture disposition is still outstanding, and the previously installed Superpowers skill path is now absent. Search of the Codex plugin cache and user skill directories found no Superpowers skill files; the current skill catalog also omits them. Restore the requested official skill before claiming that workflow runs. Existing H2 plan/subagent authorization remains valid. No application source or dependencies were added.

## WSL tool installation - 2026-10-25

The user authorized using WSL and installing the needed tools. Ubuntu installation completed using the WSL root account because passwordless sudo was unavailable; runtime checks ran as the existing user, UID 1000. No sudo/password policy was changed.

- Installed `build-essential` 12.10ubuntu1 and its dependencies; GCC 13.3.0 and GNU Make 4.3 verified. `ca-certificates` updated to 20260816ubuntu0.24.04.1. No general distribution upgrade was performed.
- Installed Go 1.26.10 linux/amd64 at `/opt/bkplight/toolchains/go1.26.10`, with new `/usr/local/bin/go` and `/usr/local/bin/gofmt` symlinks. Existing executable paths were checked before creation; no existing Go installation was overwritten.
- Official download metadata from `https://go.dev/dl/?mode=json&include=all` supplied SHA-256 `ccfe7ae822d2c2f41f4fe7c956199967a4dac22d0ee0d2b2bfdb440aab3b20c5`; downloaded archive matched before extraction.
- `go version`: `go version go1.26.10 linux/amd64`. `GOTOOLCHAIN=local go env GOOS GOARCH CGO_ENABLED`: linux, amd64, 1.
- Non-root C compile/execution and a temporary Go concurrency smoke test using `go test -race -count=1 -v` passed. The temporary Go test uses the standard library only and is not an application acceptance test.
- Native Linux `/tmp` smoke checks passed: directory 0700, file/socket 0600, exclusive flock contention/release and Unix-socket message exchange. Temporary probe files were removed by their scoped temporary-directory contexts.
- The same permission probe in the Windows-mounted managed checkout returned 0777 for both file and directory after chmod to 0600/0700. This checkout is unsuitable for permission-sensitive test data. Use native Linux test storage or a native execution snapshot; do not weaken assertions or remount the host filesystem silently.

The toolchain blocker is resolved. Remaining execution gates: establish the native Linux test execution arrangement and explicitly record the M01 synthetic-fixture disposition while M00 production inventory stays incomplete. H2 plan/subagent approval persists. No application code, module, project dependency installation, merge or deployment occurred. No application tests are claimed as passing.

## Purpose

Record isolated workspace setup and baseline evidence before application changes.

## Authority and Git baseline

The user explicitly requested Superpowers using-git-worktrees to prepare an isolated M01 workspace and verify dependencies, environment and baseline tests before changing code. This supersedes the earlier restriction on worktree creation for this preparation only. It does not select an execution mode, approve package installation or authorize feature implementation, merge or deployment.

- Task: WT-M01, preparation complete; implementation prerequisites unresolved.
- Managed workspace: `C:/Users/USER/.codex/worktrees/m01-secure-foundation/BkpLight`.
- Branch: `feature/m01-secure-foundation`.
- Base: `2aa208057f4ad6a44bf96260c55710126fd55a31` (detailed M01 plan).
- Original checkout: `D:/Project/BkpLight`, branch `feature/m01-implementation-plan`, clean at inspection.
- Stable main remains `7a5ba9e3020ecc2bd9e3af8687cbdd7183195669`.
- Native managed-worktree creation and attachment succeeded; no source worktree changes were copied or discarded. No merge, push or remote configuration.

## Environment observations

| Check | Observed result | Implication |
|---|---|---|
| Windows `go version` | go1.24.5 windows/amd64 | Does not meet planned Go 1.26.10/Linux baseline |
| Windows compiler discovery | No cc/gcc/clang found on PATH | No demonstrated local race-test compiler |
| `wsl --list --verbose` | Ubuntu, WSL 2; initially stopped | Existing Linux environment available |
| Ubuntu `uname -srm`, OS release | Linux 6.6.87.2-microsoft-standard-WSL2 x86_64; Ubuntu 24.04.3 LTS | Linux amd64 available; not production-target qualification |
| Ubuntu `id -u` | 1000 | Non-root session |
| Ubuntu executable discovery | Git 2.43.0; no go/cc/gcc on noninteractive login PATH | Go and C compiler must be supplied before Linux tests |
| `stat -f -c %T /home /mnt/c` | ext2/ext3; v9fs | Managed checkout is Windows-backed; qualify required permissions/socket/flock semantics or choose a separately approved native Linux execution location |

WSL inspection started the existing distribution; no packages, configuration or application services were installed. No Docker daemon or production endpoint was contacted. Missing PATH entries do not prove software is absent everywhere on disk.

## Dependency verification

Read-only HTTP requests to the official Go module proxy returned 200 for the exact pinned manifests:

| Manifest | Verified metadata |
|---|---|
| [YAML v3.0.5](https://proxy.golang.org/go.yaml.in/yaml/v3/@v/v3.0.5.mod) | module go.yaml.in/yaml/v3; go 1.16 |
| [SQLite v1.61.0](https://proxy.golang.org/modernc.org/sqlite/@v/v1.61.0.mod) | module modernc.org/sqlite; go 1.26.0; modernc.org/libc v1.79.0 |
| [Cron v3.0.1](https://proxy.golang.org/github.com/robfig/cron/v3/@v/v3.0.1.mod) | module github.com/robfig/cron/v3; go 1.12 |

The [official Go release history](https://go.dev/doc/devel/release) lists Go 1.26.10. These checks establish publication and declared requirements only. No module graph was resolved, package archive installed, checksum/license audit completed or runtime SQLite version verified. Those checks remain pending until an authorized module/toolchain exists. Pins were not changed.

## Baseline and limitations

Repository inspection found documentation, skills and .gitignore only: no go.mod/go.sum, Go source, executable tests, Docker/CI configuration or other package manifest. Therefore `go test ./...`, build, vet, race and integration tests are **not applicable to this baseline**, not passing. Do not create a module or placeholder tests just to manufacture a green preflight.

Documentation links, file inventory, Git whitespace and scoped staged content are checked for this preparation commit; results are recorded in the final task report. The base contained 47 Markdown files. This preparation adds this report and updates only agent instructions, navigation, status and handoff.

Sandbox attempts to list WSL, access module-proxy DNS and resolve managed-worktree Git status initially failed. Authorized elevated read-only retries succeeded. These are tooling restrictions, not application test failures; no security check was bypassed.

## Remaining prerequisites and next step

- TODO: approve/provide Go 1.26.10 and a C compiler in the Linux fixture environment, and qualify filesystem behavior. Do not silently use Windows tests as Linux evidence.
- TODO: resolve M00 or explicitly authorize the M01-only fixture disposition described by the plan.
- TODO: approve the exact implementation plan and choose inline or subagent execution before T01. Current instruction approves workspace preparation only.
- Resolve dependencies/checksums and run the plan's meaningful tests after the appropriate files exist under authorized implementation; record failures and evidence per task.
- T01-T09 remain backlog; H3 and H4 remain absent. No product acceptance or production readiness is claimed.

## Related documentation

- [M01 plan](../03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md)
- [Git workflow](../03-planning/git-workflow.md)
- [Current status](../05-progress/current-status.md)
- [Handoff](../08-agent-memory/session-handoff.md)
