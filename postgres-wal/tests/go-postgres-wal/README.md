# PostgreSQL WAL Stack Go Test

This is a small functional test tool for the `postgres-wal` stack.

It checks:

- Pgpool application login.
- Concurrent database users/requests.
- Replication visibility from the primary.
- WAL archive generation using `pg_switch_wal()`.
- Simple backup and restore round trip using `pg_dump` and `psql` inside the running container.

## Run

Start the stack first:

```powershell
cd D:\TOOLS\DOCKER-DEV\postgres-wal
docker compose --env-file ..\postgres-wal.env up -d
```

Run the test:

```powershell
cd D:\TOOLS\DOCKER-DEV\postgres-wal\tests\go-postgres-wal
go mod tidy
go run .
```

Useful flags:

```powershell
go run . -concurrency 100 -requests 1000
go run . -host localhost -port 55432
go run . -skip-restore
```

The restore test creates a temporary table named `wal_test_restore_<timestamp>` in `appdb`, dumps it into `postgres-wal/backups`, drops it, restores it, validates row count, then drops it again.
