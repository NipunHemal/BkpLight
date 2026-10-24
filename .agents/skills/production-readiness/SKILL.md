---
name: "production-readiness"
description: "Use when assessing BkpLight release readiness, reviewing operational evidence, or identifying missing production prerequisites."
---

# Production readiness

**Document status:** Setup skill; no release or runtime evidence exists yet.

## Purpose

Assess a concrete release using actual evidence without deploying or substituting documentation for runtime validation.

## Procedure

1. Read [AGENTS.md](../../../AGENTS.md), [current status](../../../docs/05-progress/current-status.md) and the [release checklist](../../../docs/06-testing/release-checklist.md).
2. Identify the release artifact, target, approved acceptance criteria and scope. Missing implementation or criteria means readiness is not established.
3. Assess applicable evidence for tests/builds, security and secrets, least privilege, supported versions, migration safety, backup integrity, isolated restore, monitoring/notifications, resource limits, rollback and recovery.
4. Label each item pass, fail, blocked, not run or not applicable, with evidence and rationale. Distinguish a failed command from a command that does not exist.
5. Run only checks authorized for this task. During setup, inspect documentation only; do not install packages, access production, send test webhooks, start containers or run restores.

## Output contract

Report the release/target assessed, evidence matrix, confirmed blockers, unknowns and verdict. Readiness review grants no deployment permission. Use installed Superpowers verification skills where applicable rather than duplicating them.

## Stage 13 production prerequisites

Apply quality gates and the production release process to the integrated immutable candidate. Require same-artifact staging evidence, reviewed environment differences, required tests/builds/integration checks, fresh applicable backup coverage and isolated restore evidence, migration compatibility, rehearsed rollback and working monitoring/operator ownership. Optional webhooks may be disabled; core monitoring may not.

Classify a missing mandatory prerequisite as a release blocker. Do not mark it not applicable to bypass the gate or infer health from unit tests. Evidence must match current artifact/configuration and relevant engine versions. Changed candidates return to affected checks. Complete a concrete release record and runbook before seeking H4 for the exact target/artifact; this skill neither authorizes nor performs deployment. During setup, report missing evidence rather than creating infrastructure or executing release checks.

## Example and common mistake

With no application or build command, state readiness is not established and tests/builds are not run; do not claim they passed or failed. TODO: populate real release evidence after authorization.

## Related documentation

- [Test strategy](../../../docs/06-testing/test-strategy.md)
- [Security design](../../../docs/02-architecture/security-design.md)
- [Backup and recovery](../../../docs/07-deployment/backup-recovery.md)
- [Monitoring](../../../docs/07-deployment/monitoring.md)
- [Rollback plan](../../../docs/07-deployment/rollback-plan.md)
- [Quality gates](../../../docs/06-testing/quality-gates.md)
- [Production release process](../../../docs/07-deployment/production-release-process.md)
