# Documentation index

**Document status:** Active navigation; architecture v0.1 approved under H1, roadmap v0.1 prepared; implementation unapproved.

## Purpose

Navigate project documentation and distinguish templates from approved or verified work.

## Status and authority

- **Template:** structure exists; substantive information remains TODO.
- **Draft:** content exists but is not approved.
- **Approved:** dated explicit approval evidence is recorded.
- **Verified:** a named check has current evidence; this is not blanket readiness.

The user approved architecture v0.1/P01-P09 and ADR-001 on 2026-10-24, then requested a milestone roadmap using Superpowers writing-plans with no implementation. The roadmap, milestone exit criteria and dependencies are populated under 03-planning. Detailed executable plans and H2/H3/H4 remain outstanding; unknown deployment inventory is not established by design approval. The subsequent Git setup request authorizes repository initialization, safe setup commits and task-wise commit policy, with no application features, merge or deployment.

## 01 Requirements

- [Working PRD and clarification log](01-requirements/PRD.md)
- [Project brief](01-requirements/project-brief.md)
- [Functional requirements](01-requirements/functional-requirements.md)
- [Non-functional requirements](01-requirements/non-functional-requirements.md)
- [Acceptance criteria](01-requirements/acceptance-criteria.md)
- [Open questions](01-requirements/open-questions.md)

## 02 Architecture

- [System overview](02-architecture/system-overview.md)
- [Technology stack](02-architecture/tech-stack.md)
- [Database design](02-architecture/database-design.md)
- [API and interface design](02-architecture/api-design.md)
- [Security design](02-architecture/security-design.md)
- [Infrastructure design](02-architecture/infrastructure-design.md)

## 03 Planning

- [Git and task-wise commit workflow](03-planning/git-workflow.md)

- [Development lifecycle: all 15 stages and human gates](03-planning/development-lifecycle.md)
- [Per-task record template](03-planning/task-template.md)
- [Roadmap](03-planning/roadmap.md)
- [Milestones](03-planning/milestones.md)
- [Task dependencies](03-planning/task-dependencies.md)
- [Implementation plans](03-planning/implementation-plans/README.md)

## 04 Decisions

- [Decision register](04-decisions/README.md)
- [ADR-001: accepted single-server architecture](04-decisions/ADR-001-single-server-backup-architecture.md)
- [ADR template](04-decisions/ADR-template.md)

## 05 Progress

- [Current status](05-progress/current-status.md)
- [Completed milestones](05-progress/completed-milestones.md)
- [Blockers and limitations](05-progress/blockers.md)

## 06 Testing

- [Test strategy](06-testing/test-strategy.md)
- [Quality gates](06-testing/quality-gates.md)
- [Release checklist](06-testing/release-checklist.md)

## 07 Deployment

- [Infrastructure inventory](07-deployment/infrastructure.md)
- [Deployment guide](07-deployment/deployment-guide.md)
- [Production release process](07-deployment/production-release-process.md)
- [Rollback plan](07-deployment/rollback-plan.md)
- [Backup and recovery](07-deployment/backup-recovery.md)
- [Monitoring](07-deployment/monitoring.md)

## 08 Agent memory

- [Project context](08-agent-memory/project-context.md)
- [Architecture index](08-agent-memory/architecture-index.md)
- [Session handoff](08-agent-memory/session-handoff.md)
- [Lessons learned](08-agent-memory/lessons-learned.md)

## Project skills

- [project-context](../.agents/skills/project-context/SKILL.md)
- [architecture-review](../.agents/skills/architecture-review/SKILL.md)
- [production-readiness](../.agents/skills/production-readiness/SKILL.md)
- [deploy-production](../.agents/skills/deploy-production/SKILL.md)
- [incident-response](../.agents/skills/incident-response/SKILL.md)

## Lifecycle navigation and authority

Use the development lifecycle as the main stage/entry/exit reference, quality gates for evidence, the task template for records in implementation plans and the production release process for staging-to-production control. ARCH-01 is complete under H1; PLAN-01 covers stage 04 roadmap documentation. H2-H4 cover detailed plan/mode, integration and exact production release respectively and remain ungranted. Significant decisions use ADRs; status and handoffs retain actual failures and verification limits.

## Discovery and maintenance

The requested project skill location is supported by [official Codex documentation](https://learn.chatgpt.com/docs/build-skills), checked on 2026-10-24. Live loading in this session is not verified. Other agents: TODO verify their discovery mechanisms. TODO: assign documentation owners and review cadence after further work is approved.

## Related documentation

- [Project README](../README.md)
- [Agent instructions](../AGENTS.md)
- [Setup evidence](05-progress/current-status.md)
