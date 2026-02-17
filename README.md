## Million Request Per Second (focus: `GET /todos?limit=100`)

High‑throughput Todo API backend whose **only goal** is to push average requests/second as high as possible for **`GET /todos?limit=100`**.  
Reads are served from **Redis cache as raw JSON**, so the database is not on the hot path.

### Quick start: high‑RPS benchmark (Docker, easiest)

From the project root:

```bash
# 1. Start Postgres, Redis, Kafka, API replicas and Nginx load balancer
docker compose -f docker/docker-compose.yml -f docker/docker-compose.scale.yml up -d --build

# 2. Apply schema (once) and seed 10,000 todos
psql "postgres://app:appsecret@localhost:5432/million_rps?sslmode=disable" -f internal/database/schema.sql
go run ./scripts/seed

# 3. Warm the cache for the high‑RPS path
curl -s "http://localhost:8080/todos?limit=100" > /dev/null

# 4. Run the benchmark (internally uses /todos?limit=100)
./scripts/benchmark.sh http://localhost:8080 60 500
```

This starts multiple API instances behind Nginx, warms Redis, and then runs a load test using `hey` (if installed).  
Use higher concurrency (for example `60 1000`) on strong hardware to push toward **hundreds of thousands of req/sec**, and scale out the cluster to chase **1M req/sec**.

### Manual benchmark commands

After the stack is up and the cache is warm:

```bash
# hey (recommended if installed)
hey -z 60s -c 500 -t 120 -m GET "http://localhost:8080/todos?limit=100"

# autocannon (Node.js)
autocannon -c 500 -d 60 -t 120 -p 20 "http://localhost:8080/todos?limit=100"
```

Both commands:

- Hit **only** `GET /todos?limit=100`.
- Use a **small payload** for maximum RPS.
- Use a **long timeout** so client timeouts don’t cap throughput.

### What the backend does (simplified mental model)

- **Hot path:** `GET /todos` / `GET /todos?limit=N`
  - Cache‑first via Redis.
  - Returns **raw JSON bytes** from Redis when cache is warm (no marshal/unmarshal on hits).
  - No DB or Kafka in the read path on cache hit.
- **Writes (optional, not used in RPS tests):**
  - `POST /todos`, `PUT /todos/:id`, `DELETE /todos/:id` are JWT‑protected.
  - Writes are published to Kafka and applied by a worker.
  - Cache is invalidated after writes so future reads re‑fill it.

### Environment (only what you usually care about)

The app loads a **`.env`** file from the current working directory if present (copy `docker/.env.example` to `.env` for local runs), and also reads standard environment variables.

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | Server port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | required for seeding/writes |
| `DB_POOL_SIZE` | Max DB connections | `5000` |
| `REDIS_URL` | Redis URL | `redis://localhost:6379/0` |
| `REDIS_POOL_SIZE` | Redis connection pool | `5000` |
| `CACHE_TTL_SEC` | Cache TTL (seconds) | `300` |
| `KAFKA_BROKERS` | Comma‑separated brokers | `localhost:9092` |
| `KAFKA_TODO_TOPIC` | Topic for todo commands | `todo-commands` |
| `KAFKA_PARTITIONS` | Topic partitions (scale workers) | `32` |
| `WORKER_POOL_SIZE` | Number of worker goroutines | `128` |
| `JWT_SECRET` | Secret for JWT verification | required for auth writes |

For **pure read benchmarking**, the important parts are:

- Redis is reachable via `REDIS_URL`.
- Database has been seeded once (so Redis can be filled).
- The high‑RPS endpoint you hit is `GET /todos?limit=100` through the load balancer.

### K8s / larger‑scale runs (optional)

If you want to go beyond a single machine and push closer to **1M req/sec**, use the manifests in `k8s/`:

- `k8s/deployment.yaml` — API deployment (multiple replicas).
- `k8s/hpa.yaml` — HorizontalPodAutoscaler (auto‑scale API pods).
- `k8s/configmap.yaml` / `k8s/secret.yaml` — configuration and secrets.
- `k8s/ingress.yaml` — Ingress for external traffic.

On Kubernetes, the rough path to very high RPS is:

- Scale **API replicas** up (HPA or manual).
- Ensure **Redis** and **ingress/load balancer** are also scaled (so they are not the bottleneck).
- Run **load generators inside the cluster** against the `million-rps-api` service to remove external LB limits.

For simple local use, you can ignore K8s entirely and just use the Docker quick‑start above.

