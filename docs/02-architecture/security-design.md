# Security design

**Document status:** Architecture v0.1 approved under H1 on 2026-10-24; no implementation.

## Approval record

The human approved architecture v0.1 and P01-P09 on 2026-10-24; see [H1 evidence in ADR-001](../04-decisions/ADR-001-single-server-backup-architecture.md). Proposal/default wording below records the approved design rationale, not a pending repeat approval. Unknown deployment facts remain unresolved; H2/H3/H4 are not granted.

## Purpose

Define permissions, trust boundaries, encryption and restore controls for the proposed single-server service. These controls are design requirements, not tested security claims.

## Assets and trust boundaries

The trusted operator controls YAML, CLI access and deployment. Protect database contents, encrypted archives, metadata, credentials, decryption/signing keys and recovery evidence. A compromised host or operator can access online keys and sources; container isolation does not remove that risk.

| Threat | Proposed control | Residual limitation |
|---|---|---|
| Public database or control API exposure | Private Docker networks; Unix-domain control socket; no published database/control ports | Other containers on a shared network can reach peers; database authentication still required |
| Backup credential compromises production | Dedicated read-oriented backup identity; separate restore/verification identities | Database export permissions may expose all selected data |
| Remote archive theft or tampering | Encrypt before persistent staging; verify hashes and signed commit metadata | Trusted signing-key compromise invalidates origin assurance |
| Wrong-target restore or cleanup | Allowlisted targets, preview bound to artifact/target, new database only, ownership tracking | Trusted operator must verify endpoint inventory and cut over applications separately |
| Stolen webhook/OAuth credentials | File references, restricted writable credential store, redaction, no secrets in audit payloads | Host administrators can read mounted credentials |
| Remote destination deletes old backups | Owned-prefix retention, minimum/pin/verification protection, separate deletion permission where possible | Provider administrator or compromised host can still delete; immutable storage is a future option |
| Malicious SQL/archive content | Verify provenance; execute restores only in isolated, restricted targets | PostgreSQL restores can execute source-authored code; signatures do not make source contents safe |

## Identity and secrets

### Minimum privilege by component

| Component | Permissions needed | Explicit boundary |
|---|---|---|
| PostgreSQL backup | CONNECT to selected databases; schema/object visibility and SELECT for exported data, including sequences as required by the tested version | Avoid superuser. Row-level security and extensions require explicit coverage checks; never silently accept an incomplete dump |
| PostgreSQL globals export, opt-in | Access required by tested pg_dumpall globals export | Role/password export is sensitive and may need elevated access; separate identity and explicit opt-in, not a default privilege escalation |
| MongoDB backup | Version-appropriate backup role or tested custom role for selected database data, indexes and metadata | No write/restore role on source; authentication database is explicit; no assumption that standalone supports oplog snapshots |
| Verification database | Create/drop only operation-owned test databases and restore objects on the isolated verification server | Dedicated credentials cannot authenticate to production; no general source-admin credential |
| Operator restore | Create/restore a new database on an approved target; matching extension/role prerequisites provisioned by operator | No in-place overwrite or automatic production cutover in V1 |
| S3/R2 adapter | List configured owned prefix; put/get objects; multipart lifecycle actions; delete only if retention enabled | Dedicated bucket/prefix identity; provider granularity differs and must be tested |
| Drive adapter | Create/read/manage backup files in the selected accessible folder under a tested OAuth model | Do not request broad Drive scope by default; recovery/listing under the selected scope must be demonstrated |
| Runtime identity | Read config and secret files; write designated state/staging/log/credential paths and local socket | Non-root, no Docker socket, no host root mount, no privileged mode |

Secrets in YAML are file references, not literal passwords or webhook URLs. Supply database credentials through native-tool supported secret files or restricted temporary files in tmpfs; prevent credentials appearing in process arguments and captured stderr. If an environment variable is unavoidable for a specific tool, document its process-inspection exposure and redact diagnostics. Never invoke a shell with interpolated credentials, database names or user arguments.

Personal Google Drive OAuth with offline refresh is proposed; Shared Drive/service-account support depends on the actual account and is not assumed. Bootstrap credentials may be read-only; refreshed tokens and sensitive resumable-upload session handles require a separate protected writable credential store, never ordinary SQLite history/log records. TODO: approve actual account model, scopes and rotation procedure before provider implementation.

Version credential references so queued work cannot silently use a newly repointed destination. Credential rotation may resume an existing destination identity; changing bucket, endpoint or folder creates a new destination revision and requires reconciliation.

## Data and networks

### Encryption and recovery keys

Propose age recipient encryption for archives and private manifests before writing persistent staging files. Use vetted libraries/tools, not custom cryptography. Ciphertext SHA-256 detects corruption; a separate Ed25519 signature over the exact commit record binds ciphertext hashes, sizes and artifact identities for offline catalog recovery. The detached signature and trusted public key are checked before trusting a remote commit. Keep the public trust anchor and decryption recovery material independently of the backup destination.

The backup writer needs recipient public keys and a signing private key. Automated verification also needs a decryption key online; this increases host-compromise exposure. Offer manual verification with separately supplied keys when the operator declines online decryption. Never log keys. Record key IDs, preserve old decryption keys for retained backups, and test recovery before rotation/removal. Losing the decryption key makes encrypted backups unusable.

Encryption protects staged/remote archives, not SQLite catalog fields, logs or restored verification data. Protect these with host permissions, restricted volumes and host-volume encryption where required by deployment policy. Define secure disposal operationally; ordinary file deletion is not guaranteed physical erasure on SSDs.

### Network and request controls

Use TLS certificate validation for provider APIs and webhooks. Private Docker networking does not itself encrypt database traffic; support database TLS with CA/hostname verification when configured and disclose plaintext private-network connections. No public control port is proposed.

Validate destination/webhook URLs: HTTPS by default, allowed schemes, explicit endpoints, no URL credentials, bounded responses/timeouts, no automatic cross-host redirects. Deny cloud metadata/link-local/loopback destinations by default. Explicitly configured internal endpoints need a narrow allowlist and the same resolved-address checks; private database endpoints belong to a separate database allowlist. Recheck resolved addresses on connections to reduce DNS-rebinding exposure. Trusted operator configuration is not an excuse to send secrets to redirected hosts.

The Unix socket is restricted to the service UID/operator group. V1 assumes one trusted operator and has no multi-user RBAC. Docker administration already grants powerful host access; CLI access is not a privilege boundary against a Docker administrator.

### Restore and retention safety

Bind restore plans to target identity, engine, artifact digest, database name and expiration. Recheck at execution; reject existing databases and source targets. Track every created resource and only clean up those owned by the operation. Names alone are not sufficient proof of target identity: deployment must supply an immutable operator-controlled target ID and validated endpoint/credentials, with collision checks against source configurations.

Verification assertions are operator-supplied read-only queries executed with restricted privileges/timeouts. Deny mutations and multiple statements as defense in depth; database permissions, not a text filter, enforce read-only behavior. Do not run restored functions with source production credentials. Apply egress restrictions to verification targets.

Retention deletes only exact provider IDs/keys recorded as service-owned. Never issue a broad bucket/folder purge. Protect active restore/download claims, pinned copies and the latest verified copy as specified in the database design. Remote import starts with deletion disabled until ownership and policies are reviewed.

## Review evidence

Planned security verification includes least-privilege exports/restores, RLS coverage, wrong-target rejection, secret scanning of all errors/logs/events, TLS failures, URL redirects/DNS changes, signature tampering, key rotation, malicious restore fixtures, scoped deletion and missing/revoked provider credentials. None has run; no runtime implementation exists.

TODO: actual engine versions, database role grants, provider scopes, host filesystem ownership, online-key acceptance and operator recovery-key custody. H1 approves the design; H4 separately controls production execution.

## References

- [PostgreSQL pg_dump permissions and restore security](https://www.postgresql.org/docs/current/app-pgdump.html)
- [MongoDB mongodump permissions and consistency](https://www.mongodb.com/docs/database-tools/mongodump/)
- [age encryption project](https://github.com/FiloSottile/age)

## Related documentation

- [Infrastructure design](infrastructure-design.md)
- [API and interface design](api-design.md)
- [Release checklist](../06-testing/release-checklist.md)
- [Database state and retention](database-design.md)
- [System overview and approval choices](system-overview.md)
