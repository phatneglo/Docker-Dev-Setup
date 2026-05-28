# PostgreSQL WAL Backup API

This service creates scheduled physical backups with `pg_basebackup`, uploads backup ZIPs and WAL files to S3-compatible storage, keeps only the configured number of backup generations, and exposes a small authenticated API with Swagger.

Swagger:

```text
http://localhost:8090/swagger
```

Authentication header:

```text
X-Backup-API-Key: value of BACKUP_API_KEY
```

Manual backup:

```powershell
$headers = @{ "X-Backup-API-Key" = "change-backup-api-key" }
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/backups/run -Headers $headers
```

Upload WAL after new data:

```powershell
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/wal/upload -Headers $headers
```

List PITR chains:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers
```

Each chain pairs one base ZIP with one WAL folder:

```text
app-backup/base/backup-YYYYMMDD-HHMMSS.zip
app-backup/wal/backup-YYYYMMDD-HHMMSS/
```

Status:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/status -Headers $headers
```

List restore options:

```powershell
Invoke-RestMethod -Uri http://localhost:8090/v1/restores/options -Headers $headers
```

Restore into a test database:

```powershell
$body = @{
  backup_name = "backup-YYYYMMDD-HHMMSS.zip"
  target_database = "appdb_restore_test"
  drop_if_exists = $true
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/restores/run -Headers $headers -ContentType "application/json" -Body $body
```

This test restore does not replay WAL. Full point-in-time recovery must use the base ZIP plus the matching WAL folder in a separate PostgreSQL restore instance.
