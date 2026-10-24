# Implementation plans

**Document status:** M01 detailed plan v0.1 prepared for review; no H2 execution approval.

## Purpose

Define the location and evidence for future approved execution plans.

## Current contents

- [M01 secure service foundation](2026-10-24-m01-secure-service-foundation.md): nine tasks with exact files/contracts, twelve acceptance criteria, test cases and Linux verification commands. Status: H2 review; M00 disposition/tooling and execution mode remain prerequisites. No code has been implemented.

The milestone roadmap, milestone exit checks and dependency map live one level above this directory. This README and the plan are not execution grants.

## Planned decomposition

Prepare a separate plan for each milestone or independently testable feature slice in the roadmap. Start with M00 prerequisites and the M01 foundation plan; prepare dependent plans when their consumed interfaces and factual prerequisites are known. Do not produce one speculative code-level plan spanning all providers, recovery and deployment.

Each future plan uses the writing-plans header (Goal, Architecture, Tech Stack, Spec, Global Constraints), the task template and concrete failing/passing tests. Name exact files and interface types; record scoped H2 and mode before execution. TODOs may record missing inventory in the roadmap but must not stand in for executable task steps. Proposed source boundaries are in task-dependencies.md; no source directories exist yet.

## Naming and authority

Use `YYYY-MM-DD-short-topic.md` after design approval. Record approved scope, design/ADR references, owner and status. Configure planning skills to use this location instead of creating competing plan trees.

## Plan template

TODO: objective, acceptance IDs, scoped files, dependencies, task steps, meaningful tests, review checkpoints, recovery and completion evidence.

## Execution

Use applicable installed Superpowers workflows only after the plan and task are authorized.

Use the installed writing-plans structure, supplemented with the task-template fields. Record H1 design and H2 plan/mode approval references, task IDs and status in each plan. Task records in the plan are authoritative; current status and handoffs link to them rather than maintaining conflicting copies. Preserve red/green, review, final verification and failed-attempt evidence. Follow git-workflow.md for scoped Conventional Commits, staged secret review and actual hash/status recording after every approved task. Significant scope changes reopen the affected lifecycle gates; a plan file alone grants no execution authority.

## Related documentation

- [Task-wise Git workflow](../git-workflow.md)

- [Development lifecycle](../development-lifecycle.md)
- [Task record template](../task-template.md)
- [Quality gates](../../06-testing/quality-gates.md)
- [Roadmap](../roadmap.md)
- [Task dependencies](../task-dependencies.md)
- [Agent instructions](../../../AGENTS.md)
