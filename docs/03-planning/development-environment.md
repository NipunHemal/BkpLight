# M01 Linux development environment

**Status:** Qualified synthetic fixture environment, 2026-10-26. Production inventory remains open in M00.

M01 runs on Linux amd64 with Go 1.26.10 and a C compiler for race tests. The recorded environment is Ubuntu 24.04 in WSL2, kernel `6.6.87.2-microsoft-standard-WSL2`, GCC 13.3.0, host UID 1000 and a native Linux filesystem. Windows/DrvFS execution and other architectures are not qualified by this evidence. Install/bootstrap of the recorded Go and C tools was separately authorized and is recorded in the [preflight](../06-testing/m01-workspace-preflight.md#wsl-tool-installation---2026-10-25); these commands assume those tools already exist.

Use a private native Linux checkout or snapshot; do not run permission, flock, SQLite or Unix socket fixtures on `/mnt/c`. The managed Windows checkout remains the source/Git authority. In this session the existing ignored snapshot helper was run **before each Go command**:

```powershell
wsl -d Ubuntu --exec sh /mnt/c/Users/USER/.codex/worktrees/m01-secure-foundation/BkpLight/.superpowers/sdd/2026-10-24-m01-secure-service-foundation/sync-wsl.sh
```

That session-specific helper synchronizes to `/home/hemal/.bkplight-m01-fixture-eba7f1c`, requires host UID nonzero, verifies the native filesystem and ownership marker, excludes Git/agent reports, and makes directories/files private. It is ignored workflow tooling, not a shipped bootstrap script. For a different checkout, establish an equivalent private native workspace and verify its source before testing. Run the following there as the non-root owner:

```sh
export GOTOOLCHAIN=local TMPDIR=/tmp
go version
go env GOOS GOARCH CGO_ENABLED
cc --version
go mod verify
go list -m all
go test ./internal/... -count=1 -timeout 120s
CGO_ENABLED=1 go test -race ./internal/... -count=1 -timeout 180s
go test ./internal/config -run '^$' -fuzz FuzzDecode -fuzztime 10s
go test ./tests/integration -count=1 -timeout 120s
go test ./... -count=1 -timeout 120s
go vet ./...
go build -trimpath -o bin/bkplight ./cmd/bkplight
./bin/bkplight version
./bin/bkplight config validate --config examples/bkplight.yaml
gofmt -l cmd internal tests
git diff --check
git diff -- go.mod go.sum
```

Dependency versions are already pinned. Do not rerun `go get` or upgrade dependencies as part of routine verification. Module downloads need the approved dependency-resolution scope; provider credentials, real databases and network services are unnecessary for these fixtures.

The integration harness builds the actual `cmd/bkplight` binary, creates private `/tmp/bkl-*` fixtures with synthetic YAML, polls the Unix API with deadlines, and terminates/reaps every daemon in cleanup. Readiness-failure tests deliberately inject a synthetic SQLite trigger and remove log-directory write permission. Those are test-owned fault injections; ordinary CLI processes never open SQLite.

Optional additional evidence, when unprivileged user/network namespaces are available:

```sh
unshare --user --map-current-user --net sh -c \
  'GOTOOLCHAIN=local TMPDIR=/tmp go test ./tests/integration -count=1 -timeout 120s'
```

The 2026-10-26 run succeeded with UID 1000 mapped to host UID 1000 and no configured network routes. The daemon still uses its private Unix socket. This establishes fixture operation without external networking; production isolation and deployment remain M08 work.

For a local fixture daemon, create YAML whose state/staging/log/secret directories are separate private absolute paths and whose socket parent is owned by the service UID with mode 0700. The daemon creates missing state/log/socket parents, keeps SQLite/socket files mode 0600, and does not create staging archives in M01. A private same-parent `.<socket-name>.lock` file serializes recovery, binding and cleanup across daemons even when their state directories differ. Its nonblocking flock is retained until after listener close; the lock file remains on disk after shutdown and is reused. Do not unlink that ownership inode while a daemon is running. Symlink/hardlink/nonprivate lock files are rejected, and the held runtime-directory identity is checked before the listener is served. Validate first, then run `bin/bkplight serve --config /absolute/fixture.yaml`; inspect `bin/bkplight health --socket /absolute/run/control.sock`. Use SIGINT or SIGTERM for an orderly stop. Enabled dependencies require readable private secret files before state is created. Config changes take effect on controlled restart.

See the [foundation evidence](../06-testing/m01-foundation-evidence.md) for actual results and limits. No production connection, merge, push, deployment or real backup is authorized by these instructions.
