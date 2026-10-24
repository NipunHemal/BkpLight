# Quality gates

**Document status:** Active workflow criteria; product evidence and commands remain TODO.

## Purpose

Define evidence required for stage transitions, task completion and production release. This document authorizes no execution.

## Evidence contract

Each check records requirement/risk, candidate revision or immutable artifact, environment, command/procedure, expected result, actual outcome, timestamp and redacted evidence reference. Use `pass`, `fail`, `blocked`, `not_run` or `not_applicable`. A claimed pass without evidence cannot satisfy a gate. A not-applicable result needs a scoped rationale; it cannot erase an explicit mandatory production prerequisite.

| Gate | Required evidence | Failure route |
|---|---|---|
| Design / H1 | Requirements traceability, design alternatives, contract/security review, disposition of blocking findings, accepted ADR and human approval references | Return to architecture/discovery; no implementation |
| Plan / H2 | Approved design, scoped tasks, satisfied dependencies, exact tests/commands, recovery steps, selected execution mode and human approval | Replan; task stays backlog/blocked |
| Workspace | Checkout identity, clean/known unrelated changes, relevant baseline checks | Record preexisting failures; resolve or obtain required human decision before proceeding |
| Per-task TDD | Expected failing behavior test before code, passing correction, refactor/regression evidence | Systematic debugging; no masking or test weakening |
| Review | Specification and quality findings, technical verification of feedback, closed blockers | Return changed tasks to implementation and reverify |
| Feature completion | Fresh relevant test suite, build/static checks and integration/acceptance checks required by plan; docs/status/handoff current | Task cannot be done while required checks are missing or failing |
| Integration / H3 | Verified candidate, exact integration authorization, merged-result checks where merged | Preserve branch/worktree and diagnose; do not proceed to release |
| Staging | Release artifact promoted unchanged to isolated staging; representative engine/config versions; smoke, integration, failure/recovery and upgrade checks as applicable | Block production; fix through task loop and repeat affected candidate checks |
| Backup and restore | Fresh pre-release backup identity/time, required coverage, integrity and usable keys/access; isolated restore evidence for the applicable format/version and accepted recovery objectives | Block production; repair backup/recovery path |
| Migration and rollback | Forward/backward compatibility assessment, data-change risks, approved previous artifact/config or recovery path, staging rollback rehearsal and exact triggers | Block production; do not assume image rollback reverses data changes |
| Monitoring | Working health checks, log collection/rotation/redaction, alert routing or explicit no-webhook configuration, overdue detection and operator ownership as required by approved design | Block production; monitoring is mandatory even if webhooks are disabled |
| Production / H4 | All release gates pass, concrete runbook, exact artifact/target approval, current preflight and authorized rollback scope | Stop before mutation; refresh invalidated evidence/approval |
| Post-deploy | Health thresholds, observation-period results, version identity, incident/rollback outcome | Approved rollback or incident-response with authorization boundaries |

## Test selection and freshness

Use meaningful tests at the level where behavior can fail: unit tests for local logic, contract/integration checks for adapters and persistence, isolated runtime exercises for restores and recovery. Documentation checks do not prove any of these. TDD nests within implementation and is repeated for each behavior change.

Run required checks after the final relevant change. Record known baseline failures separately; do not silently ignore unrelated failures found in required suites. A change to code, configuration, schema, dependency, artifact or target invalidates affected evidence. Tests passing on one artifact do not establish a different artifact's readiness.

## Staging and production safety

Staging, backups with restore evidence, rollback and monitoring are mandatory before production. Do not test a restore against production to satisfy a staging gate. Use isolated data and credentials. A local staging environment may share a host only when its resources, identities and destructive targets are isolated and reviewed. Product-specific topology and commands remain TODO.

No agent can mark a required production gate not applicable because evidence is expensive or unavailable. Report blockers and obtain decisions within user scope; never infer deployment approval from a readiness verdict.

## Setup verification

For workflow-only changes, validate links, frontmatter, stage/skill mapping, human gates, task/release status distinctions, preservation and scenario behavior. No application test/build/deployment claim follows from these checks.

## Related documentation

- [Development lifecycle](../03-planning/development-lifecycle.md)
- [Task template](../03-planning/task-template.md)
- [Test strategy](test-strategy.md)
- [Release checklist](release-checklist.md)
- [Production release process](../07-deployment/production-release-process.md)
