# Software development lifecycle

**Document status:** Active workflow policy for future authorized work; product execution is not authorized by this setup.

## Purpose

Define repeatable stages, evidence, human approvals and recovery without choosing a product architecture or implementing features.

## Authority and skill ownership

This policy originally accompanied workflow setup. Subsequent requests granted discovery, H1 for architecture v0.1/P01-P09, roadmap documentation and now Git initialization/setup commits with task-wise commit integration. Product H2, merge H3 and deployment H4 remain outstanding. See ADR-001 for design approval and git-workflow.md for the Git setup boundary; this policy itself does not authorize application implementation.

Use official installed `superpowers:*` skills directly. The custom skills are `architecture-review`, `production-readiness`, `deploy-production` and `incident-response`; `project-context` supports every stage. Read the actual applicable skill before use. Disclose unavailable skills and do not claim they executed. This project uses `docs/02-architecture/` for design documents and `docs/03-planning/implementation-plans/` for plans instead of parallel default Superpowers directories. Keep their required document contents and approval semantics.

## Human approval gates

An approval record identifies gate, human approver, date, scope, document revision or artifact, target where relevant, conditions and a reference to the actual instruction. A checkbox, agent review or agent-authored status is not approval. Existing explicit approval remains valid within its recorded scope; do not repeatedly request it for routine tasks.

| Gate | Required human decision | Timing and renewal |
|---|---|---|
| H0 | Authorize product discovery and its scope | Before stage 01; workflow setup alone does not satisfy it |
| H1 | Approve the concrete design and scope, including significant ADRs | Stage 03, after architecture review; revisit for material requirement, contract, security or architecture changes |
| H2 | Approve the implementation plan and execution mode | End of stage 05, before worktree setup/execution; renewed for material replanning, not every routine task |
| H3 | Select/authorize integration action and destination | Stage 12; existing task authorization may cover the exact action, otherwise obtain it; destructive discard requires explicit confirmation |
| H4 | Explicitly approve the exact production release, target, procedure and rollback scope | After stage 13 evidence and before stage 14 mutation; changed artifact/target or invalidated evidence requires renewed approval |

Any additional destructive data recovery, privileged access or incident containment outside recorded authorization requires its own explicit human decision. An emergency does not grant new permissions. Prepare reviewable work before seeking the final relevant approval.

## Stage matrix: entry, exit and owning documents

The numbers describe the overall lifecycle. TDD is nested inside implementation; debugging and review feedback loop back rather than waiting for a later phase.

| Stage | Skill(s) | Entry criteria | Exit criteria and evidence | Documentation owner |
|---|---|---|---|---|
| 01 Discovery | `superpowers:brainstorming` | H0 recorded; current context inspected | Problem, users, scope, constraints, acceptance candidates and unresolved questions documented; assumptions labeled | `01-requirements/`: brief, functional/non-functional requirements, acceptance criteria, open questions |
| 02 Architecture | `superpowers:brainstorming` + custom `architecture-review` | Discovery inputs available; material unknowns visible | Concrete design, alternatives, contracts, data/security/infrastructure model and proposed significant ADRs reviewed; unresolved findings recorded | `02-architecture/` and `04-decisions/` |
| 03 Design approval | custom `architecture-review` + human approval | Reviewable versioned design and findings | H1 recorded for exact design/scope; blocking findings resolved; accepted ADRs linked; no implicit acceptance of templates | `02-architecture/`, `04-decisions/`, approval references in `05-progress/` |
| 04 Roadmap | Planning through existing documents | H1; acceptance scope known | Milestones, dependencies, order, owners, entry/exit criteria and risks mapped to approved scope | `03-planning/`: roadmap, milestones, task dependencies |
| 05 Task planning | `superpowers:writing-plans` | Approved design and dependency map | Actionable plan, task IDs, exact scoped files/interfaces, test commands and expected results, recovery and execution mode; H2 recorded | `03-planning/implementation-plans/`, task template, `05-progress/` |
| 06 Worktree setup | `superpowers:using-git-worktrees` | H2; Git state and unrelated changes inspected | Suitable isolated checkout reused/created, paths and branch recorded, relevant baseline checks run; failures disclosed and approved before proceeding | Task record and `08-agent-memory/session-handoff.md` |
| 07 Implementation | `superpowers:subagent-driven-development` OR `superpowers:executing-plans` | Ready task, approved mode, prerequisites and workspace available | Scoped changes complete through stages 08-11; reviewed task-wise commit and hash/evidence recorded; no completion from implementer assertion alone | Task plan, `05-progress/`, affected design/contracts |
| 08 Per-task TDD | `superpowers:test-driven-development` | Behavior/task and acceptance IDs identified | Meaningful failing behavior test observed, minimal implementation passes, refactor and required regression suite pass; evidence saved | Task evidence and `06-testing/` |
| 09 Debugging | `superpowers:systematic-debugging` when needed | Failure, unexpected behavior or failed baseline | Reproduction, cause, tested correction and regression evidence; or explicit blocked/failed status with next hypothesis | Task failure log, `05-progress/blockers.md` |
| 10 Review | `superpowers:requesting-code-review` + `superpowers:receiving-code-review` | Reviewable diff and test evidence; design/plan references supplied | Spec compliance and code quality assessed; feedback verified and addressed or explicitly dispositioned; blocking findings closed | Task review record, affected docs, ADRs for significant deviations |
| 11 Verification | `superpowers:verification-before-completion` | Current candidate and review outcomes available | Fresh required checks for that candidate with actual outputs; acceptance traceability, unresolved limitations and doc/handoff updates | `06-testing/`, task record, `05-progress/` |
| 12 Integration | `superpowers:finishing-a-development-branch` | Stages 10-11 pass; H3 covers chosen action | Selected merge/PR/keep/discard action recorded; integrated result verified when integrated; failing merge result stops progress; retained work remains recoverable | `05-progress/`, task integration field, session handoff |
| 13 Release readiness | custom `production-readiness` | Integrated candidate identified for release | Quality gates pass; same candidate validated in staging; backup/restore, rollback, migration safety and monitoring evidence current; concrete production plan ready for H4 | `06-testing/quality-gates.md`, `07-deployment/production-release-process.md` |
| 14 Deployment | custom `deploy-production` | H4 plus stage 13 evidence; preflight unchanged | Approved actions only; health/observation evidence and final release state recorded, or approved rollback/incident handoff | `07-deployment/`, release record and session handoff |
| 15 Maintenance | custom `incident-response`; debugging/TDD/review skills for corrective work | Released service or operational event with authorized response scope | Incident triaged, actions/evidence recorded, recovery verified, follow-ups created; corrective changes re-enter relevant lifecycle gates | Monitoring/recovery runbooks, `05-progress/`, `08-agent-memory/lessons-learned.md` |

Paths in the matrix are relative to `docs/`. Stage exit criteria are required where applicable; not-applicable decisions need evidence and rationale. Missing staging, backup, rollback or monitoring evidence blocks production and cannot be waived by an agent. No technology or runtime commands are selected here.

## Repeatable per-feature loop

1. Load context; select approved requirement/acceptance IDs and the current design/ADR baseline.
2. If scope/design changes, return to stages 01-03 and H1. Otherwise use existing approval.
3. Update milestones/dependencies; create a scoped plan and task records using the task template. Obtain H2 for plan and mode.
4. Establish the worktree/baseline. Choose subagent or inline execution according to H2 and available tools; do not dispatch conflicting work on shared files.
5. For each ready task: `in_progress` -> red test -> minimal code -> green/refactor -> review -> verification. Debug failures immediately; re-run affected checks after changes. In subagent mode, use the installed skill's task reviewer and final whole-branch review; preserve inline mode's own review requirements.
6. Review the working diff and exact task-owned staged diff, check secrets/whitespace and commit using Conventional Commits. Record actual commit hash, tests, review, acceptance evidence and task status. Persist post-commit hashes through the bookkeeping rule in git-workflow.md. Continue within the approved plan without asking for approval between routine tasks; never include unrelated staged user changes.
7. Complete whole-feature verification, H3 integration and candidate verification. A task marked done does not mean integrated, released or production ready.
8. For a release, satisfy stages 13-14 separately. Maintain the service using stage 15 and feed corrective tasks back into this loop.

## Task status and session continuity

The [Git workflow](git-workflow.md) defines stable main, feature/fix/hotfix branches, scoped staging, Conventional Commits and worktree use. At milestone completion apply requesting-code-review, verification-before-completion and finishing-a-development-branch to prepare integration; merge only after explicit human approval. No force-push or automatic deployment. Task hashes and verified state are tracked separately from integration/release state.

Use `backlog`, `ready`, `in_progress`, `in_review`, `verified`, `done`, `blocked`, `failed` or `cancelled`.

- `backlog` -> `ready`: plan/H2 covers scope, dependencies are satisfied and test/recovery instructions are actionable.
- `ready` -> `in_progress` -> `in_review` -> `verified` -> `done`: evidence passes at each boundary; `done` means the task deliverable is accepted within the plan.
- Any active task -> `blocked` for an unmet prerequisite; -> `failed` for an unsuccessful attempt. Record cause and last safe state.
- Resume via `ready` only after the dependency is resolved or a recovery/replan is reviewed within existing approval. A code-changing review correction returns to `in_progress` and invalidates affected verification.
- `cancelled` requires a scope decision and preservation/disposition of partial work; do not delete work as a status change.

The authoritative task record is its checklist/status entry in the approved implementation plan using the task-template fields; avoid competing task ledgers. Current status summarizes tasks and links to those records. Track integration and release states separately. Update status on transitions and the handoff before ending or transferring a session, including failed commands and dirty workspace state.

## Failed-task recovery and replanning

Stop the affected task and dependent work; preserve logs and scoped changes. Record command, environment/revision, actual result, impact and last safe state. Diagnose with systematic-debugging; test one evidence-backed hypothesis at a time. Do not hide baseline failures, disable tests or rerun indefinitely to obtain a green result. Define attempt/time budgets in the task; on exhaustion, mark blocked and present evidence and recovery choices.

For a local implementation defect within approved scope, return through TDD/review/verification. For a wrong design assumption or changed contract, revise the design and proposed ADR, obtain H1, revise impacted tasks/dependencies, obtain H2 and re-establish baselines. Never erase the original failed attempt or rewrite an accepted ADR as if the new decision had always applied. Rollback/reset/delete actions need appropriate authorization and must preserve unrelated changes.

## Significant decisions and completion

Use ADRs for consequential alternatives involving architecture, contracts, persistence, security, deployment or recovery. Link requirement/task, rationale, alternatives, consequences, approver and supersession. Do not create an ADR for every implementation detail or treat a proposed ADR as accepted.

Before saying complete, verify the current candidate at the required level, close blocking findings, link actual evidence and update status/handoff. Checks are `pass`, `fail`, `blocked`, `not_run` or `not_applicable`; only evidence-backed passes satisfy gates. TODO: product-specific commands, targets, thresholds and owners after product work is approved.

## Related documentation

- [Agent instructions](../../AGENTS.md)
- [Task template](task-template.md)
- [Git and task-wise commits](git-workflow.md)
- [Quality gates](../06-testing/quality-gates.md)
- [Production release process](../07-deployment/production-release-process.md)
- [Session handoff](../08-agent-memory/session-handoff.md)
- [ADR register](../04-decisions/README.md)
