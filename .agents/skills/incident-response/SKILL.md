---
name: "incident-response"
description: "Use when triaging a BkpLight operational incident, failed deployment, recovery failure, or repeated backup or verification outage."
---

# Incident response

**Document status:** Active workflow procedure; no operational action is authorized by setup.

## Purpose

Preserve evidence, assess impact and guide verified recovery within explicit access and mutation authorization.

## Authorization boundary

Read AGENTS.md and the current incident/release authorization. During setup, document the procedure only; do not connect to production, send alerts, request credentials, start containers or perform recovery. Skill invocation and urgency do not grant permissions. Reuse valid explicit authorization for its exact scope; obtain a human decision before additional destructive recovery or containment.

## Response loop

1. Record incident ID/time, reporter, affected environment/candidate, symptoms, impact, severity rationale, owner and known last healthy state. TODO: actual severity thresholds, contacts and response budgets after operations are authorized.
2. Preserve redacted logs, timeline, alerts, task/release evidence and backup identities using permitted access. Separate observed facts from hypotheses; do not overwrite evidence or expose secrets.
3. Triage dependencies and failure boundaries using installed `superpowers:systematic-debugging`. Stop affected releases/jobs only if covered by authorization. Do not disable backup retention protections, monitoring or tests to hide the incident.
4. Propose one evidence-backed recovery action at a time with expected outcome, attempt/time budget, health criteria and rollback implications. Execute only approved actions. No unbounded retries, blind restarts, volume deletion or improvised production restores.
5. Use approved rollback/recovery runbooks and compatible data state. If authorization, backup integrity, keys, target identity or safe recovery is missing, stop mutation and escalate the concrete decision to the human. Do not assume image rollback reverses migrations.
6. Verify recovery using actual health, data-integrity and affected behavior checks for the recorded candidate/target, plus the agreed observation period. If checks fail, preserve the new evidence and remain in incident response.
7. Record resolution or unresolved handoff, actual actions and results, remaining risks and follow-up owners. Corrective changes re-enter discovery/design gates when scope changes, then planning, TDD, review and verification. Significant changes need an ADR and affected H1/H2 approval; production redeployment always needs H4.

## Output contract and common mistake

Provide incident state, impact, timeline/evidence, approval scope, actions attempted, verification, remaining blockers and next responsible action. Never call an incident resolved because a process restarted or a webhook was delivered. Never send incident messages to others without explicit authorization.

## Related documentation

- [Agent instructions](../../../AGENTS.md)
- [Development lifecycle](../../../docs/03-planning/development-lifecycle.md)
- [Monitoring](../../../docs/07-deployment/monitoring.md)
- [Backup and recovery](../../../docs/07-deployment/backup-recovery.md)
- [Rollback plan](../../../docs/07-deployment/rollback-plan.md)
- [Production release process](../../../docs/07-deployment/production-release-process.md)
- [Session handoff](../../../docs/08-agent-memory/session-handoff.md)
- [Lessons learned](../../../docs/08-agent-memory/lessons-learned.md)
