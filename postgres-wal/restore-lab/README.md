# PITR Restore Lab

This folder is for safe physical point-in-time recovery testing.

It does not restore into the production `pg-0`, `pg-1`, `pg-2`, or Pgpool containers. It starts a separate PostgreSQL container named `postgres-wal-pitr-restore` on host port `55433`.

## What This Tests

This tests the real physical restore flow:

```text
base backup ZIP + WAL files -> separate PostgreSQL restore instance
```

This is different from `/v1/restores/run`, which restores only the logical SQL dump into a new database and does not replay WAL.

## Step 1: Pick A Chain

```powershell
$headers = @{ "X-Backup-API-Key" = "change-backup-api-key" }
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers | ConvertTo-Json -Depth 8
```

Choose the `id`, for example:

```text
backup-20260528-180000
```

The matching local ZIP is:

```text
postgres-wal/backups/backup-20260528-180000.zip
```

The matching S3 or MinIO WAL folder is:

```text
app-backup/wal/backup-20260528-180000/
```

For local testing, the script copies WAL from:

```text
postgres-wal/wal-archive
```

## Step 2: Prepare The Restore Files

From `postgres-wal`:

```powershell
.\restore-lab\prepare-pitr-restore.ps1 -BackupName backup-20260528-180000.zip
```

Optional target time:

```powershell
.\restore-lab\prepare-pitr-restore.ps1 -BackupName backup-20260528-180000.zip -RecoveryTargetTime "2026-05-28 18:30:00+08"
```

The script:

- Extracts `base.tar.gz` into `restore-lab/data`.
- Extracts `pg_wal.tar.gz` into `restore-lab/data/pg_wal`.
- Copies matching WAL archive files into `restore-lab/wal`.
- Creates `recovery.signal`.
- Writes a minimal `postgresql.conf` with `restore_command = 'cp /restore-wal/%f %p'`.

## Step 3: Start The Restore Container

Always reset the restore volume first:

```powershell
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env up -d
```

Watch recovery:

```powershell
docker logs -f postgres-wal-pitr-restore
```

When recovery is complete, PostgreSQL promotes itself and starts accepting connections.

Good signs in the logs:

```text
restored log file "000000010000000000000072" from archive
archive recovery complete
database system is ready to accept connections
```

These messages are usually normal at the end of available WAL:

```text
cannot stat '/restore-wal/00000002.history'
cannot stat '/restore-wal/000000010000000000000077'
```

They mean PostgreSQL looked for a next timeline or next WAL file, did not find one, and finished recovery with the WAL it had. The important final line is `database system is ready to accept connections`.

## Step 4: Verify The Restored Database

Connect through the separate restore port:

```powershell
docker exec -e PGPASSWORD=0yq5h3to9 postgres-wal-pitr-restore psql -U appuser -d appdb -c "select current_database(), current_user, now();"
```

From Windows host:

```powershell
psql -h localhost -p 55433 -U appuser -d appdb
```

Your PHPMaker app can also be pointed to:

```text
host: localhost
port: 55433
database: appdb
user: appuser
password: POSTGRES_PASSWORD
```

Only do this for testing. Do not point production traffic to the restore lab unless you intentionally promote it as a replacement after a disaster.

## Step 5: Stop The Restore Lab

```powershell
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
```

The `-v` matters because it deletes the old restore volume. Without it, Docker may reuse an older restored database.

## If WAL Is In MinIO Only

Download the matching chain folder from MinIO first, then pass it as `-WalSource`.

Example target folder:

```text
postgres-wal/restore-lab/wal-download/backup-20260528-180000
```

Then:

```powershell
.\restore-lab\prepare-pitr-restore.ps1 -BackupName backup-20260528-180000.zip -WalSource .\restore-lab\wal-download\backup-20260528-180000
```

Do not mix WAL from random backup folders. Use the WAL folder shown by `/v1/pitr/chains`.
