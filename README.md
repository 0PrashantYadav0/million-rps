# Million Request Per Seconds

High-throughput Todo API backend: public `GET /todos`, JWT-protected create/update/delete, with Redis cache and Kafka queue so the database is not a bottleneck.

![go](https://github.com/user-attachments/assets/7a2c6a9d-2278-4667-8d0b-b5a5f3de0020)

## Features

- **Public**: `GET /todos` — returns all todos (cache-first via Redis, then DB).
- **JWT auth**: `POST /todos`, `PUT /todos/:id`, `DELETE /todos/:id` — require `Authorization: Bearer <token>`.
- **Scalable**: Writes are published to Kafka and processed by a worker; API returns 202 Accepted immediately.
- **Cache**: Redis caches the todo list; cache invalidated on any write.
- **Config**: All tuning via environment variables.

## Prerequisites

- Go 1.21+
- PostgreSQL, Redis, Kafka (or use the [Docker setup](#local-development-with-docker) below)

## Environment

The app loads a **`.env`** file from the current working directory at startup (if present). You can copy `docker/.env.example` to `.env` and set `DATABASE_URL`, `JWT_SECRET`, etc. Otherwise set environment variables in your shell or deployment.

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | required |
| `DB_POOL_SIZE` | Max DB connections | `100` |
| `REDIS_URL` | Redis URL | `redis://localhost:6379/0` |
| `REDIS_POOL_SIZE` | Redis connection pool | `500` |
| `CACHE_TTL_SEC` | Cache TTL (seconds) | `300` |
| `KAFKA_BROKERS` | Comma-separated brokers | `localhost:9092` |
| `KAFKA_TODO_TOPIC` | Topic for todo commands | `todo-commands` |
| `KAFKA_PARTITIONS` | Topic partitions (scale) | `16` |
| `JWT_SECRET` | Secret for JWT verification | required for auth |

## Database schema

Run once against your PostgreSQL:

```bash
psql "$DATABASE_URL" -f internal/database/schema.sql
```

## Seed data (10,000 todos)

From the project root (with `.env` or `DATABASE_URL` set):

```bash
go run ./scripts/seed
```

This inserts 10,000 todos with `user_id=seed-user`. Useful for load testing or local dev.

## Local development with Docker

Dependencies (PostgreSQL, Redis, Kafka) can be run locally with Docker Compose. All files live in the **`docker/`** folder.

| File | Purpose |
|------|---------|
| `docker/docker-compose.yml` | Starts Postgres, Redis, and Kafka (KRaft) |
| `docker/.env.example` | Example env vars to connect the app to these services |
| `docker/README.md` | Full instructions for the Docker setup |

**Quick start:**

```bash
# 1. Start PostgreSQL, Redis, and Kafka
docker compose -f docker/docker-compose.yml up -d

# 2. Set env (or copy docker/.env.example to .env and source it)
export DATABASE_URL="postgres://app:appsecret@localhost:5432/million_rps?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export KAFKA_BROKERS="localhost:9092"
export JWT_SECRET="your-secret"

# 3. Apply schema once
psql "$DATABASE_URL" -f internal/database/schema.sql

# 4. Run the app
go run ./cmd
```

Stop dependencies: `docker compose -f docker/docker-compose.yml down`. See **`docker/README.md`** for more details.

## Run

```bash
go run ./cmd
```

Or build and run:

```bash
go build -o app ./cmd && ./app
```

## API

- **GET /todos** (public) — List all todos. Served from Redis as raw JSON when warm (max throughput).
- **GET /todos?limit=N** (public) — List first N todos (pagination). Smaller payload = higher RPS; use for load testing (e.g. `?limit=100`).
- **POST /todos** (auth) — Body: `{"title":"...","description":"..."}`. Returns `202` with `id` (queued).
- **PUT /todos/:id** (auth) — Body: `{"title":"...","description":"...","completed":true}`. Returns `202`.
- **DELETE /todos/:id** (auth) — Returns `202`.

## Benchmarking (10k → 100k → 1M RPS)

**1. Warm the cache** (one request so later requests are served from Redis):
```bash
curl -s http://localhost:8080/todos?limit=100 > /dev/null
# or for full list:
curl -s http://localhost:8080/todos > /dev/null
```

**2. Target ~10k–100k RPS on one machine** — use a **small payload** so Redis and network aren’t the bottleneck:
```bash
# Small response (~100 items) = high RPS (aim for 10k–50k+ req/sec on a good box)
hey -z 30s -c 200 -m GET "http://localhost:8080/todos?limit=100"
# or autocannon:
autocannon -c 200 -d 30 -p 20 "http://localhost:8080/todos?limit=100"
```

**3. Full list** (large JSON, ~10k items) — expect lower RPS due to payload size; warm cache first:
```bash
hey -z 30s -c 100 -m GET http://localhost:8080/todos
```

**4. Scale toward 100k–1M RPS** — run multiple app instances behind a load balancer, and scale Redis (cluster or more memory/network). Each instance can do tens of thousands of small-payload req/s when cache is warm.

**Using `hey`:** `go install github.com/rakyll/hey@latest`  
**Using `autocannon`:** If you see `libsimdjson` error, run `brew reinstall simdjson && brew reinstall node`.

## Scaling to high RPS (10k → 100k → 1M)

### Health endpoints (for load balancers & K8s)

- **GET /health** — Liveness: returns 200 if process is alive.
- **GET /ready** — Readiness: returns 200 if DB and Redis are reachable.

### Docker Compose multi-instance (local scale test)

Run 5 API replicas behind Nginx load balancer:

```bash
# 1. Start deps + API replicas + LB
docker compose -f docker/docker-compose.yml -f docker/docker-compose.scale.yml up -d --build

# 2. Run schema (once)
psql "postgres://app:appsecret@localhost:5432/million_rps?sslmode=disable" -f internal/database/schema.sql

# 3. Seed and benchmark
go run ./scripts/seed
./scripts/benchmark.sh http://localhost:8080 30 200
```

### Kubernetes (production scale)

See **`k8s/README.md`** for full instructions. Summary:

```bash
# Build image
docker build -t million-rps:latest .

# Deploy (requires K8s cluster + Postgres/Redis/Kafka)
kubectl create namespace million-rps
kubectl apply -f k8s/secret.yaml -f k8s/configmap.yaml -n million-rps
kubectl apply -f k8s/deployment.yaml -f k8s/hpa.yaml -n million-rps
kubectl patch svc million-rps-api -n million-rps -p '{"spec":{"type":"LoadBalancer"}}'

# Benchmark (replace LB_IP with your load balancer)
hey -z 30s -c 200 "http://<LB_IP>/todos?limit=100"
```

- **Deployment:** 10 replicas by default.
- **HPA:** Auto-scales 5–50 pods on CPU/memory.
- **Ingress:** Optional; see `k8s/ingress.yaml` for nginx/ALB.

### Path to 1M RPS

| Layer | Approach |
|-------|----------|
| **API** | 15–25 replicas (K8s HPA or fixed); each ~50k–100k RPS for small payloads |
| **Load balancer** | HAProxy, Envoy, or cloud LB; ensure high connection capacity |
| **Redis** | Redis Cluster or high-memory instance; 10+ Gbps network |
| **Payload** | Use `GET /todos?limit=100` for maximum RPS |

### Benchmark script

```bash
./scripts/benchmark.sh [base_url] [duration_sec] [concurrency]
# Example:
./scripts/benchmark.sh http://localhost:8080 60 500
```
