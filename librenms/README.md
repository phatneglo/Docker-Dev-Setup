# LibreNMS Docker Setup

This folder runs LibreNMS with MariaDB and Redis using Docker Compose.

The web UI is available at:

```text
http://localhost:8000
```

## Files

- Compose file: [docker-compose.yml](/d:/DOCKER_CONTAINER_FILES/librenms/docker-compose.yml:1)
- Env file: [librenms.env](/d:/DOCKER_CONTAINER_FILES/librenms.env:1)

## Start LibreNMS

Run from PowerShell:

```powershell
cd D:\DOCKER_CONTAINER_FILES\librenms
docker compose --env-file ..\librenms.env up -d
```

## Stop LibreNMS

```powershell
cd D:\DOCKER_CONTAINER_FILES\librenms
docker compose --env-file ..\librenms.env down
```

## Check Running Containers

```powershell
docker ps
```

## View LibreNMS Logs

```powershell
docker logs --tail 200 librenms-librenms-1
```

## Create the First Admin User

LibreNMS does not use `librenmspass` as the website password. That value is only the database password.

Create your web login user with this command:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan user:add admin --password="YourStrongPasswordHere" --role=admin --email="you@example.com" --full-name="Admin User"
```

Example:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan user:add phatneglo --password="MyLibreNMS123!" --role=admin --email="phat@example.com" --full-name="Phat Neglo"
```

After that, log in at:

```text
http://localhost:8000/login
```

## Scan Your Network

Scan a subnet:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan scan 192.168.100.0/24
```

Scan multiple subnets:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan scan 192.168.1.0/24 192.168.100.0/24
```

Ping-only scan:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan scan 192.168.100.0/24 --ping-only
```

Scan with more threads:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan scan 192.168.100.0/24 --threads=64
```

## Common Notes

- Host port `8000` is mapped to LibreNMS container port `8000`.
- Always start Compose with `--env-file ..\librenms.env` because the env file is outside this folder.
- If devices fail to add during scan, the usual reason is missing or incorrect SNMP settings on those devices.

## Useful Commands

Check how many users exist:

```powershell
docker exec librenms-db-1 mariadb -ulibrenms -plibrenmspass librenms -e "SELECT COUNT(*) AS users_count FROM users;"
```

List available LibreNMS CLI commands:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan list
```
