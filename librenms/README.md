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

## Troubleshooting: Devices Not Showing Data (e.g., Ruijie Switches)

Good news: **LibreNMS can already talk to your device**. Your working SNMP command proves it's alive:

```bash
snmpwalk -v2c -c unms 10.10.0.1 1.3.6.1.2.1.1
```

So the correct SNMP community is confirmed. However, the device may be added but discovery/polling has not fully populated ports, graphs, CPU, memory, sensors, etc.

### Do this first: run discovery and poller manually

Run these from PowerShell:

```powershell
docker exec -u librenms librenms-librenms-1 bash -lc "cd /opt/librenms && ./lnms device:discover 10.10.0.1 -vvv"
```

Then:

```powershell
docker exec -u librenms librenms-librenms-1 bash -lc "cd /opt/librenms && ./lnms device:poll 10.10.0.1 -vvv"
```

After that, refresh LibreNMS and click the device name.

### Check if device exposes interface data

Your SNMP test only checks system info. Now test if the device exposes ports/interfaces:

```powershell
docker exec -u librenms librenms-librenms-1 snmpwalk -v2c -c unms 10.10.0.1 1.3.6.1.2.1.2.2.1.2
```

This should show interface names (LAN/WAN/eth ports). Also test:

```powershell
docker exec -u librenms librenms-librenms-1 snmpwalk -v2c -c unms 10.10.0.1 1.3.6.1.2.1.31.1.1.1.1
```

If these return data, LibreNMS should be able to discover ports. If they return nothing, your device SNMP may only expose basic system info.

### Check device SNMP settings

On your router/switch, verify SNMP configuration:

| Setting            | Recommended Value |
| ------------------ | ----------------- |
| SNMP Version       | v2c               |
| Community          | `unms`            |
| Permission         | Read-only         |
| MIB View           | All               |
| Allowed Manager IP | Your LibreNMS IP   |
| Port               | UDP 161           |

The critical setting is **MIB View = All**. If limited, LibreNMS may only see basic info.

### Verify LibreNMS SNMP settings

In LibreNMS, check the device configuration:

```text
Devices > [Your Device IP] > Edit Device > SNMP
```

Ensure the community matches what's configured on the device. If you added it with the wrong community:

1. Delete the device from LibreNMS
2. Add it again
3. Use SNMP v2c
4. Use the correct community string

### Check if poller is running

Validate your LibreNMS setup:

```powershell
docker exec -u librenms librenms-librenms-1 bash -lc "cd /opt/librenms && ./validate.php"
```

Also check containers:

```powershell
docker compose ps
```

If the poller/dispatcher container is not running, devices will be added but metrics won't update.

### Quick fix: run both discovery and polling together

```powershell
docker exec -u librenms librenms-librenms-1 bash -lc "cd /opt/librenms && ./lnms device:discover 10.10.0.1 -vvv && ./lnms device:poll 10.10.0.1 -vvv"
```

Then refresh LibreNMS to see updated data.

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
