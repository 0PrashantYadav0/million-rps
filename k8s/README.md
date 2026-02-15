# Kubernetes deployment for million-rps

Deploy the API at scale for high RPS. Uses 10 replicas by default; HPA scales 5–50 based on CPU/memory.

## Prerequisites

- Kubernetes cluster (minikube, kind, EKS, GKE, etc.)
- PostgreSQL, Redis, Kafka (in-cluster or external)
- Docker image built: `docker build -t million-rps:latest .`

## Quick start

1. Create namespace and secrets (override with your DB/Redis/Kafka URLs):
   ```bash
   kubectl create namespace million-rps
   kubectl apply -f k8s/secret.yaml -n million-rps
   kubectl apply -f k8s/configmap.yaml -n million-rps
   ```

2. Update `k8s/secret.yaml` or create secret manually with real values:
   ```bash
   kubectl create secret generic million-rps-secret -n million-rps \
     --from-literal=DATABASE_URL='postgres://...' \
     --from-literal=REDIS_URL='redis://...' \
     --from-literal=KAFKA_BROKERS='kafka:9092' \
     --from-literal=JWT_SECRET='...'
   ```

3. Deploy API:
   ```bash
   kubectl apply -f k8s/deployment.yaml -n million-rps
   kubectl apply -f k8s/hpa.yaml -n million-rps
   ```

4. Expose externally (choose one):
   - LoadBalancer: `kubectl patch svc million-rps-api -n million-rps -p '{"spec":{"type":"LoadBalancer"}}'`
   - Ingress: `kubectl apply -f k8s/ingress.yaml -n million-rps` (requires Ingress controller)

5. Warm cache and benchmark:
   ```bash
   LB_IP=$(kubectl get svc million-rps-api -n million-rps -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   curl "http://$LB_IP/todos?limit=100" -o /dev/null
   hey -z 30s -c 200 "http://$LB_IP/todos?limit=100"
   ```

## Scaling for 1M RPS

- Start with 10–20 replicas; HPA will scale up under load.
- Use Redis Cluster or a high-memory Redis instance.
- Ensure load balancer supports high connection count (HAProxy, Envoy, or cloud LB).
- Use small payloads: `GET /todos?limit=100`.
