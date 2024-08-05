# Docker Container Files

This repository contains Docker Compose configurations for various services used in our project.

## Directory Structure

```
D:\DOCKER_CONTAINER_FILES\
│
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
├── qdrant\
│   ├── docker-compose.yml
│   └── data\
├── redis\
│   ├── docker-compose.yml
│   └── data\
│
├── elasticsearch.env
├── emqx.env
├── kafka.env
├── minio-archive.env
├── minio-working.env
├── qdrant.env
└── redis.env
```

## Services

1. **Elasticsearch**: A distributed, RESTful search and analytics engine.
2. **EMQX**: A highly scalable, distributed MQTT broker for IoT.
3. **Kafka**: A distributed event streaming platform.
4. **MinIO (Archive)**: Object storage for archival purposes.
5. **MinIO (Working)**: Object storage for active data.
6. **Qdrant**: Vector similarity search engine.
7. **Redis**: In-memory data structure store, used as a database, cache, and message broker.

## Setup and Running

1. Ensure Docker and Docker Compose are installed on your system.
2. Clone this repository to `D:\DOCKER_CONTAINER_FILES\`.
3. Navigate to each service directory and start the service:

   ```
   cd D:\DOCKER_CONTAINER_FILES\<service_name>
   docker-compose --env-file ../<service_name>.env up -d
   ```

   Replace `<service_name>` with the name of the service you want to start (e.g., elasticsearch, emqx, kafka, etc.).

4. To stop a service:

   ```
   docker-compose --env-file ../<service_name>.env down
   ```

## Service Details

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

### Qdrant
- HTTP Port: 6333
- gRPC Port: 6334

### Redis
- Port: 6379

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