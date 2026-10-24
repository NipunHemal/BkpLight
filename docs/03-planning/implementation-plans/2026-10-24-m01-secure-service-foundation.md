# Milestone 1 Secure Service Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Human H2 approval must select the mode first; this document authorizes neither execution nor delegation.

**Goal:** Build the M01 configuration, secret, metadata, process, logging and local-control foundation without performing database backups or provider operations.

**Architecture:** One Linux Go daemon owns a local SQLite store, immutable redacted configuration revisions and a transactional event outbox. A CLI communicates through a restricted Unix socket; native process supervision and safe telemetry are internal capabilities. Database/storage adapters and scheduling follow in later milestones.

**Tech Stack:** Go 1.26.10; standard library CLI/HTTP/JSON/testing/slog; `go.yaml.in/yaml/v3` v3.0.5; `modernc.org/sqlite` v1.61.0; `github.com/robfig/cron/v3` v3.0.1 for syntax validation only.

**Spec:** [Approved system overview](../../02-architecture/system-overview.md), [API/configuration](../../02-architecture/api-design.md), [metadata](../../02-architecture/database-design.md), [security](../../02-architecture/security-design.md), [infrastructure](../../02-architecture/infrastructure-design.md), [technology](../../02-architecture/tech-stack.md), [H1 evidence](../../04-decisions/ADR-001-single-server-backup-architecture.md), [M01 milestone](../milestones.md).

## Global Constraints

- "The daemon is the sole state owner."
- "V1 applies changes on controlled restart, not live reload."
- "CLI processes communicate with that daemon and do not write SQLite directly."
- "With zero channels, events/history still exist but no outbound delivery is scheduled."
- "A partial or nonzero-exit dump is never a valid backup."
- "No guaranteed RPO, automatic topology conversion or zero-impact performance promise is made."
- No public TCP control listener, Docker socket, shell execution hooks, provider requests or production connections in M01.
- Credentials are file references; logs, error responses, config snapshots and events never contain secret values or raw native stderr.
- No source-instance restore, overwrite, cutover, package installation or implementation is authorized by this planning request.

## Review Focus

The first review found five additional input cases worth pinning down. Tests below now assign each case to its owner:

- Explicit YAML `enabled: null` must not silently enable a job/channel; T02 rejects it while an omitted field retains its documented default.
- State, staging and log directories must not overlap; a rotation/cleanup path must never contain state or secret files. T02 checks lexical paths; T09 checks existing resolved identities before mutation.
- A secret exactly at the byte limit, including a trailing newline, must be returned intact rather than trimmed or rejected; T03 exercises this boundary.
- Replaying an already-accepted request after its job is disabled must return the original run, not create new work or reinterpret it under current config; T06 checks replay before current eligibility.
- Duplicate JSON mutation keys must be rejected even when both names are known; T08 adds duplicate-key detection because DisallowUnknownFields alone does not cover this case.

---

**Document status:** Plan v0.1 approved for subagent execution on 2026-10-25; M01 blocked at entry, T01 blocked and T02-T09 backlog; runtime checks not_run.

## Execution authorization and blocked entry - 2026-10-25

The human requested: "Use Superpowers subagent-driven-development to execute the approved Milestone 1 plan task by task. Follow the documented architecture, require tests and reviews, update progress, and stop on unresolved blockers."

This grants H2 for M01 plan v0.1 (plan introduced at `2aa208057f4ad6a44bf96260c55710126fd55a31`, unchanged at inspected HEAD `563fcf0d71c89fe7692860b55a8bcb150774ae98`) and selects subagent-driven execution. Earlier pending-H2 statements below are historical; do not ask for plan/mode approval again. It grants no merge/deployment approval and does not establish missing environment evidence.

Fresh checks: existing managed branch `feature/m01-secure-foundation` was clean; Windows Go remains 1.24.5; Ubuntu WSL is Linux x86_64, UID 1000, with go/cc/gcc absent from the checked PATH. `/mnt/c` reports v9fs and `/home` ext2/ext3. No source/module/tests exist. No package installation or application test was attempted.

M01 is **blocked before T01**. Required Linux Go 1.26.10/C compiler and filesystem qualification remain unresolved. Installing system tools in WSL or establishing a native Linux execution location changes state outside this managed worktree and needs scoped authorization. M00 is not complete; record an explicit M01 synthetic-fixture-only disposition before proceeding, retaining production inventory as blockers for later milestones. No need to reopen accepted architecture or request credentials.

A read-only subagent reviewed these entry gates; this is prerequisite review, not implementation/code review. The full task/interface conflict scan is deferred until the environment gate is resolved, before dispatching T01. Stop here as requested; T01 is blocked, T02-T09 remain backlog. Next action: authorize/provide the Linux toolchain and fixture scope, qualify the filesystem, then resume the already-authorized subagent workflow with TDD and per-task reviews.

## Purpose and authority

Translate M01.1-M01.3 into exact file boundaries, contracts, TDD tasks, acceptance checks and verification commands. The user requested a detailed plan, not implementation. H1 for architecture v0.1/P01-P09 is held. H2, H3 merge approval and H4 deployment approval are absent. New package pins, API details and bounded defaults here are implementation-plan choices for H2 review, not retroactive architecture changes.

Inspection baseline: clean `main` at `7a5ba9e3020ecc2bd9e3af8687cbdd7183195669`, then a documentation branch `feature/m01-implementation-plan`. Repository contains documentation, custom skills and `.gitignore`; no Go module/source/tests exist. Observed local toolchain: `go1.24.5 windows/amd64`. No dependencies were downloaded and no Docker daemon or target environment was contacted.

## Entry gates and dependencies

| Gate | Current state | Resolution required before execution |
|---|---|---|
| H1 | Held | Use accepted architecture v0.1; material deviations reopen only affected decisions |
| M00 | Incomplete | Record M00 exit or explicitly approve an M01-only fixture scope, leaving production engine/provider/resource inventory blocked for later milestones |
| H2 | Plan/mode granted 2026-10-25 | Subagent execution approved; unresolved environment and M00 gates still apply |
| Linux tooling | Installed and smoke-tested 2026-10-25 | Go 1.26.10/GCC 13.3.0 in WSL; race smoke check passes. Native /tmp permission/flock/socket probes pass; Windows-mounted checkout modes fail. Establish native test arrangement; see preflight evidence |
| Dependency resolution | Not run | Resolve exact pins and go.sum, verify checksums/licenses and bundled SQLite; do not silently upgrade/rewrite pins if unavailable |
| Workspace | Managed worktree prepared | feature/m01-secure-foundation at the recorded path; Linux execution semantics still unqualified; no merge/push |

All M01 tests use synthetic fixtures, temporary directories and helper processes. D01/D03/D04/D05 production DB versions, provider accounts and recovery targets are not needed for these fixtures. This does not mark M00 complete or waive the roadmap dependency. D02 Linux target/tooling, D06 contracts/pins and D07 execution mode are the immediate gates. Do not ask for credentials in chat.

### Dependency choices and evidence

| Dependency | Pin / responsibility | Verification before accepting it |
|---|---|---|
| Go | 1.26.10; `go 1.26.0`, `toolchain go1.26.10` | `go version` must report the selected toolchain; `GOTOOLCHAIN=local` prevents silent downloads |
| YAML | `go.yaml.in/yaml/v3 v3.0.5` | Strict KnownFields plus explicit duplicate/alias/depth checks; fuzz bounded decoding |
| SQLite driver | `modernc.org/sqlite v1.61.0` | SQL runtime version query and WAL/foreign-key/busy/migration tests; preserve driver's exact transitive libc requirement from its tagged go.mod |
| Cron grammar | `github.com/robfig/cron/v3 v3.0.1` | Five-field parser only, no descriptors/seconds; no scheduler started in M01 |
| Everything else | Standard library | No ORM, broker, web framework, CLI framework or provider SDK in M01 |

Library/API references for implementation: [root-contained file operations](https://pkg.go.dev/os#Root.OpenFile) and [SQLite consistent VACUUM INTO backups](https://www.sqlite.org/lang_vacuum.html).

The pins are based on primary documentation inspected on 2026-10-24: [Go releases](https://go.dev/doc/devel/release), [YAML package](https://pkg.go.dev/go.yaml.in/yaml/v3), [SQLite driver](https://pkg.go.dev/modernc.org/sqlite), [cron release](https://github.com/robfig/cron/releases/tag/v3.0.1). Publication is not compatibility/security test evidence. The selected SQLite documentation lists SQLite 3.54.1 and warns to match its libc dependency; its [tagged go.mod](https://gitlab.com/cznic/sqlite/-/raw/v1.61.0/go.mod) declares Go 1.26.0 and modernc.org/libc v1.79.0. If resolution reveals different requirements, stop and update the reviewed plan before running it.

## Scope and acceptance criteria

| ID | M01 / baseline | Observable acceptance | Owning tasks |
|---|---|---|---|
| M01-AC01 | M01.1; REQ-004 | Strict YAML rejects duplicate/unknown keys, aliases, excess size/depth, invalid references and invalid policy values without echoing submitted values | T02 |
| M01-AC02 | M01.1; security | Offline validation reads no secrets; startup checks only enabled-job dependencies and enabled channels; missing/unreadable/oversize secrets fail closed | T02, T03, T09 |
| M01-AC03 | M01.1; metadata | Equivalent normalized configs share a hash; revisions are immutable and contain references only; removal disables current jobs without deleting history | T02, T06 |
| M01-AC04 | M01.2; P01 | One daemon owns state; WAL, foreign keys, bounded busy time and ordered checked migrations; newer schema/partial migration/full disk cannot start healthy | T05, T06, T09 |
| M01-AC05 | M01.2; REQ-006 | Concurrent identical request IDs create one run/event set; changed payload conflicts; invalid/stale transitions and failed outbox writes are atomic rollbacks | T06 |
| M01-AC06 | M01.2; P02 | Only Unix IPC, restrictive socket/directory modes, bounded request/response sizes, safe status/errors and bounded deterministic history pagination | T08, T09 |
| M01-AC07 | M01.3; REQ-012 | Structured allowlisted logs rotate by size/count/age; diagnostics cannot leak credentials, URLs, SQL or raw process output; disk/log failures are visible | T04, T09 |
| M01-AC08 | M01.3; process boundary | Approved executable/arguments only; bounded stderr drainage, context cancellation, TERM/KILL process-group shutdown and child reaping | T07 |
| M01-AC09 | M01.3; health | Liveness is distinct from operational readiness; writable-state failure prevents mutations; readiness never claims backups are running or current | T08, T09 |
| M01-AC10 | REQ-009/010/011 foundation | Atomic outbox records with no network sender; zero channels create zero delivery rows; enabled routes snapshot channel revision | T06, T09 |
| M01-AC11 | Shutdown/restart | Startup reconciles only M01-owned interrupted work, stop closes socket/state/logs and reaps helpers; crash releases singleton lock | T05-T09 |
| M01-AC12 | M01 deliverables | Linux unit/race/integration tests, build/vet, module verification, example config and development instructions pass on the recorded candidate | T01-T09 |

Not delivered: dumps/encryption (M02), remote commits/upload (M03), restore (M04), running cron/retention/overdue calculations (M05), Drive (M06), notification delivery (M07), production image/Compose/CI (M08) or release (M09). These commands must return an explicit unsupported/not_implemented outcome, never a fake successful backup. M01 covers run/operation history primitives; artifact/copy history schema is added with its owning later milestone.

## Task register and order

| Task | Deliverable | Dependencies | Status | Commit / runtime evidence |
|---|---|---|---|---|
| T01 | Module, shared safe errors and CLI skeleton | Entry gates | blocked | No implementation; tools installed, native test arrangement/fixture disposition unresolved, 2026-10-25 |
| T02 | Strict typed configuration and offline validation | T01 | backlog | none / not_run |
| T03 | Bounded secret-file resolver | T02 | backlog | none / not_run |
| T04 | Safe rotating structured logs | T01 | backlog | none / not_run |
| T05 | Singleton SQLite ownership and migrations | T01 | backlog | none / not_run |
| T06 | Config revisions, idempotent run state and outbox | T02, T05 | backlog | none / not_run |
| T07 | Linux native-process supervisor | T01 | backlog | none / not_run |
| T08 | Local HTTP contracts and CLI client | T02, T06 | backlog | none / not_run |
| T09 | Daemon composition, health/shutdown and acceptance harness | T03, T04, T05, T06, T07, T08 | backlog | none / not_run |

Execute serially T01-T09 unless H2 selects parallel mode and assigns disjoint file ownership. T01 owns shared errors, T02 owns config, T05/T06 own schema/store; consumers cannot silently change them. The record above is authoritative for task status. After each real commit record its hash/evidence in the task record using the Git bookkeeping policy. Integration/release state: not_started. Implementation owner/reviewer: unassigned until H2; no invented staffing.

## File and interface map

All paths are relative to repository root and do not exist unless explicitly marked Modify. Listed test files are real deliverables to create during authorized execution, not files created by this planning task.

| Owner | Exact files | Responsibility |
|---|---|---|
| T01 | `go.mod`, `go.sum`, `cmd/bkplight/main.go`, `internal/cli/run.go`, `internal/cli/run_test.go`, `internal/safeerr/error.go`, `internal/safeerr/error_test.go` | Module, CLI dispatch/error envelope |
| T02 | `internal/config/types.go`, `internal/config/decode.go`, `internal/config/validate.go`, `internal/config/revision.go`, `internal/config/config_test.go`, `internal/config/fuzz_test.go`, `internal/config/testdata/valid.yaml`, `examples/bkplight.yaml`; Modify `internal/cli/run.go`, `internal/cli/run_test.go`, `go.mod`, `go.sum` | Closed config schema, normalized revisions, config validate |
| T03 | `internal/secrets/files_linux.go`, `internal/secrets/files_linux_test.go`; Modify `internal/cli/run.go`, `internal/cli/run_test.go` | Root-contained bounded secret reads and explicit check-secrets option |
| T04 | `internal/observability/logger.go`, `internal/observability/rotate.go`, `internal/observability/logger_test.go`, `internal/observability/rotate_test.go` | Typed safe log records and rotation |
| T05 | `internal/state/open_linux.go`, `internal/state/lock_linux.go`, `internal/state/migrate.go`, `internal/state/migrations/001_foundation.sql`, `internal/state/open_linux_test.go`, `internal/state/migrate_test.go`; Modify `go.mod`, `go.sum` | Singleton descriptor, database setup and checked schema |
| T06 | `internal/state/types.go`, `internal/state/config.go`, `internal/state/runs.go`, `internal/state/events.go`, `internal/state/health.go`, `internal/state/runs_test.go`, `internal/state/config_test.go`, `internal/state/events_test.go` | Transactional config/run/event operations |
| T07 | `internal/process/runner_linux.go`, `internal/process/runner_linux_test.go` | Bounded internal exec supervisor |
| T08 | `internal/control/types.go`, `internal/control/server_linux.go`, `internal/control/client_linux.go`, `internal/control/server_linux_test.go`, `internal/control/client_linux_test.go`; Modify `internal/cli/run.go`, `internal/cli/run_test.go` | Unix HTTP API and client commands |
| T09 | `internal/app/serve_linux.go`, `internal/app/health_linux.go`, `internal/app/serve_linux_test.go`, `tests/integration/foundation_linux_test.go`, `docs/06-testing/m01-foundation-evidence.md`, `docs/03-planning/development-environment.md`; Modify `cmd/bkplight/main.go`, `internal/cli/run.go`, `internal/cli/run_test.go`, `README.md`, `docs/README.md`, `docs/03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md`, `docs/05-progress/current-status.md`, `docs/08-agent-memory/session-handoff.md` | Composition, Linux fixture harness, evidence and instructions |

Do not create public `pkg/` abstractions or empty later-milestone packages. `_linux.go` files intentionally scope OS behavior; this milestone does not promise native Windows support. Portable packages may be tested on Windows but that cannot satisfy milestone acceptance.

### Shared contracts (owners enforce these signatures)

```go
// internal/safeerr: fields carry codes and field identifiers, never raw errors.
type Code string
const (
    Invalid Code = "invalid_config"
    Denied Code = "denied"
    Conflict Code = "conflict"
    Busy Code = "busy"
    Unavailable Code = "unavailable"
    NotFound Code = "not_found"
    NotImplemented Code = "not_implemented"
    Internal Code = "internal"
)
type Error struct { Code Code; Field string; Retryable bool }
func (e *Error) Error() string
func Public(err error) Error // unknown error -> Internal, no err.Error() reflection

// internal/config: structs are the closed schema in the next section.
func Decode(r io.Reader) (Config, error) // bounded, syntax + semantics, no file/network IO
func Revision(c Config) (id string, snapshot []byte, err error) // normalized JSON, SHA-256
func SecretRefs(c Config) []string // unique sorted active dependency paths

// internal/secrets
type Resolver struct { /* private root handles; no public secret map */ }
func New(roots []string, maxBytes int64) (*Resolver, error)
func (r *Resolver) Read(ctx context.Context, absolutePath string) ([]byte, error)
func (r *Resolver) Close() error

// internal/observability: enum-like MessageCode, not arbitrary prose/attributes.
type Record struct { Time time.Time; Level string; MessageCode string; RunID string; ErrorCode string }
type Options struct { Directory string; MaxBytes int64; MaxFiles int; MaxAge time.Duration; Now func() time.Time }
type Logger struct { /* private writer, mutex and sticky write-error flag */ }
func Open(o Options) (*Logger, error)
func (l *Logger) Write(r Record) error
func (l *Logger) Healthy() bool
func (l *Logger) Close() error

// internal/state
type Store struct { /* owns DB and lock descriptor */ }
type Options struct { Directory string; Now func() time.Time }
type JobRevision struct { ID, JobID, SourceID, ConfigID string; Snapshot []byte }
type Channel struct { Revision string; Events []string }
type Run struct { ID, JobRevisionID, RequestID, PayloadHash, State string; Version int64; StartedAt time.Time; ErrorCode string }
type Begin struct { ID, JobRevisionID, RequestID, PayloadHash string }
type Transition struct { RunID, From, To string; Version int64; ErrorCode string }
type Page struct { Items []Run; Next string }
func Open(ctx context.Context, o Options) (*Store, error)
func (s *Store) Activate(ctx context.Context, c config.Config) (string, error)
func (s *Store) BeginRun(ctx context.Context, b Begin) (run Run, replay bool, err error)
func (s *Store) Move(ctx context.Context, t Transition) (Run, error)
func (s *Store) Cancel(ctx context.Context, runID, requestID string) (Run, error)
func (s *Store) GetRun(ctx context.Context, id string) (Run, error)
func (s *Store) ListRuns(ctx context.Context, limit int, after string) (Page, error)
func (s *Store) Jobs(ctx context.Context) ([]JobRevision, error)
func (s *Store) ProbeWrite(ctx context.Context) error
func (s *Store) Recover(ctx context.Context) error
func (s *Store) Close() error

// internal/process: caller supplies an allowlisted executable, not a shell.
type Spec struct { Executable string; Args []string; Env []string; Dir string; Stdout io.Writer; StderrLimit int; Grace time.Duration }
type Result struct { ExitCode int; StderrBytes int64; StderrTruncated bool; Cancelled bool }
type Runner struct { AllowedExecutables []string }
func (r Runner) Run(ctx context.Context, spec Spec) (Result, error)

// internal/control
type Health struct { Live, Ready bool; Scope, State, Logs, Scheduler, BackupEngine string }
type Backend interface {
    Jobs(context.Context) ([]state.JobRevision, error)
    GetRun(context.Context, string) (state.Run, error)
    ListRuns(context.Context, int, string) (state.Page, error)
    Cancel(context.Context, string, string) (state.Run, error) // runID, requestID
    Health(context.Context) Health
}
type Server struct { /* private http.Server and owned socket */ }
func Listen(path string, b Backend) (*Server, error)
func (s *Server) Serve() error
func (s *Server) Close(ctx context.Context) error
type Client struct { /* timeout, Unix-only transport; no environment proxy */ }
func NewClient(socket string) *Client
func (c *Client) Do(ctx context.Context, method, path string, body []byte) ([]byte, error)

// internal/cli and internal/app
func Run(ctx context.Context, args []string, out, errOut io.Writer) int // cli package
func Serve(ctx context.Context, configPath string) error // app package
```

Implement every referenced type in its owning file. Private fields shown as comments are encapsulation boundaries, not missing public contracts. No error object wraps arbitrary text into a public JSON payload. Use Go standard-library types/imports named in signatures; import shared packages through module `bkplight.local/bkplight`. This private local module path is a planning choice, not a claimed Git hosting URL.

### Configuration schema and fixed M01 limits

All fields use explicit yaml/json snake_case tags. Map keys are IDs matching `^[a-z][a-z0-9_-]{0,63}$`; unknown fields fail at every nested level. `Config` contains `Version int`, `Service Service`, `Sources map[string]Source`, `Destinations map[string]Destination`, `Jobs map[string]Job`, `RestoreTargets map[string]Target`, `VerificationTargets map[string]Target`, `Encryption Encryption`, `Notifications Notifications`, `Logging Logging`. Struct definitions are generated directly from this field table, never `map[string]any` blobs.

| Struct | Field names and Go types | Constraints/defaults |
|---|---|---|
| Service | `Timezone string`, `StateDir string`, `SocketPath string`, `SecretRoots []string`, `StagingDir string`, `StagingMaxBytes int64`, `MinFreeBytes int64`, `MaxHeavyOperations int` | IANA zone; absolute cleaned private paths; staging budget/reserve positive when jobs enabled; heavy default 1 |
| Source | `Type string`, `Host string`, `Port int`, `CredentialsFile string`, `Databases []string`, `Consistency string`, `Globals bool`, `TLS TLS` | postgres/mongodb; host (no URL/userinfo); port 1..65535; nonempty unique database names; Mongo requires live_not_guaranteed; globals only PG |
| TLS | `Enabled bool`, `CAFile string`, `ServerName string` | No insecure-skip field; enabled requires CA path and server name; actual TLS connection deferred |
| Destination | `Type string`, `Endpoint string`, `Bucket string`, `Prefix string`, `Region string`, `FolderID string`, `CredentialsFile string` | s3/r2/drive; HTTPS endpoint without userinfo/query/fragment; bucket/prefix for s3/r2, folder_id for Drive; duplicate physical bindings rejected |
| Job | `Enabled *bool`, `Source string`, `Schedule Schedule`, `Destinations []JobDestination`, `Retention Retention`, `Verification Verification`, `Notify []string` | enabled absent means true; source/dest/target/channel refs resolve; at least one required destination if enabled |
| Schedule | `Cron string`, `Daily string`, `Weekly string`, `Timezone string` | Exactly one selector; daily HH:MM; weekly SUN..SAT@HH:MM; normalize to five-field cron; job override or service zone |
| JobDestination | `ID string`, `Required bool` | Unique existing destination IDs |
| Retention | `KeepLast int`, `KeepDays int`, `MinSuccessful int`, `ProtectLastVerified *bool` | Nonnegative count/days, at least one positive; min >=1 (default 1); protect default true |
| Verification | `Schedule *Schedule`, `Destination string`, `Target string`, `Profile string` | All absent disables; otherwise complete, selected destination and matching target engine, profile portable_data/faithful |
| Target | `Type string`, `Host string`, `Port int`, `Identity string`, `CredentialsFile string`, `TLS TLS` | Same syntax as source; immutable configured ID unique; configured source collisions rejected; runtime identity validation belongs to M04 |
| Encryption | `Recipients []string`, `SigningKeyFile string`, `IdentityFile string` | Enabled job requires nonempty public recipients and signing-key reference; automatic verification additionally requires identity reference; format validation/crypto deferred to M02 |
| Notifications | `Channels map[string]ChannelConfig` | Omitted/empty means no delivery; no implicit default channel |
| ChannelConfig | `Enabled *bool`, `Type string`, `URLFile string`, `AuthFile string`, `HMACFile string`, `Events []string` | custom_http/discord; enabled default true; custom optional auth/HMAC, Discord no mentions; event names from architecture; no plaintext URL field |
| Logging | `Directory string`, `MaxBytes int64`, `MaxFiles int`, `MaxDays int` | Default 20 MiB, 10 files total including active, 14 days; positive; absolute private directory |

M01 fixed implementation bounds (reviewed with H2): YAML 1 MiB, depth 32, 20,000 nodes; disallow aliases/anchors/merge keys and multiple documents. Secret read 64 KiB; IPC body 64 KiB; JSON response 1 MiB; HTTP header timeout 2 seconds, request deadline 5 seconds, idle timeout 30 seconds. History limit default 50, max 200, opaque base64url `(started_at,id)` cursor. Run ID generated from 16 crypto-random bytes encoded hex; request IDs match `[A-Za-z0-9_-]{1,128}`. Safe errors return enum codes, not rejected values. Counts are safety defaults, not production capacity claims.

Normalize map ordering through encoding/json, default application and daily/weekly expansion. SHA-256 hash the normalized redacted JSON; never expand secret contents. Same normalized config is idempotent; secret rotation at the same reference does not alter config revision. Enabled routes/credentials are checked at startup. An empty jobs map is valid maintenance mode; health must state backup engine/scheduler not_implemented rather than advertise protection.

## T01 - Shared safe errors and command skeleton

**Dependencies:** all entry gates. **Files:** the T01 file-map row. **Acceptance:** M01-AC06/12 foundation. **Consumes:** standard library only. **Produces:** safeerr and cli signatures above.

- [ ] Create go.mod with module `bkplight.local/bkplight`, `go 1.26.0`, `toolchain go1.26.10`; add no external requirements until the task that imports them. Create the test below before its implementation. T01 creates no go.sum until dependencies require it.

```go
// internal/safeerr/error_test.go, package safeerr
func TestPublicHidesUnknownError(t *testing.T) {
    got := Public(errors.New("synthetic-sensitive-value"))
    if got.Code != Internal || strings.Contains(got.Error(), "synthetic-sensitive-value") {
        t.Fatalf("unsafe error classification: %q", got.Code)
    }
}
```

- [ ] Run `GOTOOLCHAIN=local go test ./internal/safeerr -run TestPublicHidesUnknownError -count=1 -v`; expect the missing API failure, not an absent compiler/dependency failure.
- [ ] Implement `Error.Error` as code plus validated field identifier, and `Public` using `errors.As` only for safeerr.Error; all others become Internal. Accept only enumerated codes and field-path characters. CLI uses flag.FlagSet with ContinueOnError and SetOutput(io.Discard), explicit subcommand switches, no raw parser errors; `version` prints `bkplight dev`, unknown command exits 2. `main` delegates exit status without printing extra errors.

```go
func Public(err error) Error {
    var e *Error
    if errors.As(err, &e) && e != nil && valid(e.Code, e.Field) {
        return *e
    }
    return Error{Code: Internal}
}
```

`valid(Code,string) bool` is a private T01 helper in error.go: switch over the nine constants, require Field empty or ASCII letters/digits/underscore/dot/brackets up to 128 characters. `Error.Error()` uses this helper and falls back to `internal`. Do not include the original error's message.

- [ ] Add `TestErrorRejectsUnknownCodeAndField`, `TestVersionCommand` and `TestUnknownCommandDoesNotEchoArgs` (pass a synthetic credential-like argument; assert only invalid-command code, exit 2).
- [ ] Run `go test ./internal/safeerr ./internal/cli -count=1`, `go vet ./internal/safeerr ./internal/cli`, `go build ./cmd/bkplight`; expect exit 0 and no module/source mutation beyond listed files. Build output is ignored by .gitignore.
- [ ] Review/stage only T01 files; commit `chore(core): establish safe command and error contracts`. Record actual hash and red/green evidence before T02.

## T02 - Strict configuration and offline validation

**Dependencies:** T01. **Files:** T02 row. **Acceptance:** M01-AC01/02/03. **Consumes:** safeerr. **Produces:** config structs/Decode/Revision/SecretRefs, `bkplight config validate --config PATH`.

- [ ] Add pinned YAML and cron requirements in go.mod and resolve only after H2; commit resulting go.sum. Create valid.yaml and examples/bkplight.yaml with the same synthetic maintenance fixture:

```yaml
version: 1
service:
  timezone: Asia/Colombo
  state_dir: /var/lib/bkplight/state
  socket_path: /run/bkplight/control.sock
  secret_roots: [/run/secrets]
  staging_dir: /var/lib/bkplight/staging
sources: {}
destinations: {}
jobs: {}
restore_targets: {}
verification_targets: {}
encryption: {}
notifications: {channels: {}}
logging: {directory: /var/log/bkplight}
```

- [ ] Write the test before decoding code (imports bytes/os/testing; package config).

```go
func TestDecodeRejectsDuplicateAndUnknown(t *testing.T) {
    raw, err := os.ReadFile("testdata/valid.yaml")
    if err != nil { t.Fatal(err) }
    for _, extra := range []string{"\nversion: 1\n", "\npassword: synthetic\n"} {
        _, err = Decode(bytes.NewReader(append(append([]byte{}, raw...), extra...)))
        if err == nil { t.Fatal("accepted invalid document") }
    }
}
```

- [ ] Run `go test ./internal/config -run TestDecodeRejectsDuplicateAndUnknown -count=1 -v`; require RED because invalid YAML is accepted or API absent, not module-fetch failure.
- [ ] Implement two-pass decoding as the exact algorithm below. `inspectNode` is a private recursive helper in decode.go taking `(*yaml.Node,depth,*count)` and returning safeerr.Invalid on duplicate mapping keys, aliases/anchors/merge keys, excessive nodes/depth or non-scalar keys.

```text
read at most 1 MiB + 1; reject excess before parsing
decode one yaml.Node; require the next decode to return io.EOF
inspectNode(root, 0, &count) without echoing keys/values
decode same bytes into Config using Decoder.KnownFields(true)
apply the field-table defaults; validate IDs, paths, enums and references
normalize Schedule via cron.NewParser(Minute|Hour|Dom|Month|Dow)
return the typed Config; never read secret files, resolve DNS or open SQLite
```

- [ ] Implement full table validation, including enabled/disabled graph traversal, no inline credentials, no URL userinfo, no source/target alias collision by configured host/port/engine, and no duplicate provider locations. Preserve nested unknown-key rejection. Runtime DNS equivalence and live source identity are M04 preflights, not proven by this syntax check.
- [ ] Add table-driven tests: `TestConfigBounds` (1 MiB+1, depth33, node20001, alias/merge/multidoc); `TestConfigReferences` (dangling source/destination/channel/target, duplicate selected destination); `TestConfigPolicies` (zero required dest, negative retention, neither count nor days, invalid timezone/cron, Mongo consistency omitted); `TestRevisionCanonical` (map order/daily-vs-cron equivalence, one meaningful change changes hash); `TestSecretRefsActiveOnly` (disabled jobs/channels excluded, shared dependency included once); `TestOfflineValidationDoesNotReadSecrets` (nonexistent absolute path with valid syntax accepted offline).
- [ ] Add the Review Focus tests below. Extend inspectNode to reject explicit null for enabled flags; validate independent state/staging/log directories and reject overlaps with secret roots before any filesystem IO. Reject duplicate selected databases, but preserve legitimate names/case/Unicode without inventing identifier-only DB name restrictions.

```go
func TestConfigRejectsNullEnabled(t *testing.T) {
    raw,err:=os.ReadFile("testdata/valid.yaml"); if err!=nil {t.Fatal(err)}
    raw=bytes.Replace(raw,[]byte("notifications: {channels: {}}"),[]byte("notifications: {channels: {notice: {enabled: null, type: custom_http, url_file: /run/secrets/hook}}}"),1)
    if _,err=Decode(bytes.NewReader(raw));err==nil {t.Fatal("null silently enabled channel")}
}
func TestConfigRejectsOverlappingDirectories(t *testing.T) {
    raw,err:=os.ReadFile("testdata/valid.yaml"); if err!=nil {t.Fatal(err)}
    raw=bytes.Replace(raw,[]byte("/var/log/bkplight"),[]byte("/var/lib/bkplight/state/logs"),1)
    if _,err=Decode(bytes.NewReader(raw));err==nil {t.Fatal("overlapping directories accepted")}
}
```
- [ ] Add `FuzzDecode` with seeds valid.yaml, empty, duplicate version and alias; assert no panic and bounded input admission. CLI validation outputs only normalized config hash and `valid`, or safe code/field; `--check-secrets` is added in T03.
- [ ] Run `go test ./internal/config ./internal/cli -count=1`, `go test ./internal/config -run '^$' -fuzz FuzzDecode -fuzztime 10s`, `go run ./cmd/bkplight config validate --config examples/bkplight.yaml`; expect PASS and no secret/state/log file creation.
- [ ] Review exact staged config examples for synthetic content; commit `feat(config): validate versioned backup configuration` with task ID and evidence.

## T03 - Root-contained bounded secret reads

**Dependencies:** T02. **Files:** T03 row plus Modify `internal/cli/run.go`, `internal/cli/run_test.go` for `--check-secrets`. **Acceptance:** M01-AC02. **Consumes:** config.SecretRefs/safeerr. **Produces:** secrets.Resolver.

- [ ] Create test-owned secret roots with `t.TempDir`, regular files chmod 0600 and fake content; never use `/run/secrets` or real credentials in tests. Write:

```go
// package secrets; imports context, os, path/filepath, testing
func TestReadRejectsOversizeAndEscape(t *testing.T) {
    dir := t.TempDir()
    if err := os.WriteFile(filepath.Join(dir,"large"), make([]byte,65),0600); err != nil { t.Fatal(err) }
    r, err := New([]string{dir},64); if err != nil { t.Fatal(err) }; defer r.Close()
    if _, err = r.Read(context.Background(),filepath.Join(dir,"large")); err == nil { t.Fatal("oversize accepted") }
    if _, err = r.Read(context.Background(),filepath.Join(dir,"..","outside")); err == nil { t.Fatal("escape accepted") }
}
```

- [ ] Run `go test ./internal/secrets -run TestReadRejectsOversizeAndEscape -count=1 -v`; observe RED.
- [ ] Implement `New` with `os.OpenRoot` for each absolute configured trusted root, closing earlier roots on failure. Read resolves the lexically containing root, derives relative path and uses Root.OpenFile with `os.O_RDONLY | syscall.O_NONBLOCK`, then checks the opened descriptor's Stat before reading; reject nonregular files, file size above limit and reads exceeding limit+1. Follow only symlinks confined by os.Root; this supports in-root secret projections but rejects escape. Check context before/after IO; no filesystem read occurs in config.Decode. Return only safe code/field on open/read errors. Do not trim bytes or log contents; caller owns format handling and minimizes lifetime.
- [ ] Add `TestReadMissingUnreadableNonRegular`, `TestReadSymlinkEscape`, `TestCancelledRead`, `TestSecretsNotIncludedInRevision`, `TestCheckSecretsOnlyEnabledGraph`. Run unreadable mode test as non-root; a root-only pass is insufficient. FIFO/directory tests must return immediately without reading because the open is nonblocking and descriptor type is checked; test symlink swaps remain confined by os.Root.
- [ ] Add this exact-limit test (bytes import). Do not trim or newline-normalize secret data in the resolver.

```go
func TestReadExactLimitPreservesBytes(t *testing.T) {
    dir:=t.TempDir();want:=append(bytes.Repeat([]byte("x"),63),'\n')
    path:=filepath.Join(dir,"secret");if err:=os.WriteFile(path,want,0600);err!=nil {t.Fatal(err)}
    r,err:=New([]string{dir},64);if err!=nil {t.Fatal(err)};defer r.Close()
    got,err:=r.Read(context.Background(),path)
    if err!=nil || !bytes.Equal(got,want) {t.Fatal("exact-limit secret changed")}
}
```
- [ ] Run `go test ./internal/secrets ./internal/config ./internal/cli -count=1`, `go test -race ./internal/secrets -count=1`; expect PASS with no secret in test logs/CLI errors.
- [ ] Commit `feat(secrets): constrain credential file resolution`; record hash/evidence. Native-tool credential format and runtime token rotation remain later adapter responsibilities.

## T04 - Safe structured logs and rotation

**Dependencies:** T01. **Files:** T04 row. **Acceptance:** M01-AC07. **Consumes:** safeerr. **Produces:** observability.Options/Record/Logger; no general arbitrary-message logging API.

- [ ] Write the test before rotation code (package observability; imports bytes/os/path/filepath/testing/time).

```go
func TestRotationKeepsBoundedFiles(t *testing.T) {
    dir := t.TempDir()
    now := func() time.Time { return time.Date(2026,10,24,0,0,0,0,time.UTC) }
    l, err := Open(Options{Directory:dir,MaxBytes:256,MaxFiles:3,MaxAge:24*time.Hour,Now:now})
    if err != nil { t.Fatal(err) }
    for i:=0; i<20; i++ {
        if err=l.Write(Record{Time:now(),Level:"info",MessageCode:"service.started"}); err!=nil { t.Fatal(err) }
    }
    if err=l.Close(); err!=nil { t.Fatal(err) }
    files, err := filepath.Glob(filepath.Join(dir,"bkplight*.log")); if err!=nil { t.Fatal(err) }
    if len(files)>3 { t.Fatalf("too many files: %d",len(files)) }
    for _, name := range files {
        b, err:=os.ReadFile(name); if err!=nil { t.Fatal(err) }
        if len(b)>256 || !bytes.HasSuffix(b,[]byte("\n")) { t.Fatal("invalid rotation boundary") }
    }
}
```

- [ ] Run `go test ./internal/observability -run TestRotationKeepsBoundedFiles -count=1 -v`; observe RED.
- [ ] Implement mutex-serialized JSON-line writes, enum validation and rotation with the exact sequence below. MessageCode allowlist: `service.started`, `service.stopped`, `service.degraded`, `config.activated`, `state.failed`, `process.started`, `process.finished`, `process.cancelled`. Level: debug/info/warn/error; RunID is empty or 32 lowercase hex chars; ErrorCode empty or safeerr code. Invalid fields are rejected without reflecting their values.

```text
encode the typed record into a bounded byte buffer, with UTC RFC3339Nano time
reject a record over min(16 KiB, MaxBytes); never split a JSON record
under mutex: if active size + record > MaxBytes, close active and rename
choose next monotonic archive suffix from owned existing names
open active bkplight.log at 0600, with symlink/ownership checks
append the complete line; on error set sticky unhealthy and return safe error
remove only owned archives by age, then oldest by count (active counts as one)
never remove active file, unrelated files, symlinks or unknown directories
Close flushes/closes once; Healthy reports write/rotation errors
```

- [ ] Add `TestLoggerRejectsUnsafeFields` (pass synthetic credential in MessageCode/RunID; no output includes it), `TestRotationAgeAndRestart`, `TestRotationPreservesForeignFiles`, `TestLoggerConcurrentWrites`, `TestLoggerWriteFailureDegrades` (inject writer failure in private test seam; no production injection switch), `TestOversizeRecordRejected`. Age tests use injected Now, not sleeps. Default remains 20 MiB/10 total files/14 days; tests use small explicit values.
- [ ] Run `go test ./internal/observability -count=1`, `go test -race ./internal/observability -count=1`; expect valid JSON, bounded file count/size and no reported secrets. Stderr fallback uses a fixed code only, not the failing path/error text.
- [ ] Commit `feat(logging): add bounded safe structured logs`; record evidence. No webhook sender is introduced.

## T05 - Singleton SQLite and migration baseline

**Dependencies:** T01; exact driver pin resolution. **Files:** T05 row. **Acceptance:** M01-AC04/11. **Consumes:** safeerr. **Produces:** state.Store/Open/Close and schema version 1. T06 defines business methods, not a second database owner.

- [ ] Add the driver import/requirement and resulting go.sum; keep SQLite and its transitive libc consistent. Write Linux lock test first (package state).

```go
func TestSecondOwnerRejected(t *testing.T) {
    o:=Options{Directory:t.TempDir(),Now:time.Now}
    a,err:=Open(context.Background(),o); if err!=nil {t.Fatal(err)}
    defer a.Close()
    b,err:=Open(context.Background(),o)
    if err==nil {b.Close();t.Fatal("second owner accepted")}
}
```

- [ ] Run `go test ./internal/state -run TestSecondOwnerRejected -count=1 -v`; observe RED. Once a no-op skeleton compiles, also demonstrate a failing singleton behavior assertion, not only a missing symbol.
- [ ] Acquire `state.lock` with nonblocking Linux `flock(LOCK_EX|LOCK_NB)` before SQLite open; keep its descriptor until after DB.Close. Never unlink the lock inode during ownership/release. State directory is local, private 0700, file 0600; reject symlink/state-path reuse that escapes configured ownership. Do not copy SQLite's own file descriptor or perform unrelated opens/closes on the DB file while it is live.
- [ ] Open SQLite using a URL-escaped path and driver pragmas on each connection, cap pool to one connection/one idle. Set/verify `foreign_keys=ON`, `journal_mode=WAL`, `synchronous=FULL`, `busy_timeout=1000`; verify runtime SQLite version. Filesystem/SQL errors map to Unavailable. Use operation contexts so contention is bounded. M01 has no remote/network FS support.
- [ ] Embed `migrations/*.sql`. Before a nonempty existing supported database receives a migration, create a consistent SQLite backup using `VACUUM INTO` on the single owner connection to a unique, private state backup path; fail if backup fails and never copy a live .db without WAL. Version 0 is allowed only for an empty database. Refuse any user tables without the migration ledger or schema version newer than supported. Transactionally apply migration SQL, ledger checksum and `PRAGMA user_version=1`; checksum mismatch fails closed. Test future upgrade path using a test-only second migration, never by editing a shipped migration.

**Exact schema for `001_foundation.sql`:** use UTC microseconds in integer timestamps; JSON is normalized/redacted, not secret storage. Application computes SHA-256 checksums over embedded SQL bytes.

```sql
CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at INTEGER NOT NULL
);
CREATE TABLE config_revisions (
  id TEXT PRIMARY KEY, snapshot BLOB NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE active_config (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  config_id TEXT NOT NULL REFERENCES config_revisions(id)
);
CREATE TABLE job_revisions (
  id TEXT PRIMARY KEY, job_id TEXT NOT NULL, source_id TEXT NOT NULL,
  config_id TEXT NOT NULL REFERENCES config_revisions(id), snapshot BLOB NOT NULL,
  UNIQUE(job_id,config_id)
);
CREATE TABLE runs (
  id TEXT PRIMARY KEY, job_revision_id TEXT NOT NULL REFERENCES job_revisions(id),
  request_id TEXT NOT NULL UNIQUE, payload_hash TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('queued','preflight','dumping','delivering','succeeded','degraded','failed','cancelled','interrupted')),
  version INTEGER NOT NULL CHECK(version>=1), started_at INTEGER NOT NULL,
  finished_at INTEGER, error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX runs_history ON runs(started_at DESC,id DESC);
CREATE TABLE run_commands (
  request_id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id),
  kind TEXT NOT NULL CHECK(kind='cancel'), payload_hash TEXT NOT NULL,
  result_version INTEGER NOT NULL
);
CREATE TABLE events (
  id INTEGER PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id),
  sequence INTEGER NOT NULL, event_type TEXT NOT NULL, payload BLOB NOT NULL,
  occurred_at INTEGER NOT NULL, UNIQUE(run_id,sequence)
);
CREATE TABLE channel_revisions (
  id TEXT PRIMARY KEY, config_id TEXT NOT NULL REFERENCES config_revisions(id),
  channel_id TEXT NOT NULL, snapshot BLOB NOT NULL, UNIQUE(config_id,channel_id)
);
CREATE TABLE deliveries (
  event_id INTEGER NOT NULL REFERENCES events(id),
  channel_revision_id TEXT NOT NULL REFERENCES channel_revisions(id),
  state TEXT NOT NULL CHECK(state IN ('pending','cancelled')),
  PRIMARY KEY(event_id,channel_revision_id)
);
CREATE TABLE health_probe (singleton INTEGER PRIMARY KEY CHECK(singleton=1), tick INTEGER NOT NULL);
INSERT INTO health_probe VALUES(1,0);
```

This is an additive M01 subset: delivery states expand in M07; source/destination revision tables, schedules/artifacts/copies/restores/claims arrive with their owning feature. Config snapshots and job revisions retain source/destination bindings now. Do not expose a successful backup based only on these foundation rows.

- [ ] Add `TestPragmasAndRuntimeVersion`, `TestNewerSchemaRejected`, `TestUnknownExistingDatabaseRejected`, `TestMigrationChecksumMismatch`, `TestMigrationRollback`, `TestMigrationBackupFailure`, `TestBusyDeadline`, `TestLockReleasedAfterCrash`. Crash test starts a helper test process, waits for an explicit ready pipe, SIGKILLs/reaps it and proves a new owner acquires the same directory. Test backup restores the captured file to a separate temp directory and verifies old schema/data; never overwrites the active test DB.
- [ ] Run `go test ./internal/state -count=1`, `go test -race ./internal/state -count=1`, `go mod verify`; expect PASS, runtime SQLite 3.54.1 (or stop if the selected pin does not match its documented version), bounded busy failure, no partial schema and preserved backup. This qualification may invalidate the pin; do not report success without running it.
- [ ] Commit `feat(state): own SQLite lifecycle and checked migrations`; record hash, runtime version and evidence.

## T06 - Revisions, idempotent runs and transactional outbox

**Dependencies:** T02/T05. **Files:** T06 row. **Acceptance:** M01-AC03/05/10/11. **Consumes:** normalized Config, Store, safeerr. **Produces:** Activate/BeginRun/Move/GetRun/ListRuns/Jobs/ProbeWrite/Recover signatures.

- [ ] Write this package-state test fixture in runs_test.go (imports bytes/context/os/path/filepath/strings/testing/time, config and yaml). It reads no real secrets. The recipient is a synthetic fixture because cryptographic validation is outside M01.

```go
func newFixture(t *testing.T) (*Store,string) {
    t.Helper()
    raw,err:=os.ReadFile("../config/testdata/valid.yaml");if err!=nil {t.Fatal(err)}
    c,err:=config.Decode(bytes.NewReader(raw));if err!=nil {t.Fatal(err)}
    c.Sources["pg"]=config.Source{Type:"postgres",Host:"fixture-pg",Port:5432,CredentialsFile:"/run/secrets/pg",Databases:[]string{"app"}}
    c.Destinations["r2"]=config.Destination{Type:"r2",Endpoint:"https://example.invalid",Bucket:"fixture",Prefix:"test/",CredentialsFile:"/run/secrets/r2"}
    c.Jobs["hourly"]=config.Job{Source:"pg",Schedule:config.Schedule{Cron:"0 * * * *"},Destinations:[]config.JobDestination{{ID:"r2",Required:true}},Retention:config.Retention{KeepLast:5,MinSuccessful:1}}
    c.Service.StagingMaxBytes=1048576;c.Service.MinFreeBytes=65536
    c.Encryption=config.Encryption{Recipients:[]string{"synthetic-public-recipient"},SigningKeyFile:"/run/secrets/signing"}
    raw,err=yaml.Marshal(c);if err!=nil {t.Fatal(err)}
    c,err=config.Decode(bytes.NewReader(raw));if err!=nil {t.Fatal(err)}
    s,err:=Open(context.Background(),Options{Directory:t.TempDir(),Now:time.Now});if err!=nil {t.Fatal(err)}
    t.Cleanup(func(){_ = s.Close()})
    if _,err=s.Activate(context.Background(),c);err!=nil {t.Fatal(err)}
    jobs,err:=s.Jobs(context.Background());if err!=nil || len(jobs)!=1 {t.Fatal("missing fixture job")}
    return s,jobs[0].ID
}
```

Close is idempotent. The test file imports only packages its final contents use. Then add the failed-event trigger test:

```go
func TestRunAndOutboxRollbackTogether(t *testing.T) {
    s,jobID:=newFixture(t); defer s.Close()
    _,err:=s.db.Exec(`CREATE TRIGGER fail_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'fixture'); END`)
    if err!=nil {t.Fatal(err)}
    _,_,err=s.BeginRun(context.Background(),Begin{ID:strings.Repeat("a",32),JobRevisionID:jobID,RequestID:"req-1",PayloadHash:strings.Repeat("b",64)})
    if err==nil {t.Fatal("expected rollback")}
    var n int
    if err=s.db.QueryRow("SELECT count(*) FROM runs").Scan(&n);err!=nil {t.Fatal(err)}
    if n!=0 {t.Fatal("run survived failed outbox transaction")}
}
```

`Store.db *sql.DB` is the private T05 field used by same-package tests; production callers never receive it.

- [ ] Run `go test ./internal/state -run TestRunAndOutboxRollbackTogether -count=1 -v`; observe RED.
- [ ] Implement Activate as a transaction: insert-or-verify immutable config/hash bytes; create stable job IDs as SHA-256(configID + NUL + jobID); persist redacted channel revisions; switch active_config; cancel pending deliveries for removed/disabled channels; do not delete historical revisions/runs. Same hash with different snapshot bytes is a conflict, not an overwrite. Jobs returns only enabled jobs from active_config.
- [ ] Implement run mutation with parameterized SQL and the transaction boundaries below. PayloadHash binds job_revision_id and command semantics, not arbitrary raw JSON ordering. Reject unrecognized terminal states/codes; ID validation occurs before transactions. Work cancellation changes state, never runs a shell.

```text
BeginRun transaction:
  find request_id; if present require same job_revision_id and payload_hash
  identical request -> return stored run with replay=true and no new event
  require the job revision exists and is currently enabled
  insert queued run/version=1 and operation.queued event/sequence=1
  derive subscribed active channel revisions from the job's notify list
  insert only matching deliveries; zero channels -> zero rows
  commit; only after commit may callers observe accepted operation
Move transaction:
  compare run id + expected state/version; require allowed transition
  update state/version and terminal time/code
  insert matching event with sequence=new version and redacted typed payload
  insert eligible delivery rows in the same transaction; commit
  compare-and-set miss -> conflict, no event, no partial update
```

M01 state events are `operation.queued`, `operation.cancelled`, `operation.interrupted`; they are internal audit events and have no webhook subscription in v1. `backup.started` is emitted when a later executor enters preflight; `backup.completed/degraded/failed` only when later real execution reaches those outcomes. Do not generate fictional backup success in M01. Allowed transitions: queued->preflight/cancelled/interrupted; preflight->dumping/failed/cancelled/interrupted; dumping->delivering/failed/cancelled/interrupted; delivering->succeeded/degraded/failed/cancelled/interrupted. Terminal states reject further Move. Backend does not expose arbitrary Move over HTTP.

Implement `Store.Cancel` in runs.go with one transaction: look up run_commands.request_id, require identical runID/kind/payload hash on replay and return the stored terminal run; otherwise require queued state, update to cancelled and insert run_commands/result_version plus its internal event together. Reject use of a request ID already used for BeginRun and vice versa. A private `moveTx(ctx, tx, transition)` helper performs the same CAS/event rules inside the existing transaction; never call a public DB-opening method while holding the only connection. Concurrent requests serialize at the single writer; request-id checks and writes remain in one transaction.

- [ ] Implement ListRuns using stable descending `(started_at,id)` keyset predicates and limit+1, returning no raw config/credentials. Decode/reject invalid cursors/limits before SQL. GetRun unknown -> NotFound. ProbeWrite uses a short BEGIN/update tick/COMMIT and checks commit errors, so a readable DB cannot claim readiness without a successful durable write. Recover marks only queued/preflight/dumping M01-owned runs interrupted with an event in a transaction; delivering states require later provider reconciliation and must block operational readiness instead of blindly succeeding/retrying. Do not remove or reset existing history.
- [ ] Add `TestBeginRunReplayAndConflict` (same request repeat yields same ID/version, changed job/hash conflicts), `TestConcurrentBeginRunSingleEvent` (20 goroutines), `TestStaleTransitionRollback`, `TestTerminalStateImmutable`, `TestConfigHistorySurvivesRemoval`, `TestZeroChannelsNoDeliveries`, `TestRoutesUseChannelRevision`, `TestDisabledChannelCancelsPending`, `TestHistoryCursorStable`, `TestRecoverIsIdempotent`, `TestWriteProbeFullDisk`.
- [ ] Add replay-after-disable coverage. Change only active_config eligibility, not the original immutable revision or request binding.

```go
func TestReplaySurvivesDisabledJob(t *testing.T) {
    s,jobID:=newFixture(t);defer s.Close();ctx:=context.Background()
    b:=Begin{ID:strings.Repeat("a",32),JobRevisionID:jobID,RequestID:"req-1",PayloadHash:strings.Repeat("b",64)}
    first,_,err:=s.BeginRun(ctx,b);if err!=nil {t.Fatal(err)}
    raw,err:=os.ReadFile("../config/testdata/valid.yaml");if err!=nil {t.Fatal(err)}
    empty,err:=config.Decode(bytes.NewReader(raw));if err!=nil {t.Fatal(err)}
    if _,err=s.Activate(ctx,empty);err!=nil {t.Fatal(err)}
    replay,yes,err:=s.BeginRun(ctx,b)
    if err!=nil || !yes || replay.ID!=first.ID {t.Fatal("lost accepted request on config change")}
    b.RequestID="req-2";b.ID=strings.Repeat("c",32)
    if _,_,err=s.BeginRun(ctx,b);err==nil {t.Fatal("new work accepted for disabled job")}
}
```
- [ ] For full disk, set SQLite `max_page_count` to its current page count on a temporary fixture, consume available free pages with test-owned writes and perform a large bounded snapshot/run transaction; assert driver SQLITE_FULL, no partial operation/event, failed ProbeWrite where a real write cannot succeed and readiness unavailable. Also inject a driver write failure at the transaction seam to cover small writes that still fit free pages. Do not fill the host filesystem or accept chmod-only tests as SQLITE_FULL evidence.
- [ ] Run `go test ./internal/state -count=1`, `go test -race ./internal/state -run 'TestConcurrent|TestRunAndOutbox|TestHistory' -count=1`; expect PASS with exact row/event/delivery counts and no duplicated effects.
- [ ] Commit `feat(state): persist idempotent runs and event outbox`; record hash/evidence. M01 exposes primitives and fixture history; no worker actually dumps databases.

## T07 - Bounded Linux process supervision

**Dependencies:** T01. **Files:** T07 row. **Acceptance:** M01-AC08/11. **Consumes:** context, io.Writer, safeerr. **Produces:** Runner.Run. This is an internal API only, never a configurable arbitrary command endpoint.

- [ ] Use Go test-helper processes: the test binary reexecutes itself with `-test.run=TestProcessHelper -- <mode>`. `TestProcessHelper` returns normally unless `BKP_TEST_HELPER=1`; modes `exit7`, `stderr-flood`, `sleep`, `spawn-child`, `ignore-term` have fixed test-owned behaviors. Parent passes its own `os.Executable()` in AllowedExecutables. Implement fixture modes inside runner_linux_test.go, not a shell script.

```go
func TestCancelReapsProcess(t *testing.T) {
    exe,err:=os.Executable(); if err!=nil {t.Fatal(err)}
    ctx,cancel:=context.WithTimeout(context.Background(),100*time.Millisecond); defer cancel()
    start:=time.Now()
    result,err:= (Runner{AllowedExecutables:[]string{exe}}).Run(ctx,Spec{
        Executable:exe,Args:[]string{"-test.run=TestProcessHelper","--","ignore-term"},
        Env:[]string{"BKP_TEST_HELPER=1"},Dir:t.TempDir(),Stdout:io.Discard,
        StderrLimit:1024,Grace:50*time.Millisecond,
    })
    if err==nil || !result.Cancelled || time.Since(start)>2*time.Second {t.Fatal("cancellation not bounded")}
}
```

- [ ] Run `go test ./internal/process -run TestCancelReapsProcess -count=1 -v`; observe RED.
- [ ] Implement exact executable allowlist after canonical absolute-path validation; no PATH lookup or `sh -c`. `exec.Command` receives an argument array, explicit sanitized environment (no inherited provider secrets), `SysProcAttr.Setpgid=true`, validated working directory and stdout writer. Stderr is drained continuously while counting bytes; retain at most StderrLimit internally, never log or return raw bytes. Reject invalid/negative limit/grace and missing stdout writer.

```text
start child, launch bounded stream-copy goroutines, wait on exit or context
on cancellation/stdout failure: SIGTERM to negative process-group ID
after Grace: SIGKILL group; wait/reap direct child; drain/close pipes
bound pipe wait even if descendants inherited descriptors
on normal parent exit: terminate any remaining process-group members
return exit code/count/truncation/cancelled flags; safe error on nonzero/failure
never signal process group 0 or another job; retain started PID ownership
```

Use a documented 1-second pipe-drain deadline after termination; broken pipe/stdout writer errors are failure, not exit-0 success. All goroutines have joined before Run returns. Helper descendants must not survive; intentionally daemonized children that leave the process group are unsupported and prohibited by the internal executable contract. Later native tools are explicitly qualified against that contract.

- [ ] Add `TestNonzeroExit`, `TestStderrFloodIsBounded`, `TestStdoutFailureCancelsProducer`, `TestArgumentIsNotShellEvaluated`, `TestUnapprovedExecutableRejected`, `TestSpawnedChildDoesNotSurvive`, `TestNoInheritedSecrets`. For descendant test obtain PID through a dedicated helper channel, then assert ESRCH after teardown with bounded polling; no sleep-only success assertion.
- [ ] Run `go test ./internal/process -count=1`, `go test -race ./internal/process -count=1`; expect PASS, no orphan PIDs, bounded stderr memory and cancellation under test deadline.
- [ ] Commit `feat(process): supervise bounded native process groups`; record hash/evidence. No native PostgreSQL/MongoDB command is invoked in M01.

## T08 - Unix API and CLI client

**Dependencies:** T02/T06. **Files:** T08 row. **Acceptance:** M01-AC06/09 and CLI portion of M01.2. **Consumes:** control.Backend, safeerr, state DTOs. **Produces:** Server/Client and commands below. Tests use a fixed fake Backend; production Backend arrives in T09.

| Command | Method / path | M01 result |
|---|---|---|
| `version` | none | Build label; exit 0 |
| `config validate --config FILE [--check-secrets]` | none | Offline syntax by default; optional local secret check; no daemon |
| `serve --config FILE` | none | T09 composition only |
| `health --socket PATH` | GET `/v1/health` | Live/ready/components; nonzero CLI exit if not ready |
| `jobs list --socket PATH` | GET `/v1/jobs` | Active enabled job IDs and engine/source labels; no config snapshot/secret refs in response |
| `history list --limit N --after CURSOR --socket PATH` | GET `/v1/runs` | Bounded page of safe run records |
| `history show RUN --socket PATH` | GET `/v1/runs/{id}` | Safe run record or 404 |
| `operation cancel RUN --request-id ID --socket PATH` | POST `/v1/runs/{id}/cancel` | Queued-run cancellation only in M01; replay returns same result, conflict for another payload/terminal state |
| Backup/upload/verify/restore/retention/notification commands | Reserved route families | 501/not_implemented and CLI exit 4, zero side effects |

Wire envelope: `{request_id,status,data}` or `{request_id,status,error:{code,message,retryable}}`; message comes from a fixed code dictionary. Operation response adds `operation_id`; no raw config, paths, arbitrary SQL or argument echo. HTTP mapping: invalid 400, denied403, not_found404, conflict409, busy429, unavailable503, not_implemented501, internal500. Accepted operations later use202. GET health returns200 when ready,503 when not; data remains parseable. CLI exits 0 success,2 invalid input,3 unavailable/operational failure,4 unsupported. No caller can submit an arbitrary transition or command.

- [ ] Write handler tests using `httptest.NewRecorder` with an unexported handler constructor in package control; use Unix socket integration in server_linux_test.go. Fake Backend is a complete test struct implementing the six interface methods with fixed fields. Write:

```go
func TestServerUsesUnixSocket(t *testing.T) {
    path:=filepath.Join(t.TempDir(),"control.sock")
    s,err:=Listen(path,&fakeBackend{}); if err!=nil {t.Fatal(err)}
    defer s.Close(context.Background())
    done:=make(chan error,1); go func(){done<-s.Serve()}()
    client:=NewClient(path)
    body,err:=client.Do(context.Background(),"GET","/v1/jobs",nil)
    if err!=nil || !json.Valid(body) {t.Fatal("local request failed")}
    fi,err:=os.Stat(path); if err!=nil || fi.Mode().Perm()!=0600 {t.Fatal("socket permissions")}
}
```

- [ ] Run `go test ./internal/control -run TestServerUsesUnixSocket -count=1 -v`; observe RED.
- [ ] Implement net.ListenUnix with 0700 parent, 0600 socket and http.Server bounds. Existing regular/symlink/socket path causes conflict; never blindly unlink an arbitrary existing socket. T09 handles proved-owned stale sockets only while holding the state lock. On close, remove only the socket inode this Server created. Client uses Unix DialContext, fixed dummy HTTP host, Proxy nil, timeout5s and bounded response reads; all non-Unix schemes/paths rejected. No net.Listen("tcp") code path.
- [ ] Strictly decode JSON with duplicate-key token validation, DisallowUnknownFields, body limit and EOF check; validate request IDs and paths; return fixed safe errors. Implement private `decodeCancel(io.Reader) (string,error)` in server_linux.go: one object containing exactly one string request_id; reject repeated keys, nested values, unknown fields, trailing JSON and invalid IDs. Limit concurrent handlers to32; reject excess as busy. Handler context deadline5s. No hidden streaming/event endpoint.
- [ ] Add this Review Focus test (strings import); decoding fails before any Backend call:

```go
func TestCancelRejectsDuplicateKeys(t *testing.T) {
    _,err:=decodeCancel(strings.NewReader(`{"request_id":"one","request_id":"two"}`))
    if err==nil {t.Fatal("ambiguous request accepted")}
}
```
- [ ] Add `TestHTTPBoundsAndUnknownFields`, `TestHistoryLimits`, `TestCancelRequestReplay`, `TestExistingSocketNotRemoved`, `TestServerCloseDoesNotDeleteReplacement`, `TestClientDoesNotUseProxy`, `TestUnsupportedHasNoEffects`, `TestCLISafeErrors`, `TestClientOversizedResponse`. In fake Backend count mutations; rejected/unsupported requests must leave count0. Include a Unix-only malicious server returning secret-like raw500 text; client prints only generic internal/unavailable.
- [ ] Run `go test ./internal/control ./internal/cli -count=1`, `go test -race ./internal/control -count=1`; expect PASS and no TCP listener/secret output.
- [ ] Commit `feat(control): expose bounded local service commands`; record hash/evidence. Do not ship fake backends as production data.

## T09 - Daemon composition, recovery and milestone evidence

**Dependencies:** T03-T08. **Files:** T09 row. **Acceptance:** all M01 criteria, especially M01-AC02/09/11/12. **Consumes:** every shared contract. **Produces:** app.Serve, real control.Backend and complete Linux fixture acceptance.

- [ ] Create the app/integration fixtures before wiring. Tests build/reexec the actual `cmd/bkplight` binary and generate YAML with only temp paths. Use readiness polling over the Unix socket and bounded deadlines, not fixed sleeps. All temp dirs/artifacts are owned by the test; shutdown and reap the daemon in t.Cleanup even on failure.

```go
// internal/app/serve_linux_test.go, package app; imports context, os, path/filepath, testing
func TestInvalidConfigCreatesNoState(t *testing.T) {
    dir:=t.TempDir(); file:=filepath.Join(dir,"bad.yaml")
    if err:=os.WriteFile(file,[]byte("version: 999\n"),0600);err!=nil {t.Fatal(err)}
    if err:=Serve(context.Background(),file);err==nil {t.Fatal("invalid config started")}
    entries,err:=os.ReadDir(dir);if err!=nil {t.Fatal(err)}
    if len(entries)!=1 {t.Fatal("invalid startup mutated filesystem")}
}
```

- [ ] Run `go test ./internal/app -run TestInvalidConfigCreatesNoState -count=1 -v`; observe RED.
- [ ] Wire startup/shutdown in order. Define private `backend struct { store *state.Store; logs *observability.Logger; ready atomic.Bool }` in app/health_linux.go; implement all control.Backend methods. Cancel first checks readiness, then delegates to the transactional Store.Cancel from T06. The run_commands table is already owned by T05; do not alter a previously committed migration in this task.

```text
decode/validate YAML before creating state/log/socket directories
resolve/check active secret refs, close root handles and discard unneeded values
open state and hold singleton lock; refuse unsupported/missing schema evidence
open safe log sink; recover supported interrupted local runs
activate immutable config; probe durable writes; mark ready only if checks pass
publish Unix control socket; register signal.NotifyContext for SIGINT/SIGTERM
no scheduler, adapter worker or notification sender is started
on cancellation: stop accepting operations, drain HTTP for <=5 seconds
cancel any owned helper contexts; wait/reap using Runner grace/drain bounds
close owned socket, flush logs and state, release singleton last
```

Health returns `live:true`, `ready:true`, `scope:"foundation"` only when state/logs are usable; components explicitly report `scheduler:"not_implemented"`, `backup_engine:"not_implemented"`. It must not expose last-backup timestamps or protection status as healthy. Failed write probe or log sink makes ready false; cancel mutation returns503 without partial state. Cache write probe results for at most1 second to avoid concurrent health traffic creating a write storm; startup probe is immediate. Permission/full-disk errors log a fixed code to stderr if the sink is unusable.

- [ ] Stale-socket handling: while holding the exclusive state lock, require the configured parent directory to be private and owned, require existing path to be a socket, attempt a bounded Unix connection and reject if any listener responds. Only an unresponsive socket in that same protected runtime directory may be removed. State/config/runtime directory identity must remain fixed for this daemon; never recover a path outside it. Test two daemons with different state directories and same socket: second must not remove the first listener.
- [ ] Before startup mutation, resolve existing directory ancestors and reject symlink aliases that make log/staging/state paths overlap each other or secret roots. `TestResolvedDirectoryOverlapPreventsStartup` uses two different lexical paths resolving to the same temporary directory; assert Serve fails without database/socket/log creation. This runtime check complements T02's lexical test; trusted operator mounts remain part of deployment review.
- [ ] Add `TestFoundationLifecycle` (start actual binary, jobs/history empty, health foundation-only, SIGTERM, exit0, socket removed, restart retains config revision); `TestEnabledMissingSecretPreventsListen`; `TestDaemonSingleton`; `TestHealthDegradesOnStateWriteFailure`; `TestHealthDegradesOnLogFailure`; `TestCancelIdempotency` (same command replay, request reused for different run conflicts, terminal wrong command conflict); `TestCrashRecoveryDoesNotReplayEffects`; `TestNoNotificationsWithoutChannels`; `TestNoTCPListener` (inspect child `/proc/<pid>/fd` socket inode set against its namespace's TCP tables, no TCP sockets; Unix socket present); `TestShutdownReapsOwnedHelper`; `TestStaleSocketSafety`; `TestUnsupportedCommandsNoMutations`.
- [ ] Native-process and SQLite fixtures use no Docker, external database, storage account or real webhook. Actual network-denial proof can run the final test binary in an authorized isolated network namespace when available; absence is recorded as additional evidence not_run, not a claim of tested egress isolation. Production network isolation remains M08.
- [ ] Create development-environment.md with exact Linux toolchain/bootstrap/test commands below and evidence doc with candidate, tool/runtime versions, acceptance mapping, command/result/timestamp, review findings and limits. Link both in docs/README.md. README states foundation-only capabilities and local-only control. No Dockerfile/CI config is added in this milestone.
- [ ] Run the final verification sequence below after review fixes; record actual outputs. Use requesting-code-review and verification-before-completion; close blocking findings. Commit `feat(app): compose secure service foundation` and record all T01-T09 hashes/status/evidence, then use finishing-a-development-branch to prepare integration options. Merge needs H3; do not merge, push or deploy automatically.

## Exact verification commands (future authorized Linux execution)

Commands below are planned, not executed during document creation. Run from the implementation checkout root as a non-root user. H2 must authorize dependency resolution and fixture execution. They require a supplied Go1.26.10 Linux toolchain and C compiler; they do not install either.

```sh
go version
go env GOOS GOARCH
cc --version
export GOTOOLCHAIN=local
go get go.yaml.in/yaml/v3@v3.0.5 modernc.org/sqlite@v1.61.0 github.com/robfig/cron/v3@v3.0.1
go mod tidy
go mod verify
go list -m all
go test ./internal/... -count=1 -timeout 120s
go test -race ./internal/... -count=1 -timeout 180s
go test ./internal/config -run '^$' -fuzz FuzzDecode -fuzztime 10s
go test ./tests/integration -count=1 -timeout 120s
go vet ./...
go build -trimpath -o bin/bkplight ./cmd/bkplight
./bin/bkplight version
./bin/bkplight config validate --config examples/bkplight.yaml
git diff --check
```

Expected: Go toolchain exactly selected version, GOOS linux/GOARCH amd64, checksum verification succeeds, tests/race/vet/build exit0, example validation exits0 without state/secret/network access. Dependency selection commands run in T02/T05 only as dependencies are introduced; the final verification reruns module verification/listing and checks `go.mod`/`go.sum` match reviewed pins, not an uncontrolled upgrade. A different supported Linux architecture requires its own native evidence before support claims.

Check formatting without silently rewriting reviewed files:

```sh
gofmt -l cmd internal tests
git diff -- go.mod go.sum
git status --short
```

Expected gofmt output empty. Inspect dependency diffs; transitive modules must be checksum-locked and match driver's libc requirements. Full test packages must contain and execute the named tests; a command printing `[no test files]` or a skipped platform suite does not satisfy acceptance. Race tests require CGO_ENABLED=1 and a working C compiler even though SQLite itself is pure Go. Never treat a Windows cross-build as Linux execution evidence.

### Per-task commit verification

Stage only the active task's reviewed paths from the file map. Each task supplies its literal commit subject; stage commands below make file selection explicit. Review working and staged diffs, test freshness and secrets before committing:

```sh
git diff --cached --name-status
git diff --cached --stat
git diff --cached --check
git diff --cached
```


Exact task staging commands (run only the command for the task just verified):

```sh
# T01 after its green tests and review
git add -- go.mod cmd/bkplight/main.go internal/cli/run.go internal/cli/run_test.go internal/safeerr/error.go internal/safeerr/error_test.go

# T02 after its green tests and review
git add -- internal/config/types.go internal/config/decode.go internal/config/validate.go internal/config/revision.go internal/config/config_test.go internal/config/fuzz_test.go internal/config/testdata/valid.yaml examples/bkplight.yaml internal/cli/run.go internal/cli/run_test.go go.mod go.sum

# T03 after its green tests and review
git add -- internal/secrets/files_linux.go internal/secrets/files_linux_test.go internal/cli/run.go internal/cli/run_test.go

# T04 after its green tests and review
git add -- internal/observability/logger.go internal/observability/rotate.go internal/observability/logger_test.go internal/observability/rotate_test.go

# T05 after its green tests and review
git add -- internal/state/open_linux.go internal/state/lock_linux.go internal/state/migrate.go internal/state/migrations/001_foundation.sql internal/state/open_linux_test.go internal/state/migrate_test.go go.mod go.sum

# T06 after its green tests and review
git add -- internal/state/types.go internal/state/config.go internal/state/runs.go internal/state/events.go internal/state/health.go internal/state/runs_test.go internal/state/config_test.go internal/state/events_test.go

# T07 after its green tests and review
git add -- internal/process/runner_linux.go internal/process/runner_linux_test.go

# T08 after its green tests and review
git add -- internal/control/types.go internal/control/server_linux.go internal/control/client_linux.go internal/control/server_linux_test.go internal/control/client_linux_test.go internal/cli/run.go internal/cli/run_test.go

# T09 after its green tests and review
git add -- internal/app/serve_linux.go internal/app/health_linux.go internal/app/serve_linux_test.go tests/integration/foundation_linux_test.go docs/06-testing/m01-foundation-evidence.md docs/03-planning/development-environment.md cmd/bkplight/main.go internal/cli/run.go internal/cli/run_test.go README.md docs/README.md docs/03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md docs/05-progress/current-status.md docs/08-agent-memory/session-handoff.md
```

Use the task's literal commit subject and include task ID/actual tests in its body. After commit run `git rev-parse HEAD`, `git show --stat --oneline HEAD`, `git status --short`. Record the hash after it exists, then persist bookkeeping as described in the Git workflow. Never stage another task's changes or amend/rewrite user history. Required application checks failing means task remains failed/blocked, not done.

## Failure recovery, review and handoff

Each task has a default investigation budget of three distinct evidence-backed hypotheses or 60 minutes of active debugging, whichever comes first. Stop dependent tasks when exhausted; retain the last reviewed commit, failing command/output and hypothesis record. This is a stop/replan limit, not a promised task duration. Tool download/environment failures are not red TDD behavior failures.

Rollback for unreleased M01 uses a preserved compatible test state snapshot and previous task commit in a separate checkout; never reset shared user work. Only delete test-owned temporary directories after verifying ownership/path. Migration failure rolls back the transaction and retains the pre-migration snapshot; no arbitrary down migration. Secrets/exposure, unexpected external requests, wrong-target writes or unbounded process survival stop the task immediately. Material contract changes need scoped H1/H2 renewal.

At each task completion update this task register and evidence doc with the real commit(s), exact environment/command/result/time, red/green evidence, review findings and status. Final review checks AC01-AC12, every public signature, actual files, no accidental production API, and no later-milestone implementation. Update current-status/handoff; keep integration and release separate. No runtime result is currently recorded.

## Plan self-review and approval request

This plan maps all three M01 work packages and their exit conditions to T01-T09. It explicitly limits later database/provider/scheduler functionality, names required fixtures, differentiates compilation/environment failures from red tests, and exposes missing M00/Linux/H2 prerequisites. The tables supply exact paths/contracts; code blocks are planning material only and were not compiled. Proposed defaults/packages require qualification during execution; no implementation or passing runtime evidence is implied.

Before execution the human must approve this revision and select **inline executing-plans** or **subagent-driven-development**, plus an authorized Linux fixture workspace and the applicable M00 completion/disposition. Until then all tasks stay backlog. This is the requested reviewable plan, not an execution grant.

## Related documentation

- [Roadmap](../roadmap.md)
- [Milestones](../milestones.md)
- [Dependency register](../task-dependencies.md)
- [Lifecycle](../development-lifecycle.md)
- [Task template](../task-template.md)
- [Git workflow](../git-workflow.md)
- [Quality gates](../../06-testing/quality-gates.md)
- [Current status](../../05-progress/current-status.md)
- [Session handoff](../../08-agent-memory/session-handoff.md)
