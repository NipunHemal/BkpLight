# Release checklist

**Document status:** Template - not approved.

## Purpose

Define future release evidence without implying a release exists.

## Release identity

TODO: version, immutable artifact, target, owner and explicit approval. No release is authorized during setup.

## Evidence checklist

- [ ] Approved requirements, architecture and release scope.
- [ ] Reproducible build and applicable tests with output.
- [ ] Security, secrets and permission review.
- [ ] Migration compatibility and recovery, if applicable.
- [ ] Same immutable candidate validated in isolated staging; environment differences reviewed.
- [ ] Fresh applicable backup coverage, integrity, usable restore dependencies and isolated restore evidence.
- [ ] Health, logging, monitoring and operator ownership; optional webhooks explicitly configured or disabled.
- [ ] Rollback artifact/configuration, triggers, data compatibility, procedure and staging rehearsal evidence.
- [ ] H4 explicit human approval for the exact artifact, target, reviewed procedure and rollback scope before production mutation.

## Exceptions and verdict

TODO: mark each check pass, fail, blocked, not run or not applicable with evidence/rationale. Missing evidence is not a pass.

Staging, backup/recovery, rollback and monitoring are mandatory production gates. An agent cannot mark them not applicable to bypass missing evidence. Readiness alone does not authorize deployment.

## Related documentation

- [Quality gates](quality-gates.md)
- [Production release process](../07-deployment/production-release-process.md)
- [Test strategy](test-strategy.md)
- [Deployment guide](../07-deployment/deployment-guide.md)
- [Rollback plan](../07-deployment/rollback-plan.md)
