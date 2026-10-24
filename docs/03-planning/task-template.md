# Task record template

**Document status:** Reusable workflow template; no product task or execution is approved.

## Purpose

Define the task fields used inside approved implementation plans. Fill actionable details before marking a task ready; do not execute this template.

## Identity and authority

| Field | Value |
|---|---|
| Task / feature / milestone IDs | TODO |
| Status | backlog |
| Owner and reviewer | TODO |
| Approved requirement / acceptance IDs | TODO |
| Design revision and accepted ADRs | TODO |
| Implementation plan and H2 approval evidence | TODO |
| Execution mode | TODO: inline or subagent, as approved |
| Scope and exclusions | TODO |
| Last updated | TODO |

## Dependencies and workspace

TODO: prerequisite task IDs and completion evidence; checkout/worktree absolute path, repository/branch/revision and baseline check results. No Git initialization or worktree creation is authorized by this template.

## Deliverable and interfaces

TODO: exact files to create/modify/test, responsibility, consumed/produced interfaces, contract constraints and measurable acceptance. Preserve unrelated files.

## Execution checklist

- [ ] Confirm scope, approvals, dependencies and baseline.
- [ ] Write meaningful behavior test; run and observe the expected failure.
- [ ] Implement the minimum scoped change.
- [ ] Run green tests, refactor and run required regression checks.
- [ ] Review spec compliance and code quality; verify and resolve feedback.
- [ ] Run fresh final checks; attach acceptance evidence and update affected docs.
- [ ] Update task status, current-status summary and session handoff.

Use the installed writing-plans skill's required plan contents. Replace checklist placeholders with exact task steps, commands and expected outcomes rather than leaving an executable plan vague.

## Evidence register

| Check / acceptance ID | Candidate / environment | Command or procedure | Expected | Actual result | Status | Evidence reference / time |
|---|---|---|---|---|---|---|
| TODO red/green/regression/build/integration as applicable | TODO | TODO | TODO | Not run | not_run | TODO |

Tests, builds, runtime recovery and business acceptance are separate entries. Missing tooling is not a failed application test. Relevant required checks must pass before completion; use evidence-backed rationale for not-applicable items.

## Failure, recovery and replanning

TODO: attempt/time budget, failure symptoms, actual commands, root-cause hypothesis, safe stopping point, recovery choices, dependencies affected and approval implications. Each failed attempt stays in the history. Do not improvise destructive cleanup or skip checks to meet the budget.

| Attempt / time | Result and cause evidence | Preserved state | Next hypothesis / action | Reapproval needed |
|---|---|---|---|---|
| TODO | TODO | TODO | TODO | TODO |

## Review and completion

TODO: review findings and disposition; final candidate; acceptance evidence; remaining risks; completion time. Status values and transitions are defined in the lifecycle. Track `integration_state` and `release_state` independently: a done task is not automatically merged or deployed.

## Related documentation

- [Development lifecycle](development-lifecycle.md)
- [Implementation plans](implementation-plans/README.md)
- [Task dependencies](task-dependencies.md)
- [Quality gates](../06-testing/quality-gates.md)
- [Session handoff](../08-agent-memory/session-handoff.md)
