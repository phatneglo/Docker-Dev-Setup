# How To Use Backup And Restore

This is the simple Docker runbook.

Use this file when you want to know:

- How automatic backup works.
- How to run a manual backup.
- How to restore a backup for testing.
- How to restore a full backup plus WAL.

## 1. Start The Docker Stack

From PowerShell:

```powershell
cd D:\TOOLS\DOCKER-DEV\postgres-wal
docker compose --env-file ..\postgres-wal.env up -d
docker compose --env-file ..\postgres-wal.env ps
```

Open Swagger:

```text
http://localhost:8090/swagger
```

Set the API header once in PowerShell:

```powershell
$headers = @{ "X-Backup-API-Key" = "change-backup-api-key" }
```

Check the backup API:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/status -Headers $headers | ConvertTo-Json -Depth 6
```

## 2. How Automatic Backup Works

These settings are in:

```text
D:\TOOLS\DOCKER-DEV\postgres-wal.env
```

```env
BACKUP_SCHEDULE_ENABLED=true
BACKUP_DAILY_TIME=02:00
BACKUP_TIMEZONE=Asia/Manila
BACKUP_RETENTION_COUNT=7
BACKUP_WAL_UPLOAD_ENABLED=true
BACKUP_WAL_UPLOAD_INTERVAL_SECONDS=60
BACKUP_WAL_UPLOAD_BATCH_SIZE=10
BACKUP_WAL_UPLOAD_FORCE_SWITCH=false
```

What happens automatically:

1. At `BACKUP_DAILY_TIME`, the API creates a new base backup ZIP.
2. The ZIP is saved locally in `postgres-wal/backups`.
3. The ZIP is uploaded to S3 or MinIO under `app-backup/base`.
4. That backup becomes the active PITR chain.
5. WAL upload scans every `BACKUP_WAL_UPLOAD_INTERVAL_SECONDS`.
6. New archived WAL files are uploaded to the active backup folder.

Example S3 layout:

```text
app-backup/base/backup-20260601-020000.zip
app-backup/wal/backup-20260601-020000/0000000100000000000000A1
app-backup/wal/backup-20260601-020000/0000000100000000000000A2
```

When tomorrow's backup runs, WAL switches to the new folder:

```text
app-backup/base/backup-20260602-020000.zip
app-backup/wal/backup-20260602-020000/
```

Check automatic backup and WAL upload status:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/status -Headers $headers | ConvertTo-Json -Depth 6
Invoke-RestMethod -Uri http://localhost:8090/v1/wal/status -Headers $headers | ConvertTo-Json -Depth 6
Invoke-RestMethod -Uri http://localhost:8090/v1/s3/status -Headers $headers | ConvertTo-Json -Depth 8
```

Important:

```text
WAL upload is not one upload per SQL query.
PostgreSQL writes queries into WAL.
When a WAL file is archived, the API uploads it to S3/MinIO.
```

## 3. Manual Backup Now

Run this if you want a backup immediately:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/backups/run -Headers $headers | ConvertTo-Json -Depth 8
```

Then check the active restore chain:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers | ConvertTo-Json -Depth 8
```

You should see something like:

```text
id: backup-20260601-143000
s3_backup_key: app-backup/base/backup-20260601-143000.zip
wal_prefix: app-backup/wal/backup-20260601-143000/
active: true
```

Manual WAL upload for testing:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/wal/upload -Headers $headers | ConvertTo-Json -Depth 6
```

Normal operation does not need the manual WAL command because automatic upload is enabled.

## 4. Restore Option A: Simple Database Restore

Use this when you only want to restore the SQL dump inside the backup ZIP into a new database.

This does not replay WAL.

List restore options:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/restores/options -Headers $headers | ConvertTo-Json -Depth 8
```

Restore into a new test database:

```powershell
$body = @{
  backup_name = "backup-YYYYMMDD-HHMMSS.zip"
  target_database = "appdb_restore_test"
  drop_if_exists = $true
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/restores/run -Headers $headers -ContentType "application/json" -Body $body | ConvertTo-Json -Depth 8
```

Verify the restored test database:

```powershell
docker exec -e PGPASSWORD=0yq5h3to9 postgres-wal-pg-0 psql -U appuser -d appdb_restore_test -c "select current_database(), current_user, now();"
```

Use this option for quick backup checks.

## 5. Restore Option B: Full Backup Plus WAL Restore

Use this when you want to restore the base backup plus WAL files.

This restores a separate PostgreSQL instance on port `55433`.

It does not overwrite production.

Step 1: choose the latest chain:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers | ConvertTo-Json -Depth 8
```

Step 2: download the latest chain from S3/MinIO:

```powershell
$body = @{
  latest = $true
  overwrite = $true
} | ConvertTo-Json

$download = Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/pitr/download -Headers $headers -ContentType "application/json" -Body $body
$download | ConvertTo-Json -Depth 8
```

Step 3: prepare the restore files:

```powershell
.\restore-lab\prepare-pitr-restore.ps1 -BackupName $download.backup_name -WalSource $download.wal_download_dir
```

Step 4: start the restore database:

```powershell
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env up -d
```

Step 5: watch recovery:

```powershell
docker logs -f postgres-wal-pitr-restore
```

Good final log:

```text
database system is ready to accept connections
```

Step 6: verify the restored database:

```powershell
docker exec -e PGPASSWORD=0yq5h3to9 postgres-wal-pitr-restore psql -U appuser -d appdb -c "select current_database(), current_user, now();"
```

Connect your test app to the restore database:

```text
host: localhost
port: 55433
database: appdb
user: appuser
password: value of POSTGRES_PASSWORD
```

Stop the restore lab when finished:

```powershell
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
```

## 6. Which Restore Should I Use?

Use database restore when:

- You want to quickly test a backup.
- You want a new database like `appdb_restore_test`.
- You do not need WAL replay.

Use full backup plus WAL restore when:

- You want the database state after the base backup.
- You want WAL replay.
- You are testing disaster recovery.
- You are okay using a separate PostgreSQL restore container.

Important rule:

```text
Database-level restore can restore to a target database name.
Full WAL replay restores a whole PostgreSQL instance.
```

## 7. Common Daily Flow

Normal day:

```text
1. Docker stack is running.
2. Scheduled backup runs at BACKUP_DAILY_TIME.
3. New S3 folder is created for that backup.
4. Automatic WAL uploader keeps uploading WAL to that folder.
5. You check /v1/pitr/chains when you need to restore.
```

Manual test day:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/backups/run -Headers $headers
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers | ConvertTo-Json -Depth 8
Invoke-RestMethod -Uri http://localhost:8090/v1/s3/status -Headers $headers | ConvertTo-Json -Depth 8
```

Restore test day:

```powershell
$body = @{
  latest = $true
  overwrite = $true
} | ConvertTo-Json

$download = Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/pitr/download -Headers $headers -ContentType "application/json" -Body $body
.\restore-lab\prepare-pitr-restore.ps1 -BackupName $download.backup_name -WalSource $download.wal_download_dir
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env up -d
docker exec -e PGPASSWORD=0yq5h3to9 postgres-wal-pitr-restore psql -U appuser -d appdb -c "select current_database(), current_user, now();"
```
