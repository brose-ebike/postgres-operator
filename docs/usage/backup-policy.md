# PgBackupPolicy
## Resource Definition

The `PgBackupPolicy` resource configures a logical (`pg_dump`) backup schedule for one or more
databases: where dumps are stored, whether they're encrypted, how long they're kept, and the
`pg_dump` options used to produce them. Databases opt into a policy via their own
[`PgDatabase.spec.backupPolicy`](./database.md) field — the schedule lives entirely on the policy,
not on each database.

```yaml
apiVersion: postgres.oebc.tools/v1
kind: PgBackupPolicy
metadata:
  name: nightly-backup
spec:
  schedule: "0 2 * * *"
  storage:
    type: s3
    s3:
      endpoint:
        value: "s3.hetzner.example.com"
      bucket:
        value: "pg-backups"
      prefix: # optional
        value: "mydb"
      secure: true # optional, default true
      accessKey:
        secretKeyRef:
          name: "backup-storage-credentials"
          key: "accessKey"
      secretKey:
        secretKeyRef:
          name: "backup-storage-credentials"
          key: "secretKey"
  encryption: # optional - omit for unencrypted dumps
    type: gpg-aes
    passphrase:
      secretKeyRef:
        name: "backup-encryption-key"
        key: "passphrase"
  retention:
    minCount: 7 # optional, hard floor - applies per database, not across the whole policy
    maxAge: "720h" # optional, 30 days
  dumpOptions: # optional
    format: custom # optional, default custom
    compressionLevel: 6 # optional, 0-9
  workspaceSizeLimit: "10Gi" # optional, default 10Gi
```

Creating this resource makes the operator reconcile two `CronJob`s: a dump job on `spec.schedule`,
and a cleanup job on a fixed daily schedule that enforces `spec.retention`. Both run the
`backup-worker` binary (the same image the operator itself runs as) **in the operator's own
namespace, under the operator's own `ServiceAccount`** — not a dynamically-created per-policy one.
The operator's existing `ClusterRole` already carries everything the worker needs (cluster-wide
`Secret`/`ConfigMap` reads for credential resolution, `PgDatabase`/`PgInstance` reads, and
`PgBackupInstance` read/write), consolidated in one place rather than minting a new
`ServiceAccount`/`Role`/`RoleBinding`/`ClusterRole`/`ClusterRoleBinding` set per policy.

!!! note "Cross-namespace databases and CronJob placement"
    A `PgDatabase` may reference a `PgBackupPolicy` in a different namespace (same as
    `PgDatabase.spec.instance` already allows for `PgInstance`) — this works without any extra RBAC,
    since the worker already runs with the operator's own cluster-wide read access. One consequence
    of running CronJobs in the operator's namespace rather than each policy's own namespace: a
    `ServiceAccount` can only be mounted by a Pod in its own namespace, so the CronJobs can't carry
    an owner reference back to a policy in a different namespace (Kubernetes ignores a namespaced
    owner reference that crosses namespaces). The operator deletes a policy's CronJobs explicitly
    when the policy is deleted, rather than relying on garbage collection.

## Attribute Description

| Attribute                        | Description                                                                                   | Required |
|------------------------------------|-------------------------------------------------------------------------------------------------|----------|
| `schedule`                       | Cron expression controlling the dump `CronJob`. The cleanup `CronJob` runs on a fixed daily schedule and is not configurable. | :white_check_mark: |
| `storage.type`                   | Storage backend. Only `s3` is implemented today.                                                | :white_check_mark: |
| `storage.s3.endpoint`            | S3-compatible endpoint, `host[:port]`, without scheme. A `PgProperty` — see below.                | :white_check_mark: |
| `storage.s3.bucket`              | Bucket dumps are stored in. Created automatically if it does not exist. A `PgProperty`.          | :white_check_mark: |
| `storage.s3.prefix`              | Optional key prefix under which objects are stored. A `PgProperty`.                              | :x: |
| `storage.s3.secure`              | Whether TLS is used to talk to the endpoint. Plain boolean, default `true` (not a `PgProperty` — there's no realistic case for sourcing a TLS toggle from a Secret/ConfigMap). | :x: |
| `storage.s3.accessKey`           | S3 access key. A `PgProperty`.                                                                   | :white_check_mark: |
| `storage.s3.secretKey`           | S3 secret key. A `PgProperty`.                                                                   | :white_check_mark: |
| `encryption.type`                | `gpg-rsa` (public-key) or `gpg-aes` (passphrase-based symmetric). Omit `encryption` entirely for unencrypted dumps. | :x: |
| `encryption.publicKey`           | Armored GPG public key. A `PgProperty`, required when `type` is `gpg-rsa`.                       | :x: at the schema level, required by CEL validation when `type: gpg-rsa` |
| `encryption.passphrase`          | Symmetric passphrase. A `PgProperty`, required when `type` is `gpg-aes`.                         | :x: at the schema level, required by CEL validation when `type: gpg-aes` |

Every `PgProperty` field above can be set from a literal `value`, a `configMapKeyRef`, or a
`secretKeyRef` — the same pattern [`PgInstance`](./instance.md) uses for its own connection fields.
Credentials aren't singled out for special treatment; the whole destination config is uniformly
resolvable, consistent with the rest of this operator's CRDs.
| `retention.minCount`             | Hard floor: at least this many of the newest backups of **each database** are always kept, regardless of `maxAge` | :x: |
| `retention.maxAge`               | Maximum age before a backup becomes eligible for deletion (subject to `minCount` salvage)        | :x: |
| `dumpOptions.format`             | `plain`, `custom`, `tar`, or `directory` (tarred+gzipped into one object before upload). Default `custom`. | :x: |
| `dumpOptions.compressionLevel`   | `pg_dump --compress=<n>`, `0`-`9`                                                                | :x: |
| `workspaceSizeLimit`             | Size limit of the scratch `emptyDir` volume used for the dump (and its encrypted copy). Default `10Gi`. | :x: |

!!! warning
    `retention.minCount`/`maxAge` are enforced **per database**, not shared across every database
    referencing the policy. `minCount: 5` means 5 kept per database, not 5 total across all of
    them.

### Sizing `workspaceSizeLimit`

The dump and its encrypted copy can briefly coexist in the scratch volume. Size this to roughly
**2x the largest target database's on-disk size**, not the compressed dump size — `pg_dump` and
encryption both need headroom before the final (smaller) artifact is uploaded and the scratch space
reclaimed.

!!! warning "Deleting a policy does not delete stored backups"
    Deleting a `PgBackupPolicy` cascades to delete its owned `PgBackupInstance` records, `CronJob`s,
    and worker RBAC — but **never** the underlying objects already stored at `spec.storage`. This is
    intentional: an accidental `kubectl delete pgbackuppolicy` must not silently destroy backup
    data. Objects are only ever removed via the normal retention/cleanup path while the policy
    existed, or by manual action directly against the storage backend.

!!! warning "Back up your encryption key outside this cluster's primary environment too"
    If `encryption` is configured, the whole point of an encrypted off-site backup is to survive
    losing the environment it was taken in. That only works if the key/passphrase in
    `encryption.secretKeyRef`'s Secret is itself retrievable from somewhere outside that same
    environment. The operator cannot enforce this — make sure whoever owns that Secret has a copy
    stored elsewhere.

## Status Conditions

| Condition Type                              | Meaning                                                                 |
|-----------------------------------------------|----------------------------------------------------------------------------|
| `pgbackuppolicy.postgres.oebc.tools/ready`   | Whether `spec.storage`'s (and, if set, `spec.encryption`'s) secret resolves. The encryption check is skipped entirely when `spec.encryption` is unset. |

See [Status & Conditions](./index.md#status-conditions) for the general condition shape.
