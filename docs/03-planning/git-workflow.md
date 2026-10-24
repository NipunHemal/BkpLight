# Git and task-wise commit workflow

**Document status:** Active workflow policy, 2026-10-24; Git setup authorized, application execution and merges unapproved.

## Purpose

Preserve user work, keep approved tasks reviewable through scoped commits and connect Git evidence to the existing Superpowers lifecycle. A commit is a recorded change, not proof of acceptance, integration or production readiness.

## Authority and initial baseline

The user explicitly requested Git initialization, `main` for a new repository, a stack-specific ignore file, a safe initial commit and task-wise commit integration. This overrides the earlier setup prohibition on those exact Git operations. It does not approve application features, package installation, merging, pushing or deployment.

Inspection found no existing repository, branches or worktrees. `git init -b main` created the local repository. Existing configured author identity was reused without changing global configuration. Initial commit `5b84b7b0d5250cf8c488ccaf5d53d27186a1dbe1` records the 45 preexisting Markdown files and the new Go/Docker/SQLite-aware `.gitignore`. No application source or generated data was added. Historical documents in that commit describe the pre-Git state; later current-status records supersede those statements.

## Branch strategy

| Branch | Purpose | Base / integration boundary |
|---|---|---|
| `main` | Stable, reviewed project baseline; eventually verified code | No feature development directly here; integrate only with explicit human approval |
| `feature/<task-or-feature>-<short-name>` | Approved feature or planned workstream | Approved stable base, normally `main`; scoped H2 before implementation |
| `fix/<task>-<short-name>` | Reviewed bug correction | Approved affected baseline; reproduction and regression evidence |
| `hotfix/<task>-<short-name>` | Urgent production correction | Exact released revision or approved maintenance base; urgency does not waive review, tests, merge or deployment approval |

The requested initial repository/workflow setup is a one-time direct-to-main documentation bootstrap, not a general permission to code on main. Do not create placeholder feature/fix/hotfix branches until work needs them. Documentation-only follow-ups can use a feature branch with `docs` commits. No remote is configured during setup; hosting-side protected branches, required checks and review rules are therefore not enforced. TODO: choose remote/provider and configure protections when explicitly requested. Local policy is not a technical branch-protection claim.

## Inspection and worktree setup

Before modifying Git state, inspect repository root, branch, staged/unstaged/untracked files, existing worktrees and remotes. A parent repository or linked worktree must not be reinitialized. Preserve unrelated user changes, including previously staged files. Do not stash, discard, reset or clean them automatically. Stop before a commit if the index contains unrelated work; isolate the task or obtain a scoped decision without overwriting that work.

Use installed `superpowers:using-git-worktrees` for authorized implementation where isolation is appropriate. Reuse suitable existing isolation and honor the approved execution plan's preference. Prefer native managed-worktree tools; fall back to Git only when native tooling is unavailable. Record the actual path, branch/ref and base commit. Before a project-local worktree, verify `.worktrees/` is ignored. Worktree creation must not trigger unauthorized dependency installs or feature work. Run the applicable baseline checks; absent source/tests are not passing application tests.

No linked worktree is necessary for this initial repository setup. Future tasks use the H2-approved workspace and mode. Worktree cleanup must preserve unmerged/unpushed/user work; never treat task completion as permission to delete a checkout or branch.

## Per-task loop

For every approved task, perform the following in order:

1. Read requirements, accepted architecture/ADRs, detailed plan, acceptance IDs and relevant skills. Confirm the task is ready and covered by scope/approval; inspect Git state and mark it `in_progress`.
2. Use `superpowers:test-driven-development` for meaningful behavior changes: observe the failing test, implement the minimum change, then pass and refactor. Documentation-only tasks use relevant document/config checks instead; do not fabricate a TDD result for prose.
3. Run the plan's relevant tests/build/static and integration checks, recording exact commands, candidate and results. Debug with `superpowers:systematic-debugging`; failing required checks prevent normal task completion commits.
4. Review all task changes for specification compliance, quality, security and scope. Use the applicable review skills; technically verify received feedback. Fix findings and repeat affected checks before staging.
5. Stage exact task-owned paths or reviewed hunks. Do not use blanket `git add .`, `git add -A` or `git commit -a` in a shared/dirty checkout. Ensure no concurrent worker is mutating those files during review/staging.
6. Review `git diff --cached --name-status`, `git diff --cached --stat`, the full `git diff --cached`, and `git diff --cached --check`. Verify deletions and generated/binary files deliberately. Inspect the exact staged bytes for secrets, not only the working copy. If stage and tested content differ, rerun the affected checks or restage/review.
7. Commit only the coherent reviewed task using the format below. Include its task/milestone reference in the commit body. A request to work through an approved plan with this workflow permits its routine local task commits; it does not approve a merge or push.
8. Read the resulting hash with `git rev-parse HEAD`; inspect `git show --stat --oneline HEAD` and `git status --short`. Record hash, task status, evidence and review outcome. A commit command error is not a successful commit; inspect state before retrying.
9. Continue to the next task already covered by the approved plan when its dependencies are satisfied. Do not ask for permission between routine approved tasks, and do not infer approval for a new scope or phase.

Default to one logical commit per verified task. If a task needs more than one, keep each reviewable and record the ordered hashes; include code, its tests and necessary documentation together. Do not split a deliberately failing red test into a completed task commit. A checkpoint for incomplete work must be explicitly scoped and clearly labeled; it never marks a task done or bypasses verification.

## Conventional Commits

Use `type(scope): imperative description`, with a concise lowercase scope naming the affected subsystem. Allowed types:

| Type | Use | Example |
|---|---|---|
| `feat` | New behavior | `feat(storage): add resumable uploads` |
| `fix` | Correct behavior | `fix(retention): preserve active restore claims` |
| `docs` | Documentation | `docs(workflow): define task-wise commit lifecycle` |
| `refactor` | Structure without behavior change | `refactor(state): isolate copy transitions` |
| `test` | Tests/fixtures | `test(restore): cover interrupted verification` |
| `chore` | Tooling/repository maintenance | `chore(repo): initialize documentation baseline and ignore rules` |
| `ci` | Automation configuration | `ci(test): run database adapter checks` |

Commit bodies identify task/milestone and validation actually performed. Never claim a passing suite that did not run. Approved breaking changes use Conventional Commits' `!`/`BREAKING CHANGE:` convention and link their accepted design change. Do not insert secrets, provider session URLs or private data into messages.

## Task records and hashes

The task record in its scoped implementation plan is authoritative. Record task ID, branch/worktree, base revision, implementation commit(s), evidence, reviewer/outcome, status and separate integration/release state. Current-status and session-handoff link to it.

A commit cannot contain its own final hash. Record the task commit's actual hash after it exists, then persist that record in the next scoped documentation/bookkeeping commit; do not amend recursively to chase a self-referential hash. The final bookkeeping commit hash belongs in the final user report or next session's ledger. Commit subjects and task references make these bookkeeping commits discoverable. Preserve failed-attempt history; do not rewrite completed task commits to conceal errors.

For this bootstrap, GIT-SETUP-01 is the initial commit above. GIT-SETUP-02 adds this workflow and updated navigation/status records; its hash is reported after the commit. These are setup tasks, not M00-M09 implementation completion.

## Secrets and generated data

The ignore file excludes local environment files, credentials/keys, runtime SQLite state/WAL, database dumps/encrypted archives, staging/logs, local Compose overrides, Go binaries/test output and local worktrees. Keep `go.mod`, `go.sum`, SQL migrations, source/tests, sanitized config examples, Docker/CI definitions and project skills versioned. Do not blanket-ignore all SQL, YAML or JSON.

Ignore rules do not remove already tracked secrets and do not inspect file contents. Inspect actual staged blobs for credentials/private keys/tokens, authenticated connection URLs and secret-bearing webhooks. Automated pattern scans support manual review but cannot prove that no arbitrary secret exists. Check even files named `.env.example`; placeholders must be synthetic. If legitimate synthetic fixtures conflict with ignores, review and document a narrow allow rule rather than force-adding real artifacts.

If a candidate secret is detected, stop the commit, identify the path and rule without printing the value, and preserve user content. Resolve the staged selection/redaction within authority. If a credential has already been committed, stop publishing, report exposure and coordinate revocation/rotation; history rewriting is not automatically authorized. Never force-push, use `--no-verify` to bypass checks, or override security restrictions to get a green result.

## Milestone review and integration

At the milestone boundary, use `superpowers:requesting-code-review` (and `receiving-code-review` for feedback), then `verification-before-completion` on the final candidate. Review all milestone commits, not only the last diff. Confirm acceptance evidence, clean/known workspace state, tests, documentation and commit/task traceability. Invalidate affected evidence after review fixes or conflict resolution.

Use `superpowers:finishing-a-development-branch` to prepare integration choices. Present the reviewed source/base, target branch, commit range, verification and unresolved limits. Do not merge without explicit human approval for the action/destination, even if the task or milestone is marked done. Never force-push. A normal push/PR also needs applicable user authorization and a configured remote. Production still requires H4, staging, backup/restore, monitoring and rehearsed rollback. An urgent hotfix has the same gates.

If a merge conflicts, preserve both sides and stop automatic integration; resolve only within the approved integration scope, review the resolution and rerun affected checks. A failed merge is not permission to reset unrelated work. Do not delete branches/worktrees automatically when they contain needed work.

## Verification and open configuration

For setup, verify ignored secret/runtime examples and tracked source/template examples, documentation links, staged whitespace/sensitive-content review, commit contents, branch and final status. No application tests exist yet. TODO: select project-specific test commands and secret-scanning/CI tooling in an authorized implementation plan; no hook or CI enforcement was installed by this documentation setup.

## Related documentation

- [Agent instructions](../../AGENTS.md)
- [Development lifecycle](development-lifecycle.md)
- [Task template](task-template.md)
- [Implementation plan policy](implementation-plans/README.md)
- [Quality gates](../06-testing/quality-gates.md)
- [Current status](../05-progress/current-status.md)
- [Session handoff](../08-agent-memory/session-handoff.md)
