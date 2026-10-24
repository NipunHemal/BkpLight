---
name: "architecture-review"
description: "Use when reviewing BkpLight changes for architecture, interface contracts, security boundaries, or maintainability compliance."
---

# Architecture review

**Document status:** Setup skill; no architecture baseline is approved yet.

## Purpose

Assess changes against verified, approved constraints and report actionable findings.

## Procedure

1. Read [AGENTS.md](../../../AGENTS.md), the [architecture index](../../../docs/08-agent-memory/architecture-index.md), applicable approved requirements and accepted ADRs.
2. Establish the actual review scope and inspect changed files plus relevant callers, tests and contracts. Use read-only Git inspection if Git exists; otherwise describe the filesystem scope.
3. Compare responsibilities, persistence, compatibility, failure handling, security and maintainability with the approved baseline. Do not treat an earlier conversational design or a TODO template as authoritative.
4. For each finding, state severity, location, violated constraint, concrete consequence and suggested correction. Mark missing evidence separately from confirmed defects.
5. Report compliance as not assessed when a baseline or implementation is absent. Proposals may be offered as proposals, never accepted decisions.

## Output contract

Provide findings first, then assumptions, missing baselines and verification limits. Refer material architecture deviations to an ADR. Do not implement fixes or create requirements unless the user authorized that work.

## Lifecycle design and change gates

Use stages 02-03 for an authorized proposal review and stage 10 for change compliance. In proposal mode, review the candidate against approved discovery requirements, alternatives, contracts, security and operational constraints; distinguish coherence findings from compliance with an accepted architecture that does not yet exist. In compliance mode, compare the change to the accepted baseline.

Record reviewed document revision, blocking findings, dispositions and proposed significant ADRs. Only a human can grant H1 for the concrete design/scope. A review recommendation cannot accept an ADR or start implementation. For a material deviation or failed-task design assumption, return to design review, renew H1, revise dependencies/plan and obtain H2 before resuming. Routine implementation decisions within the approved plan need no new design gate.

## Example and common mistake

With only architecture templates and no code, report that compliance cannot be established; do not declare the project compliant or invent a design. TODO: accepted baseline references when available.

## Related documentation

- [System overview](../../../docs/02-architecture/system-overview.md)
- [API and interface design](../../../docs/02-architecture/api-design.md)
- [Security design](../../../docs/02-architecture/security-design.md)
- [Decision register](../../../docs/04-decisions/README.md)
- [Development lifecycle](../../../docs/03-planning/development-lifecycle.md)
- [Quality gates](../../../docs/06-testing/quality-gates.md)
