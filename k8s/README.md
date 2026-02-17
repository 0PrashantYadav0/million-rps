## Kubernetes deployment for million-rps

Run **everything inside the cluster** – Postgres, Redis, Kafka, API, and Service/Ingress – so you can push RPS without relying on Docker Compose.

### Prerequisites

- Kubernetes cluster (minikube, kind, EKS, GKE, etc.).
- Docker image built locally or pushed to a registry (adjust image name if needed):

```bash
docker build -t million-rps:latest .
```

### Quick start: full stack in K8s

```bash
# 1. Create namespace
kubectl create namespace million-rps

# 2. Deploy in-cluster Postgres, Redis, Kafka
kubectl apply -f k8s/deps.yaml -n million-rps

# 3. Apply app config + secrets (DB/Redis/Kafka URLs already point at in-cluster services)
kubectl apply -f k8s/secret.yaml -n million-rps
kubectl apply -f k8s/configmap.yaml -n million-rps

# 4. Deploy API + HPA
kubectl apply -f k8s/deployment.yaml -n million-rps
kubectl apply -f k8s/hpa.yaml -n million-rps

# 5. Expose externally (choose one)
#    a) LoadBalancer Service
kubectl patch svc million-rps-api -n million-rps -p '{"spec":{"type":"LoadBalancer"}}'

#    b) Ingress (requires ingress controller in the cluster)
kubectl apply -f k8s/ingress.yaml -n million-rps
```

### Warm cache and benchmark (for high RPS)

Using a LoadBalancer service:

```bash
LB_IP=$(kubectl get svc million-rps-api -n million-rps -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# Warm Redis cache on the small-payload path
curl -s "http://$LB_IP/todos?limit=100" > /dev/null

# Run high-RPS benchmark
hey -z 60s -c 500 -t 120 "http://$LB_IP/todos?limit=100"
```

For even higher RPS, run your load generator **inside the cluster** (so the cloud/network LB is not the bottleneck) and hit the `million-rps-api` service directly.

