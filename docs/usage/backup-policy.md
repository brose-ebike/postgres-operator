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
      endpoint: "s3.hetzner.example.com"
      bucket: "pg-backups"
      prefix: "mydb" # optional
      secure: true # optional, default true
      secretRef:
        name: "backup-storage-credentials" # keys: accessKey, secretKey
  encryption: # optional - omit for unencrypted dumps
    type: gpg-aes
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

Creating this resource makes the operator reconcile two owned `CronJob`s in the same namespace: a
dump job on `spec.schedule`, and a cleanup job on a fixed daily schedule that enforces
`spec.retention`. Both run the `backup-worker` binary (the same image the operator itself runs as)
under a dedicated, per-policy `ServiceAccount`/`Role`/`RoleBinding` the operator also creates and
owns.

!!! note "Cross-namespace databases"
    A `PgDatabase` may reference a `PgBackupPolicy` in a different namespace (same as
    `PgDatabase.spec.instance` already allows for `PgInstance`). To support this, the operator also
    creates a per-policy, cluster-scoped `ClusterRole`/`ClusterRoleBinding` granting the worker
    read-only, cluster-wide access to `PgDatabase`/`PgInstance` and the `Secret`/`ConfigMap` objects
    their credentials resolve from — it has no way to know in advance which namespaces a policy's
    databases will live in. Unlike the namespaced `ServiceAccount`/`Role`/`RoleBinding` above, these
    two objects cannot carry an owner reference (Kubernetes does not support a cluster-scoped object
    being owned by a namespaced one) and are deleted explicitly by the operator when the policy is
    deleted, rather than garbage-collected automatically.

## Attribute Description

| Attribute                        | Description                                                                                   | Required |
|------------------------------------|-------------------------------------------------------------------------------------------------|----------|
| `schedule`                       | Cron expression controlling the dump `CronJob`. The cleanup `CronJob` runs on a fixed daily schedule and is not configurable. | :white_check_mark: |
| `storage.type`                   | Storage backend. Only `s3` is implemented today.                                                | :white_check_mark: |
| `storage.s3.endpoint`            | S3-compatible endpoint, `host[:port]`, without scheme                                           | :white_check_mark: |
| `storage.s3.bucket`              | Bucket dumps are stored in. Created automatically if it does not exist.                         | :white_check_mark: |
| `storage.s3.prefix`              | Optional key prefix under which objects are stored                                              | :x: |
| `storage.s3.secure`              | Whether TLS is used to talk to the endpoint. Default `true`.                                     | :x: |
| `storage.s3.secretRef.name`      | Name of a Secret (same namespace) with `accessKey`/`secretKey` keys                              | :white_check_mark: |
| `encryption.type`                | `gpg-rsa` (public-key) or `gpg-aes` (passphrase-based symmetric). Omit `encryption` entirely for unencrypted dumps. | :x: |
| `encryption.secretKeyRef`        | Key material: an armored public key for `gpg-rsa`, a passphrase for `gpg-aes`                   | :x: at the schema level, but required if `encryption` is set |
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
