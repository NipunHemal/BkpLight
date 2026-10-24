# Project context

**Document status:** Documentation-only project with local Git on main; H1 held, application execution unapproved.

## Purpose

Provide a small verified context entry point for future agents.

## Verified facts

- Working project label/directory: BkpLight, `D:\Project\BkpLight`.
- Root was empty when inspected on 2026-10-24.
- Original authorization covered documentation/skill setup. The user's later request explicitly authorized requirements discovery and a complete PRD using Superpowers brainstorming, with no code implementation; H0 is now granted for that scope.
- No source, tests, manifests, Docker/CI files or Git repository existed at original inspection. Git was subsequently initialized on main at the user's explicit request; initial commit 5b84b7b0d5250cf8c488ccaf5d53d27186a1dbe1 preserves the documentation and ignore rules. No remote or linked worktree was added.
- The subsequent user request authorized architecture design, alternatives and a human approval request. ARCH-01 produced six architecture documents and ADR-001, subsequently approved under H1. The current task PLAN-01 produces the milestone roadmap with no implementation permission.

## Authority and uncertainty

The PRD records original user requirements and C01-C05. The later explicit approval accepts architecture v0.1/P01-P09, including Go/SQLite, R2/S3/Drive, CLI/local IPC, encrypted staging, protected retention and weekly/new-target restore policies. See ADR-001 for human evidence. These are accepted design choices, not installed software. M00 tracks missing deployment inventory and acceptance synchronization; H2/H3/H4 remain outstanding.

## Refresh

Read current status and inspect files before reusing notes. Distinguish observed facts from proposals and old snapshots from fresh verification.

## Ownership

TODO: named planning/execution owners and future H2/H3/H4 records. These repository notes are separate from global assistant memory; do not modify global memory without user instruction.

## Related documentation

- [Task-wise Git workflow](../03-planning/git-workflow.md)

- [Accepted ADR and H1 evidence](../04-decisions/ADR-001-single-server-backup-architecture.md)
- [Milestone roadmap](../03-planning/roadmap.md)

- [Working PRD and clarification decisions](../01-requirements/PRD.md)
- [Agent instructions](../../AGENTS.md)
- [Current status](../05-progress/current-status.md)
- [Architecture index](architecture-index.md)
