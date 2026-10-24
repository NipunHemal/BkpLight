# Blockers and limitations

**Document status:** M01 execution authorized in subagent mode; blocked on entry prerequisites, 2026-10-25.

## Toolchain blocker resolved - 2026-10-25

The user authorized WSL tool installation. Go 1.26.10, GCC 13.3.0 and Make 4.3 are installed and non-root compiler/race-detector smoke checks pass. Native Linux `/tmp` permission/flock/socket checks pass; the managed Windows-backed checkout fails restrictive permission checks (0777 remains after chmod). See [installation evidence](../06-testing/m01-workspace-preflight.md#wsl-tool-installation---2026-10-25). Earlier missing-tool statements below are superseded.

M01/T01 remain blocked pending the native Linux test arrangement and explicit M01 fixture-only disposition; H2 plan/subagent approval is retained. No application source/tests were created or run. Do not reinstall these tools or ask for plan/mode approval again.

## Latest execution attempt - 2026-10-25

H2 for M01 plan v0.1 and subagent-driven execution is granted by the latest explicit human request; earlier pending-H2/setup-only statements below are superseded. **M01/T01 are blocked before implementation**, with T02-T09 backlog. The isolated workspace remains `C:/Users/USER/.codex/worktrees/m01-secure-foundation/BkpLight`, branch `feature/m01-secure-foundation`, inspected base `563fcf0d71c89fe7692860b55a8bcb150774ae98`.

Fresh Ubuntu checks still find no Go or C compiler on PATH; Windows Go remains 1.24.5, and the Windows-mounted execution filesystem is unqualified. A read-only subagent reviewed prerequisites; no implementer was dispatched. No module, application files or test results were created. The required full pre-task conflict scan remains pending until the entry gate is resolved.

Next: authorize/provide Linux Go 1.26.10 and a C compiler, qualify a native Linux fixture execution location, and explicitly record the M01-only fixture disposition while M00 production inventory stays incomplete. Do not re-ask for plan/mode approval or waive tests. See the [authoritative plan execution record](../03-planning/implementation-plans/2026-10-24-m01-secure-service-foundation.md#execution-authorization-and-blocked-entry---2026-10-25). No merge, push or deployment authorization.

## Purpose

Separate authorization boundaries and missing evidence from technical failures.

## Authorization boundary

Architecture v0.1/P01-P09 is approved under H1; roadmap documentation is authorized. Implementation needs a scoped detailed plan/H2, integration H3 and production H4. These are workflow boundaries, not application defects. Actual engine versions, host resources and provider accounts remain prerequisites for dependent implementation tasks; see open questions.

## Observed limitations

- Git is now initialized on main with the documentation baseline; the earlier missing-repository limitation is resolved. No remote/hosting-side branch protection is configured. Worktree mode and product H2/merge H3/deployment H4 remain outstanding.
- Local Python has no PyYAML. Do not install packages. Validate the simple skill frontmatter with standard-library checks for the restricted YAML subset used here.
- Static checks do not establish live skill discovery or other-agent compatibility.

## Blocker register

The dependency map lists D01-D09 with affected milestones and required resolution evidence. No application failure is reported. TODO: actual named owners and resolved inventory; missing H2 and exact task plans prevent execution, not roadmap completion.

## Related documentation

- [Git setup and commit policy](../03-planning/git-workflow.md)

- [Prerequisite register D01-D09](../03-planning/task-dependencies.md)

- [Current status](current-status.md)
- [Open questions](../01-requirements/open-questions.md)
