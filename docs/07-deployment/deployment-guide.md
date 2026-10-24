# Deployment guide

**Document status:** Template - not approved.

## Purpose

Reserve a controlled runbook for future explicitly approved deployments.

## Authorization

Setup only: no deployment commands or production access are authorized. TODO: release, target, operator, window and approval evidence.

## Prerequisites

TODO: immutable artifact, configuration, secret references, permissions, readiness evidence, backups and rollback prerequisites.

Apply the production release process and quality gates: staging validation, current backup/restore evidence, rehearsed compatible rollback, active monitoring and H4 exact-target/artifact human approval are mandatory. Fill this target-specific runbook only when that work is authorized; workflow setup does not authorize staging or production operations.

## Procedure

TODO: exact reviewed commands and expected results after implementation and deployment planning approval. Do not invent commands or execute placeholders.

## Health and stop conditions

TODO: pre/post checks, observation period, thresholds and rollback triggers.

## Deployment record

TODO: approval reference, actual commands, health evidence and final state.

## Related documentation

- [Production release process](production-release-process.md)
- [Quality gates](../06-testing/quality-gates.md)
- [Release checklist](../06-testing/release-checklist.md)
- [Rollback plan](rollback-plan.md)
- [Deployment skill](../../.agents/skills/deploy-production/SKILL.md)
