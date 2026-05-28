# Docker Container Files

This repository contains Docker Compose configurations for various services used in our project.

## Directory Structure

```
D:\TOOLS\DOCKER-DEV\
│
├── clickhouse\
│   ├── docker-compose.yml
│   └── data\
├── coturn\
│   ├── docker-compose.yml
│   ├── turnserver.conf
│   ├── start.bat
│   ├── test.html
│   └── data\
├── elasticsearch\
│   ├── docker-compose.yml
│   └── data\
├── emqx\
│   ├── docker-compose.yml
│   ├── data\
│   └── log\
├── kafka\
│   ├── docker-compose.yml
│   └── data\
├── minio-archive\
│   ├── docker-compose.yml
│   └── data\
├── minio-working\
│   ├── docker-compose.yml
│   └── data\
├── onlyoffice\
│   ├── docker-compose.yml
│   └── data\
├── postgres-wal\
│   ├── docker-compose.yml
│   ├── docker-compose.restore.yml
│   ├── README.md
│   ├── architecture.md
│   ├── architecture.svg
│   ├── backup-api.md
│   ├── backup-api\          # Go service + Dockerfile for backup/WAL/PITR API
│   ├── config\              # wal-archive.conf and other PG overrides
│   ├── restore-lab\         # Isolated PITR restore harness (data/ is gitignored)
│   ├── tests\               # Go integration tests
│   ├── backups\             # Local base-backup output (gitignored)
│   └── wal-archive\         # WAL files written by archive_command (gitignored)
├── qdrant\
│   ├── docker-compose.yml
│   └── data\
├── redis\
│   ├── docker-compose.yml
│   └── data\
│
├── clickhouse.env
├── coturn.env
├── elasticsearch.env
├── emqx.env
├── kafka.env
├── minio-archive.env
├── minio-working.env
├── onlyoffice.env
├── postgres-wal.env
├── qdrant.env
└── redis.env
```

## Services

1. **CoTURN**: STUN/TURN server for WebRTC NAT traversal and media relay.
2. **Elasticsearch**: A distributed, RESTful search and analytics engine.
3. **EMQX**: A highly scalable, distributed MQTT broker for IoT.
4. **Kafka**: A distributed event streaming platform.
5. **MinIO (Archive)**: Object storage for archival purposes.
6. **MinIO (Working)**: Object storage for active data.
7. **OnlyOffice**: An online office suite for document editing and collaboration.
8. **Qdrant**: Vector similarity search engine.
9. **Redis**: In-memory data structure store, used as a database, cache, and message broker.
10. **ClickHouse**: Open-source column-oriented database management system.
11. **PostgreSQL HA + WAL**: Reference PostgreSQL cluster with replicas, Pgpool, and WAL archiving.

## Setup and Running

1. Ensure Docker and Docker Compose are installed on your system.
2. Clone this repository to `D:\TOOLS\DOCKER-DEV\`.
3. Navigate to each service directory and start the service:

   ```
   cd D:\TOOLS\DOCKER-DEV\<service_name>
   docker compose --env-file ../<service_name>.env up -d
   ```

   Replace `<service_name>` with the name of the service you want to start (e.g., elasticsearch, emqx, kafka, etc.).

4. To stop a service:

   ```
   docker compose --env-file ../<service_name>.env down
   ```

## Service Details

### CoTURN
- STUN/TURN Port: 3478 (UDP/TCP)
- STUN/TURN TLS Port: 5349 (UDP/TCP)
- UDP Relay Port Range: 60000-60100
- Authentication: username=user, password=coturn123secret
- External IP: 127.0.0.1 (key for TURN functionality)
- **Quick Start**: `cd coturn && start.bat`
- **Test Page**: test.html
- **Configuration**: Based on official CoTURN README.turnserver

### EMQX
- MQTT Port: 1883
- MQTT/WebSocket Port: 8083
- MQTT/SSL Port: 8084
- MQTT/SSL Port: 8883
- Dashboard Port: 18083

### Elasticsearch
- HTTP Port: 9200

### Kafka
- Broker Port: 9092

### MinIO (both Archive and Working)
- API Port: 9000 (Working), 9010 (Archive)
- Console Port: 9001 (Working), 9011 (Archive)

### OnlyOffice
- Document Server: 8088

### Qdrant
- HTTP Port: 6333
- gRPC Port: 6334

### Redis
- Port: 6379

### ClickHouse
- HTTP interface: 8123
- Native TCP interface: 9000
- MySQL protocol: 9004
- PostgreSQL protocol: 9005
- Inter-server communication: 9009

### PostgreSQL HA + WAL
- Folder: `postgres-wal`
- Env file: `postgres-wal.env`
- Entrypoint: Pgpool on host port `55432` to avoid conflict with local PostgreSQL on 5432
- Internal container port: `5432` (use `pgpool:5432` from other compose projects attached to `postgres-wal-network`)
- Includes: 1 primary, 2 streaming replicas, read load balancing, repmgr failover checks, WAL archiving to `./wal-archive`
- Prometheus exporter: port `9187` — enable with `--profile monitoring`
- Backup API: port `8090` — Swagger at `http://localhost:8090/swagger`, auth via `X-Backup-API-Key` header (value from `BACKUP_API_KEY` in `postgres-wal.env`)
- PITR restore lab: `docker-compose.restore.yml` + `restore-lab/prepare-pitr-restore.ps1`
- Docs: [postgres-wal/README.md](postgres-wal/README.md), [postgres-wal/architecture.md](postgres-wal/architecture.md), [postgres-wal/backup-api.md](postgres-wal/backup-api.md), [postgres-wal/architecture.svg](postgres-wal/architecture.svg)

## Setup for Different Projects

1. Decide which services your project needs.
2. Create a new `docker-compose.yml` file in your project directory.
3. For each service you need, add a service definition that uses the external network from the corresponding service in this repository.

For example, if your project needs Elasticsearch and Redis:

```yaml
version: '3.8'

services:
  your_app:
    # Your app configuration here
    networks:
      - elasticsearch_network
      - redis_network

networks:
  elasticsearch_network:
    external: true
    name: elasticsearch_network
  redis_network:
    external: true
    name: redis_network
```

4. Start the required services from this repository:

```bash
cd D:\DOCKER_CONTAINER_FILES\elasticsearch
docker-compose --env-file ../elasticsearch.env up -d

cd D:\DOCKER_CONTAINER_FILES\redis
docker-compose --env-file ../redis.env up -d
```

5. Then start your project's services:

```bash
cd /path/to/your/project
docker-compose up -d
```
