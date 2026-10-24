---
name: "project-context"
description: "Use when starting or resuming work in BkpLight, answering project-context questions, or resolving conflicting project notes."
---

# Project context

**Document status:** Setup skill; project details remain evidence-dependent.

## Purpose

Load and summarize verified context without promoting proposals into approved decisions.

## Procedure

1. Read [AGENTS.md](../../../AGENTS.md), [current status](../../../docs/05-progress/current-status.md) and [handoff](../../../docs/08-agent-memory/session-handoff.md).
2. Inspect the current repository inventory. Load only documents and source needed for the user's authorized task.
3. Classify findings as observed implementation, explicit user instruction, approved decision, proposal or unknown. A template is not an approved specification.
4. Check dates and evidence references before reusing notes. Report contradictions and use TODO for missing information.
5. Respect the active phase. During setup, do not begin requirements analysis, select architecture, create code, install tools or deploy.
6. Read the lifecycle and identify the active stage, H0-H4 approval scope, authoritative task record, workspace and last verified candidate. Distinguish task status from integration/release state. Refresh stale evidence before resuming; a failed task or changed design may require recovery/replanning rather than direct execution.

## Output contract

Summarize verified facts with file/evidence references, proposals separately, missing evidence and the next action permitted by current authorization. Do not fill unrelated templates simply to complete context loading.

## Example and common mistake

If conversation proposed Go/SQLite but no decision was accepted, report the stack as proposed, not selected. TODO: link accepted decisions when approval actually exists.

## Related documentation

- [Documentation index](../../../docs/README.md)
- [Project context](../../../docs/08-agent-memory/project-context.md)
- [Architecture index](../../../docs/08-agent-memory/architecture-index.md)
- [Development lifecycle](../../../docs/03-planning/development-lifecycle.md)
- [Task template](../../../docs/03-planning/task-template.md)
