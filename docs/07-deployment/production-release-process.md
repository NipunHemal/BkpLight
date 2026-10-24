# Production release process

**Document status:** Active release workflow; no release, target or deployment is authorized during setup.

## Purpose

Make staging, backup, rollback, monitoring and final human approval prerequisites to any production mutation.

## Release record template

| Field | Required value |
|---|---|
| Release ID, owner and operator | TODO |
| Exact artifact version/digest and source revision | TODO |
| Target/environment identity and configuration revision | TODO |
| Related tasks, accepted ADRs and integration/H3 evidence | TODO |
| Staging evidence and reviewed differences from production | TODO |
| Backup IDs, coverage, integrity and restore rehearsal evidence | TODO |
| Migration compatibility and data recovery implications | TODO |
| Previous artifact/config and rollback rehearsal | TODO |
| Health thresholds, observation period and monitoring owner | TODO |
| Exact deployment/rollback commands and triggers | TODO |
| H4 human approver, time, instruction reference and approved rollback scope | TODO |
| Actual outcome, incident reference and final artifact/target state | TODO |

Use a dated release record in this folder when release preparation is authorized. No record or runnable command is created by this template.

## R1 - Prepare and validate in staging

Entry: integrated, verified candidate, approved release-preparation scope and a defined staging environment.

Deploy the candidate to isolated staging within authorized access. Use the same immutable artifact intended for production. Validate smoke/integration checks, relevant upgrade/migration paths, backup restoration, failure recovery, rollback, health/logging and monitoring. Record expected production differences and their risks. Protect production credentials and volumes from staging operations.

Exit: candidate-specific staging evidence passes and differences are reviewed. A rebuilt or changed candidate returns to affected staging checks.

## R2 - Complete readiness and prepare final approval

Entry: R1 evidence. Use custom `production-readiness` against quality gates and the release checklist.

Prepare the complete target-specific runbook first: exact commands, access references, required pre-release backup procedure, restore evidence, migration precautions, rollback artifact/steps/triggers, health thresholds, observation period, alert handling and operator ownership. Backups must cover the current production state before deployment; validate their freshness at preflight. Existing rehearsal evidence must match the applicable versions/format and approved recovery objectives.

Exit: readiness verdict ready, all mandatory evidence present and the release record is concrete enough for human review. TODO: actual commands, thresholds, environment inventory and recovery objectives after product approval.

## R3 - Human production approval: H4

Present the finished plan and evidence. Obtain explicit human approval identifying exact artifact, target, scope, allowed production actions and rollback scope. An earlier design/plan approval, green tests, a ready verdict or a skill invocation is insufficient.

Existing explicit approval can be reused only while it covers the unchanged release/target and its conditions remain satisfied. Do not deploy if approval is missing, unclear, revoked or invalidated. Do not ask again for the same still-valid authorization.

## R4 - Preflight and controlled deployment

Entry: H4, unchanged candidate/target and ready evidence.

Confirm access, capacity, current state, backup freshness and integrity, usable restore dependencies, rollback availability and active monitoring. If the approved runbook includes taking a final production backup, complete it and validate its result before changing the release. Stop on any unsatisfied prerequisite. Execute only reviewed steps covered by approval; record outcomes with secrets redacted. This document provides no concrete deployment commands.

Exit: the approved candidate is deployed or an incident/rollback is active; neither outcome alone means health is verified.

## R5 - Verify and observe

Run agreed health and smoke checks, confirm version identity and watch monitoring for the specified observation period. Record actual measurements against approved thresholds. Optional notification channels can be disabled; core health, logs and operator visibility remain required. If a configured notification channel is a release-critical signal, its failure blocks closing the release.

Exit: healthy, observed candidate with evidence, or a rollback/incident decision based on recorded triggers.

## R6 - Roll back or escalate

Use the approved rollback procedure only within recorded authorization and data compatibility. Restore operations and irreversible migrations need explicit recovery scope; do not assume reverting an image restores data. If the procedure is unsafe, unapproved or fails, stop further mutation, preserve evidence and invoke custom `incident-response`. No unlimited retry loops or improvised destructive restores.

Exit: previous/approved state restored and verified, or unresolved incident handed to the responsible human with actual state and evidence.

## R7 - Close and maintain

Record deployed/rolled-back artifact, target, approvals, backup identity, commands, health/observation results, remaining issues and incident links. Update current status and handoff; open corrective tasks that re-enter design/TDD/review/verification as appropriate. Do not report a failed or unobserved release as successful.

## Related documentation

- [Development lifecycle and H0-H4](../03-planning/development-lifecycle.md)
- [Quality gates](../06-testing/quality-gates.md)
- [Release checklist](../06-testing/release-checklist.md)
- [Deployment guide](deployment-guide.md)
- [Rollback plan](rollback-plan.md)
- [Backup and recovery](backup-recovery.md)
- [Monitoring](monitoring.md)
- [Deploy-production skill](../../.agents/skills/deploy-production/SKILL.md)
- [Incident-response skill](../../.agents/skills/incident-response/SKILL.md)
