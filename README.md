# BkpLight

**Status:** M01 Linux foundation reviewed and locally merged into `main`; M02 planning is in progress. No production release.

## Purpose

Build a single-server database backup service in stages, following the approved architecture and scoped implementation plan.

## Current state

The Linux Go daemon provides strict offline YAML validation, bounded secret-file resolution, private SQLite ownership and configuration/run history primitives, safe rotating logs, internal native-process supervision and a restricted Unix-socket control API. Config changes apply on controlled restart. State has one daemon owner; CLI commands use local IPC.

Health explicitly reports scope `foundation`, with scheduler and backup engine `not_implemented`. M01 does not perform dumps, uploads, restores, retention, scheduled work or webhook delivery. Backup/provider commands return unsupported status. No Docker deployment or CI configuration is shipped in this milestone.

After building on the [qualified Linux development environment](docs/03-planning/development-environment.md):

```sh
bin/bkplight config validate --config examples/bkplight.yaml
bin/bkplight serve --config /absolute/private-fixture.yaml
bin/bkplight health --socket /absolute/private-runtime/control.sock
```

Use fixture-specific private directories and active secret files before starting the daemon; the example uses deployment-style paths and is safe for offline validation. Serve handles SIGINT/SIGTERM and releases its owned socket, logs, state and singleton lock. Control has no TCP listener.

## Start here

Read the agent instructions, documentation index and current status. Use accepted architecture v0.1/P01-P09 as the design baseline; the milestone roadmap is not an executable implementation plan.

## Next phase

M01 task and whole-branch reviews are closed. The user approved its local fast-forward merge into `main`, and full native WSL tests passed after integration. M00 production inventory and M02 planning remain open. M02 application code needs an approved detailed plan and execution mode; push and deployment remain unapproved. See [foundation evidence](docs/06-testing/m01-foundation-evidence.md), [current status](docs/05-progress/current-status.md) and the [task register](docs/03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md#task-register-and-order).

## Related documentation

- [Milestone roadmap](docs/03-planning/roadmap.md)

- [Working PRD](docs/01-requirements/PRD.md)
- [Architecture proposal and approval choices](docs/02-architecture/system-overview.md)
- [Agent instructions](AGENTS.md)
- [Documentation index](docs/README.md)
- [Current status](docs/05-progress/current-status.md)
