---
name: "deploy-production"
description: "Use when preparing or carrying out an explicitly authorized BkpLight production release with target-specific health and rollback controls."
---

# Deploy production

**Document status:** Setup procedure only; deployment is not authorized.

## Purpose

Define controlled release preparation, final approval, deployment checks and rollback without implying permission to mutate production.

## Authorization boundary

Read [AGENTS.md](../../../AGENTS.md) first. During setup, report the active gate and missing prerequisites, then stop without production connections, credential requests, package installs, deployments or migrations. Invoking this skill cannot override the setup-only instruction.

## Future controlled procedure

1. Once further work is authorized, identify the target, operator, immutable release artifact and exact scope.
2. Review stage 13 evidence under the quality gates and production release process: same-artifact staging checks, required tests, current backup coverage and isolated restore evidence, migration safety, rehearsed rollback, working monitoring and operator ownership. Complete a concrete deployment plan, reviewed commands, access references, preflight checks, health thresholds, observation period and rollback criteria. Never invent unknown commands or execute TODO placeholders. Missing mandatory evidence blocks production.
3. Present that reviewable plan for H4 final explicit human production approval, including authorized rollback scope. Reuse an existing approval only when it covers this exact scope, target and release and its conditions remain valid; a readiness verdict alone is not approval. Changed artifact/target or invalidated evidence requires renewed approval.
4. Recheck candidate/target, backup freshness and integrity, restore dependencies, rollback availability and monitoring before mutation; if a final backup is in the approved procedure, complete and validate it before deployment. Execute only approved steps, recording redacted outcomes. Run health checks against the agreed thresholds and observe for the agreed period.
5. On a rollback trigger, follow the approved rollback procedure within its authorization. If rollback is unapproved, unsafe, fails or is incompatible with data state, stop further mutation, preserve evidence and invoke incident-response within authorized scope; request the needed decision and do not improvise destructive recovery.
6. Record the deployed or rolled-back version, health evidence, remaining issues and handoff. Do not claim success before fresh verification.

## Output contract and example

During setup: state that deployment was not performed and list missing prerequisites. After an authorized release: provide artifact/target identity, approval reference, command outcomes, health and rollback results. TODO: target-specific runbook after implementation and release planning approval.

## Related documentation

- [Deployment guide](../../../docs/07-deployment/deployment-guide.md)
- [Release checklist](../../../docs/06-testing/release-checklist.md)
- [Rollback plan](../../../docs/07-deployment/rollback-plan.md)
- [Backup and recovery](../../../docs/07-deployment/backup-recovery.md)
- [Monitoring](../../../docs/07-deployment/monitoring.md)
- [Development lifecycle](../../../docs/03-planning/development-lifecycle.md)
- [Quality gates](../../../docs/06-testing/quality-gates.md)
- [Production release process](../../../docs/07-deployment/production-release-process.md)
- [Incident response](../incident-response/SKILL.md)
