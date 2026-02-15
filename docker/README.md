# Docker dependencies (PostgreSQL, Redis, Kafka)

This folder contains Docker Compose to run the backing services for **million-rps** on your machine.

## What runs

| Service   | Image              | Port | Purpose                    |
|-----------|--------------------|------|----------------------------|
| PostgreSQL| `postgres:16-alpine` | 5432 | Persistent todo data       |
| Redis     | `redis:7-alpine`   | 6379 | Todo list cache            |
| Kafka     | `bitnami/kafka:3.7`| 9092 | Todo command queue (KRaft) |

Kafka runs in **KRaft** mode (no Zookeeper).

## Start dependencies

From the **project root**:

```bash
docker compose -f docker/docker-compose.yml up -d
```

Or from this folder:

```bash
cd docker && docker compose up -d
```

Check that all containers are up:

```bash
docker compose -f docker/docker-compose.yml ps
```

## Connect the app

1. Start the dependencies (see above).
2. Set environment variables to point at localhost (see `docker/.env.example`):

   ```bash
   export DATABASE_URL="postgres://app:appsecret@localhost:5432/million_rps?sslmode=disable"
   export REDIS_URL="redis://localhost:6379/0"
   export KAFKA_BROKERS="localhost:9092"
   export JWT_SECRET="your-secret"
   ```

3. Apply the DB schema once:

   ```bash
   psql "$DATABASE_URL" -f internal/database/schema.sql
   ```

4. Run the app:

   ```bash
   go run ./cmd
   ```

## Multi-instance (scale test)

To run multiple API replicas behind an Nginx load balancer:

```bash
docker compose -f docker/docker-compose.yml -f docker/docker-compose.scale.yml up -d --build
```

This starts 5 API replicas + Nginx LB on port 8080. Benchmark: `hey -z 30s -c 200 http://localhost:8080/todos?limit=100`.

## Stop

```bash
docker compose -f docker/docker-compose.yml down
```

To remove data volumes as well:

```bash
docker compose -f docker/docker-compose.yml down -v
```
