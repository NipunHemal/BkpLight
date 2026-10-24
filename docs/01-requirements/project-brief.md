# Project brief

**Document status:** Discovery draft - confirmed inputs recorded; PRD clarification in progress.

## Purpose

Capture user-stated intent and boundaries for the authorized requirements phase; distinguish confirmed inputs from unresolved choices.

## Project identity

Working label: BkpLight. A Docker-hosted service for scheduled backups of multiple PostgreSQL and MongoDB databases on the user's own server, using private Docker networks and YAML configuration.

## Problem and outcomes

The operator needs configurable remote storage, backup history/retention, restore verification and a restore process, optional custom/Discord webhooks and local log files. User-defined schedules are in scope; guaranteeing or managing a maximum data-loss duration is outside V1. TODO: operational scale and remaining measurable acceptance targets.

## Scope and exclusions

Confirmed: one server; daily/weekly and other configurable schedules; applications remain online during backup; current MongoDB is standalone, with live-backup consistency limits explicitly disclosed; temporary local backup archives with a configurable disk limit are allowed for upload retries. No topology change is authorized. See the PRD for confirmed requirement IDs and proposals. TODO: finalize first-release storage, restore, verification, resource and security policies.

## Stakeholders and approval

The user authorized discovery/PRD documentation on 2026-10-24 with: "Use Superpowers brainstorming to analyze my project requirements ... create a complete PRD in docs/01-requirements/. Do not implement code." H0 is granted for this scope. Final PRD/design, implementation and deployment approvals remain outstanding. TODO: named operational owner and approver details if needed beyond the user.

## Related documentation

- [Working PRD](PRD.md)
- [Functional requirements](functional-requirements.md)
- [Open questions](open-questions.md)
