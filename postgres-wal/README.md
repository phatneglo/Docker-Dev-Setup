# PostgreSQL HA + WAL Reference Stack

This folder contains a practical PostgreSQL high-availability reference setup:

- 1 primary PostgreSQL node.
- 2 streaming replicas.
- Pgpool-II as the single database entrypoint. It exposes host port `55432` by default and uses container port `5432` internally.
- Read load balancing for eligible read queries.
- Automatic failover through `repmgr`/Pgpool health checks.
- WAL archiving into `./wal-archive` for point-in-time recovery workflows.
- Backup API that creates daily base backup ZIPs and organizes uploaded WAL by backup generation.
- Optional Prometheus PostgreSQL exporter on port `9187`.

The stack uses `bitnamilegacy/postgresql-repmgr` and `bitnamilegacy/pgpool` because the previous `bitnami/...` Docker Hub repositories no longer expose the required tags.

Open [architecture.svg](architecture.svg) for the simple diagram.
Read [architecture.md](architecture.md) for the PHPMaker-focused explanation, scenarios, restore flow, and FAQs.
Read [backup-api.md](backup-api.md) for scheduled S3 backups, WAL upload chains, retention, Swagger, and step-by-step restore concepts.
Read [restore-lab/README.md](restore-lab/README.md) for the safe physical PITR restore lab.

## Important Reality Check

This is a strong Docker Compose reference stack, not a magic "never down" system. True enterprise PostgreSQL needs separate machines or availability zones, durable storage, tested failover, monitoring, network controls, and regular restore drills. A single Docker host can still fail completely.

For production, use this design pattern, but deploy nodes across separate hosts. For the safest managed option, use a cloud managed PostgreSQL HA service with PITR enabled.

## Quick Start

```powershell
cd D:\TOOLS\DOCKER-DEV\postgres-wal
notepad ..\postgres-wal.env
docker compose --env-file ..\postgres-wal.env up -d
docker compose ps
```

Applications should connect to:

```text
host: localhost
port: 55432
database: value of POSTGRES_DB
username: value of POSTGRES_USER
password: value of POSTGRES_PASSWORD
```

Inside another Docker Compose project, connect through the external network:

```yaml
networks:
  postgres-wal-network:
    external: true
    name: postgres-wal-network
```

Then use `pgpool:5432` if the app is attached to the same network.

The container still listens on `5432` internally. The Windows host port is `55432` by default so it does not conflict with an existing PostgreSQL running on your machine.

## Start With Monitoring Exporter

```powershell
docker compose --env-file ..\postgres-wal.env --profile monitoring up -d
```

Exporter endpoint:

```text
http://localhost:9187/metrics
```

## Start With Backup API

```powershell
docker compose --env-file ..\postgres-wal.env up -d backup-api
```

Swagger UI:

```text
http://localhost:8090/swagger
```

Use `X-Backup-API-Key` with the value from `BACKUP_API_KEY` in `postgres-wal.env`.

Basic backup/WAL flow:

```powershell
$headers = @{ "X-Backup-API-Key" = "change-backup-api-key" }
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/backups/run -Headers $headers
Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/wal/upload -Headers $headers
Invoke-RestMethod -Uri http://localhost:8090/v1/pitr/chains -Headers $headers
```

The important result is one restore chain:

```text
app-backup/base/backup-YYYYMMDD-HHMMSS.zip
app-backup/wal/backup-YYYYMMDD-HHMMSS/
```

Automatic WAL upload is controlled by:

```env
BACKUP_WAL_UPLOAD_ENABLED=true
BACKUP_WAL_UPLOAD_INTERVAL_SECONDS=60
BACKUP_WAL_UPLOAD_BATCH_SIZE=10
BACKUP_WAL_UPLOAD_FORCE_SWITCH=false
```

This means the API continuously uploads archived WAL to the active chain. It does not upload one file for every SQL query; it uploads completed PostgreSQL WAL files.

Test full physical PITR in a separate container:

```powershell
$body = @{
  latest = $true
  overwrite = $true
} | ConvertTo-Json

Invoke-RestMethod -Method Post -Uri http://localhost:8090/v1/pitr/download -Headers $headers -ContentType "application/json" -Body $body

.\restore-lab\prepare-pitr-restore.ps1 -BackupName backup-YYYYMMDD-HHMMSS.zip
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env down -v
docker compose -f docker-compose.restore.yml --env-file ..\postgres-wal.env up -d
docker logs -f postgres-wal-pitr-restore
```

## WAL Archive

WAL files are copied to:

```text
postgres-wal/wal-archive
```

The mounted config file `config/wal-archive.conf` overrides the repmgr image default `archive_command = '/bin/true'` so WAL files are actually copied to the archive folder.

WAL archiving is not the same as a full backup. You need both:

- A base backup or dump from your backup tool.
- The WAL archive generated after that backup.

Together, those allow point-in-time recovery. Without base backups, WAL files alone are not enough.

## Common Operations

Check cluster status:

```powershell
docker exec -it postgres-wal-pg-0 bash -lc "USER=postgres /opt/bitnami/repmgr/bin/repmgr cluster show"
```

Check replication slots and senders:

```powershell
docker exec -it -e PGPASSWORD=<POSTGRES_SUPERUSER_PASSWORD> postgres-wal-pg-0 psql -U postgres -d postgres -c "select * from pg_stat_replication;"
```

Check replica lag:

```powershell
docker exec -it -e PGPASSWORD=<POSTGRES_SUPERUSER_PASSWORD> postgres-wal-pg-1 psql -U postgres -d postgres -c "select now() - pg_last_xact_replay_timestamp() as replica_lag;"
```

Force WAL switch to verify archiving:

```powershell
docker exec -it -e PGPASSWORD=<POSTGRES_SUPERUSER_PASSWORD> postgres-wal-pg-0 psql -U postgres -d postgres -c "select pg_switch_wal();"
Get-ChildItem .\wal-archive
```

Simulate primary failure:

```powershell
docker stop postgres-wal-pg-0
docker exec -it postgres-wal-pg-1 bash -lc "USER=postgres /opt/bitnami/repmgr/bin/repmgr cluster show"
```

After failover testing, do not blindly restart the old primary as if nothing happened. Rejoin it properly or rebuild the node from the current primary.

## Performance Defaults

The compose file sets conservative starter values:

- `max_connections=300`
- Pgpool children: `128`
- `shared_buffers=512MB`
- `effective_cache_size=1536MB`
- `maintenance_work_mem=128MB`
- `work_mem=16MB`
- `wal_compression=on`
- `checkpoint_completion_target=0.9`

These are not universal optimal values. Tune them based on server RAM, CPU, disk latency, query patterns, and measured load tests. For thousands of requests, the most important rule is to pool connections and avoid opening one database connection per request.

## Best Practices Checklist

- Put Pgpool or PgBouncer in front of PostgreSQL so application spikes do not exhaust database connections.
- Keep write traffic on the primary and move safe read traffic to replicas.
- Put each database node on separate hardware or availability zones in production.
- Store WAL archives on durable storage, preferably object storage with lifecycle rules and immutability/versioning.
- Monitor replication lag, disk usage, WAL archive failures, slow queries, locks, CPU, memory, and connection counts.
- Test restore regularly. Untested backups are only assumptions.
- Enable TLS and restrict database network access before exposing this outside a trusted Docker network.
- Use strong passwords. For real production, use a secrets manager instead of committed env files.
- Add application-level retry logic for transient failover errors.
- Keep schema migrations controlled so they do not overload replicas or lock hot tables during peak traffic.

## Recommended Enterprise Target Architecture

For serious production, evolve this into:

- Load balancer or PgBouncer layer with health checks.
- 3 PostgreSQL nodes across separate hosts or availability zones.
- Synchronous replication for critical transactions if the latency tradeoff is acceptable.
- Asynchronous replicas for read scaling and reporting.
- WAL archiving to S3-compatible storage, MinIO, or another durable remote store.
- Automated base backups using your existing backup tool.
- Prometheus, Grafana, and alerting.
- Restore runbooks and regular disaster recovery drills.
