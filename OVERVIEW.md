## Million RPS Todo API – External Technical Overview

This document explains **what we built**, **why we built it this way**, and **how we achieved ~500k requests per second** on a single machine for a simple read API.

---

## 1. High‑Level Summary

We built a **Todo API** whose core purpose is to push HTTP throughput as high as possible on a single box:

- Public read endpoint: `GET /todos?limit=N`
- Authenticated write endpoints: `POST/PUT/DELETE /todos`
- Achieved **~500k requests per second** for:

  ```bash
  autocannon -c 2000 -d 60 -p 80 "http://localhost:8080/todos?limit=1"
  ```

  on an Apple M5 machine.

Key ideas:

- Reads are **fully cache‑backed** and served from memory (Redis).
- Writes are **asynchronous**, decoupled from reads via Kafka and a worker.
- The API is **stateless and horizontally scalable**, designed to be run behind a load balancer or Kubernetes Service.

---

## 2. Architecture Layers and Technology Choices

We deliberately used a small set of well‑known components, each playing a specific role.

### 2.1 Client / Load Generation

- **Tool**: `autocannon` (Node.js HTTP benchmark tool).
- **Why**:
  - Supports **high concurrency** (`-c 2000`).
  - Supports **HTTP pipelining** (`-p 80`) to minimize per‑request connection overhead.
  - Produces detailed metrics (latency percentiles, RPS, bandwidth).

### 2.2 Load Balancer Layer

- **Technology**: Nginx (Docker container).
- **Role**:
  - Accepts all external traffic on `localhost:8080`.
  - Distributes requests across multiple API instances using a **least‑connections** strategy.
  - Keeps persistent connections and reuses them (`keepalive`).

- **Why Nginx**:
  - Widely used, well understood.
  - Efficient at handling many concurrent connections.
  - Easy to configure inside Docker and Kubernetes.

> In our tests, Nginx on the Apple M5 machine became the **first bottleneck**, reaching 100% CPU before the API instances did. This is an important finding: the load balancer, not the application code, capped us around 500k req/s on a single box.

### 2.3 Application Layer (Go API)

- **Technology**: Go + Gin web framework.
- **Role**:
  - Implements all HTTP endpoints.
  - For reads: orchestrates Redis cache, falling back to Postgres only on cache misses.
  - For writes: validates input and publishes messages to Kafka (no direct DB writes).

- **Why Go + Gin**:
  - Go’s runtime handles **massive concurrency** with lightweight goroutines and channels.
  - Gin is a minimal overhead HTTP framework, with explicit control over middleware and logging.
  - Mature libraries for Redis, Kafka, Postgres, and JWT.

### 2.4 Caching Layer

- **Technology**: Redis.
- **Role**:
  - Stores serialized JSON responses for Todo lists and slices:
    - `todos:limit:1` (used for the highest‑RPS benchmark).
    - `todos:limit:100`, etc.
  - On cache hit, the API returns the **exact bytes** from Redis as the HTTP response body.

- **Why Redis**:
  - In‑memory data store with very low latency.
  - Perfect fit for key‑value caching of pre‑computed JSON.
  - Supports high QPS with proper configuration and connection pooling.

### 2.5 Database Layer

- **Technology**: PostgreSQL.
- **Role**:
  - Single source of truth for all Todo data.
  - Handles:
    - Initial data seeding (10k initial todos).
    - Cache warm‑up queries on misses.
    - Write operations applied by the worker.

- **Why Postgres**:
  - Reliable relational database.
  - Strong tooling and ecosystem.
  - Familiar model for most backend teams.

### 2.6 Messaging / Asynchronous Processing

- **Technology**: Kafka + Worker.
- **Role**:
  - `POST/PUT/DELETE /todos` publish a **TodoCommand** event into Kafka.
  - Worker processes commands:
    - Updates the Postgres database.
    - Invalidates relevant Redis cache keys.

- **Why Kafka**:
  - Decouples write traffic from read traffic.
  - Ensures back‑pressure and durability for write operations.
  - Allows write throughput to scale independently via:
    - Multiple partitions.
    - Multiple worker consumers.

---

## 3. Request Lifecycle – How a Single Request Is Handled

This section walks through the life of individual requests at a conceptual level.

### 3.1 High‑volume reads: `GET /todos?limit=1`

1. **Client**  
   - `autocannon` sends HTTP GET requests with:
     - `-c 2000` (concurrent connections).
     - `-p 80` (up to 80 in‑flight requests per connection).
   - Total effective in‑flight operations ≈ 160,000 (2,000 × 80).

2. **Load Balancer (Nginx)**  
   - Receives all incoming requests on port `8080`.
   - Uses **least‑connections** to distribute requests across API instances.
   - Maintains persistent connections to each API, reducing TCP overhead.

3. **API Instance (Go + Gin)**  
   - Receives the HTTP request from Nginx.
   - Parses the query parameter `limit=1`.
   - For this endpoint, executes the following logic:
     - Generate a Redis key: `todos:limit:1`.
     - Try to **read bytes directly from Redis** for that key.
       - On cache hit:
         - The JSON bytes are sent back as the HTTP response body.
         - No database, no Kafka, minimal CPU per request.
       - On cache miss (happens once after startup or invalidation):
         - Query Postgres to fetch the first todo.
         - Serialize to JSON.
         - Return the response to the client.
         - Fire an async Redis `SET` to store the bytes for future hits.

4. **Response to Client**  
   - Each response is ~400 bytes (JSON + headers).
   - With pipelining, multiple responses flow back per connection without waiting on each other’s round‑trip.

The critical point: **once the cache is warm**, the average request for `GET /todos?limit=1` never touches the database and involves almost no allocation or logging in application code.

### 3.2 Authenticated writes: `POST /todos`

1. **Client call**:

   ```bash
   curl -X POST "http://localhost:8080/todos" \
     -H "Authorization: Bearer <JWT>" \
     -H "Content-Type: application/json" \
     -d '{"title":"example","description":"..." }'
   ```

2. **Load Balancer**:
   - Same as reads; forwards to an API instance.

3. **API Instance**:
   - Validates JWT (HS256 with shared secret).
   - Parses JSON payload.
   - Builds a `TodoCommand` message.
   - Publishes the command to Kafka.
   - Immediately returns `202 Accepted` to the client.

4. **Worker + Database + Cache**:
   - Worker consumes the command from Kafka.
   - Applies the corresponding operation in Postgres (insert/update/delete).
   - Invalidates related Redis keys (e.g., `todos:all`, `todos:limit:*`).
   - Future reads repopulate the cache on demand.

This design ensures **writes never block read performance**, even under heavy write load.

---

## 4. How We Reached ~500k Requests Per Second

### 4.1 Benchmark Configuration

The key benchmark command:

```bash
autocannon -c 2000 -d 60 -p 80 "http://localhost:8080/todos?limit=1"
```

On an Apple M5 machine (10‑core CPU), this produced:

- **Average Req/Sec**: ~500,023.45
- **Total Requests**: ~30,001,000 (30,001k)
- **Average Latency**: ~320 ms
- **Total Read**: ~11.45 GB

### 4.2 Why These Flags and Endpoint

- **`-c 2000` (concurrency)**  
  Keeps Go’s runtime busy across cores; 2,000 simultaneous connections create enough scheduling pressure without collapsing the system.

- **`-p 80` (HTTP pipelining)**  
  Allows up to 80 outstanding requests per connection, drastically reducing:
  - Connection setup/teardown overhead.
  - TCP and TLS handshakes.

- **`?limit=1` (small payload)**  
  Minimizes:
  - JSON size (~400 bytes per response).
  - CPU required for serialization/deserialization.
  - Network and memory bandwidth per request.

### 4.3 Validating the Numbers

**Throughput vs concurrency vs latency**

- Effective concurrency ≈ 2,000 connections × 80 pipelined = 160,000 in‑flight requests.
- Average latency ≈ 320 ms (0.320 s).
- Using the standard relationship:

  \[
  \text{Throughput} \approx \frac{\text{Concurrency}}{\text{Average Latency}}
  \]

  we get:

  \[
  \text{Throughput} \approx \frac{160{,}000}{0.320} \approx 500{,}000\ \text{req/s}
  \]

This closely matches the observed **~500,023 req/s**.

**Total data read**

- ~30M responses × ~400 bytes per response ≈ 12 GB.
- Reported total read ≈ 11.45 GB, which matches once you account for:
  - Byte vs MiB vs MB conversions.
  - Protocol overhead and rounding in reporting.

Conclusion: The reported metrics are **internally consistent** and align with the benchmark configuration, payload size, and hardware capabilities.

### 4.4 Where the Bottleneck Is: Load Balancer vs API

During testing:

- The **Nginx load balancer process** was observed to max out **CPU usage first**, while API instances still had headroom.
- This means:
  - The **load balancer, not the application code, is the primary limiter** at ~500k req/s on a single Apple M5 box.

To push beyond 500k req/s toward **1M+ req/s**, we would need:

- **More powerful hardware**:
  - More cores, higher frequency, or multiple machines.
- **Multiple load balancers**:
  - Several Nginx/ingress instances “in front” of the API.
  - Or a cloud load balancer designed for millions of concurrent connections.
- **Distributed load generation**:
  - Several `autocannon` instances from different machines to avoid saturating a single client host.

The **current architecture** (cache‑first reads, async writes, stateless API) is intended to scale horizontally once these infrastructure constraints are addressed.

In addition to the read benchmark, our **authenticated routes** (`POST/PUT/DELETE /todos` with JWT) sustain **~20k–50k req/s average** at moderate concurrency (around 200 connections), with p50 latencies in the tens of milliseconds and very low error rates. These endpoints publish commands to Kafka and return `202 Accepted` quickly, so the write path remains responsive and does not interfere with the high‑throughput cached read path.

---

## 5. Infrastructure Diagram (Mermaid)

The diagram below illustrates the main components and how requests flow through the system.

```mermaid
graph LR
  subgraph Clients
    AC[autocannon<br/>high concurrency & pipelining]
  end

  subgraph Edge[Load Balancer]
    LB[Nginx<br/>localhost:8080]
  end

  subgraph Apps[Stateless API Layer]
    API1[API Instance 1]
    API2[API Instance 2]
    APIN[API Instance N]
  end

  subgraph Data[Data & Queue]
    R[(Redis Cache)]
    K[(Kafka)]
    W[Worker(s)]
    DB[(Postgres)]
  end

  AC --> LB
  LB --> API1
  LB --> API2
  LB --> APIN

  %% Read path
  API1 -->|GET /todos?limit=1<br/>read/write| R
  API2 -->|GET /todos?limit=1<br/>read/write| R
  APIN -->|GET /todos?limit=1<br/>read/write| R

  %% Write path
  API1 -->|TodoCommand| K
  API2 -->|TodoCommand| K
  APIN -->|TodoCommand| K

  K --> W
  W -->|writes| DB
  W -->|cache invalidation| R
```

**Key points shown:**

- Clients only talk to **Nginx**, which is why it becomes the first CPU bottleneck.
- Stateless API instances can be scaled out horizontally as needed.
- Redis is on the hot path for reads; Postgres is only used on cache miss or by the worker.
- Kafka + Workers form a separate path for writes, decoupled from the read path.

---

## 6. Takeaways

- By combining a **cache‑first design**, **async writes**, and a **stateless Go API** behind a load balancer, we reached **~500k req/s** on a single Apple M5 machine.
- The limiting factor at this point is **not** the business logic or persistence, but the **load balancer and hardware**.
- With:
  - Stronger machines,
  - Multiple LBs,
  - And/or distributed load generation,
  the same design can be scaled further toward the **1M req/s** mark and beyond.
