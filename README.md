# Million Request Per Seconds

High-throughput Todo API backend: public `GET /todos`, JWT-protected create/update/delete, with Redis cache and Kafka queue so the database is not a bottleneck.

![Uploading go.gif…]()

## Features

- **Public**: `GET /todos` — returns all todos (cache-first via Redis, then DB).
- **JWT auth**: `POST /todos`, `PUT /todos/:id`, `DELETE /todos/:id` — require `Authorization: Bearer <token>`.
- **Scalable**: Writes are published to Kafka and processed by a worker; API returns 202 Accepted immediately.
- **Cache**: Redis caches the todo list; cache invalidated on any write.
- **Config**: All tuning via environment variables.

## Prerequisites

- Go 1.21+
- PostgreSQL
- Redis
- Kafka

## Environment

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

## Run

```bash
go run ./cmd
```

Or build and run:

```bash
go build -o app ./cmd && ./app
```

## API

- **GET /todos** (public) — List all todos. Served from Redis when warm.
- **POST /todos** (auth) — Body: `{"title":"...","description":"..."}`. Returns `202` with `id` (queued).
- **PUT /todos/:id** (auth) — Body: `{"title":"...","description":"...","completed":true}`. Returns `202`.
- **DELETE /todos/:id** (auth) — Returns `202`.

## Scaling to high RPS

- Run multiple API instances behind a load balancer.
- Run multiple worker instances (same Kafka consumer group); partitions are shared across instances.
- Tune `DB_POOL_SIZE`, `REDIS_POOL_SIZE`, and Kafka partitions per load.
