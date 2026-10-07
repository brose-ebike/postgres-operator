# PgBackupInstance
## Resource Definition

A `PgBackupInstance` records one dump attempt of one database — created and fully managed by the
`backup-worker` binary (the dump `CronJob` owned by a [`PgBackupPolicy`](./backup-policy.md)), not
normally created by hand.

```yaml
apiVersion: postgres.oebc.tools/v1
kind: PgBackupInstance
metadata:
  name: nightly-backup-mydb-abc123
spec:
  database:
    namespace: "default"
    name: "mydb"
  backupPolicy:
    namespace: "default"
    name: "nightly-backup"
status:
  phase: Success
  location:
    type: s3
    s3:
      endpoint: "s3.hetzner.example.com"
      bucket: "pg-backups"
      secure: true
      url: "s3://pg-backups/default/mydb/nightly-backup-mydb-abc123.dump.gpg"
  dumpOptions:
    format: custom
    compressionLevel: 6
  sizeBytes: 104857600
  startedAt: "2026-01-01T02:00:00Z"
  finishedAt: "2026-01-01T02:03:12Z"
  expiresAt: "2026-01-31T02:03:12Z"
```

## Attribute Description

| Attribute                   | Description                                                                                  |
|-------------------------------|--------------------------------------------------------------------------------------------------|
| `spec.database`              | The `PgDatabase` this is a backup of                                                             |
| `spec.backupPolicy`          | The `PgBackupPolicy` that triggered this backup                                                  |
| `status.phase`               | `Pending` → `Success`/`Failure`, see Lifecycle below                                             |
| `status.location`            | Where the (possibly encrypted) dump was stored, same discriminated-union shape as `PgBackupPolicy.spec.storage`. Only populated once `phase` is `Success`. |
| `status.dumpOptions`         | Snapshot of the `pg_dump` options actually used for this specific backup — independent of later edits to the policy's `spec.dumpOptions` |
| `status.sizeBytes`           | Size of the stored (possibly encrypted) object                                                   |
| `status.startedAt`/`.finishedAt` | When the attempt started/finished. `finishedAt` is the timestamp retention is computed from. |
| `status.expiresAt`           | Informational only — cleanup always recomputes retention from `finishedAt` against the policy's *current* settings, not this cached value |
| `status.message`             | Human-readable status/error message                                                              |

## Lifecycle

```
Pending ──success──▶ Success
   │
   └──failure or stale timeout (30m)──▶ Failure
```

- **`Pending`** is set the moment the dump attempt starts, before `pg_dump` even runs — so a
  worker crash mid-dump still leaves an observable record instead of silently vanishing.
- A `Pending` instance stuck for more than 30 minutes is marked `Failure` by the next cleanup run
  (`"stale pending: dump worker did not complete within the timeout (likely crashed or was
  killed)"`).
- `Failure` records are never counted toward `retention.minCount` and never salvaged — they aren't
  valid backups. Cleanup deletes them once `finishedAt` is older than the policy's `retention.maxAge`
  (or a 7-day fallback window if `maxAge` is unset), independent of the per-database retention
  applied to `Success` records.
- `Success` records are retained per `PgBackupPolicy.spec.retention`, evaluated **per database** —
  see the warning on that page.

## Status Conditions

`PgBackupInstance` uses the standard `status.conditions` shape but does not currently set any
condition types of its own — its `status.phase`/`status.message` fields above are the primary
signal. See [Status & Conditions](./index.md#status-conditions) for the general shape.

## Metrics

Each `PgBackupInstance` reconcile updates two Prometheus gauges reflecting the **most recent**
attempt per `(namespace, policy, database)` — not every instance individually, so one stale record
can never shadow a later one:

| Metric                                                     | Labels                              | Meaning |
|---------------------------------------------------------------|----------------------------------------|-------------|
| `postgres_operator_backup_last_phase`                       | `namespace`, `policy`, `database`, `phase` | `1` for the phase of the most recent attempt, `0` for the other two phases of the same triple |
| `postgres_operator_backup_last_success_timestamp_seconds`   | `namespace`, `policy`, `database`      | Unix timestamp (`finishedAt`) of the most recent `Success` — use this to alert on "no successful backup in N days" even if the latest attempt failed |
