# Blockers and limitations

**Document status:** Active inventory and execution prerequisites; H1 held, H2/H3/H4 outstanding.

## Purpose

Separate authorization boundaries and missing evidence from technical failures.

## Authorization boundary

Architecture v0.1/P01-P09 is approved under H1; roadmap documentation is authorized. Implementation needs a scoped detailed plan/H2, integration H3 and production H4. These are workflow boundaries, not application defects. Actual engine versions, host resources and provider accounts remain prerequisites for dependent implementation tasks; see open questions.

## Observed limitations

- No Git repository exists; Git diff/status validation is unavailable. Compare filesystem inventories instead.
- Local Python has no PyYAML. Do not install packages. Validate the simple skill frontmatter with standard-library checks for the restricted YAML subset used here.
- Static checks do not establish live skill discovery or other-agent compatibility.

## Blocker register

The dependency map lists D01-D09 with affected milestones and required resolution evidence. No application failure is reported. TODO: actual named owners and resolved inventory; missing H2 and exact task plans prevent execution, not roadmap completion.

## Related documentation

- [Prerequisite register D01-D09](../03-planning/task-dependencies.md)

- [Current status](current-status.md)
- [Open questions](../01-requirements/open-questions.md)
