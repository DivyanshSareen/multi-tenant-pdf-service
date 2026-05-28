# Multi-Tenant PDF Summary Ingestion Service

A microservice that accepts PDF files via REST API, summarizes them using an LLM, and stores results in dynamically-provisioned, tenant-isolated storage.

## Architecture

```
Client
  │
  ▼
Gin HTTP Server (Go)
  ├── POST /api/v1/upload
  │     ├── Provision tenant (if new)
  │     │     ├── PostgreSQL  → tenant registry record
  │     │     ├── MongoDB     → tenant_<name> database + documents collection
  │     │     └── MinIO       → tenant-<name> bucket
  │     ├── Extract PDF text  (pdftotext / raw fallback)
  │     ├── Summarize         (OpenAI GPT or Ollama)
  │     ├── Store PDF         → MinIO bucket
  │     └── Store metadata    → MongoDB documents collection
  │
  ├── GET  /api/v1/tenants
  ├── GET  /api/v1/tenants/:name
  ├── GET  /api/v1/tenants/:name/documents
  └── DELETE /api/v1/tenants/:name
```

| Component     | Role                                                         |
|---------------|--------------------------------------------------------------|
| PostgreSQL    | Master tenant registry (name, status, DB/bucket mapping)    |
| MongoDB       | Per-tenant document storage (one DB per tenant)             |
| MinIO         | Per-tenant object storage (one bucket per tenant)           |
| OpenAI/Ollama | PDF summarization                                           |

## Prerequisites

- Go 1.22+
- Docker
- [Kind](https://kind.sigs.k8s.io/)
- kubectl
- Helm 3
- Terraform (optional — for infra-as-code path)
- `poppler-utils` (`brew install poppler` / `apt install poppler-utils`) for local PDF extraction

## Quick Start

### 1. Create the Kind cluster

```bash
./scripts/setup-cluster.sh
```

### 2. Build and deploy

```bash
export OPENAI_API_KEY="sk-..."   # or leave empty to use Ollama
./scripts/deploy.sh
```

### 3. Test the API

```bash
./scripts/test-api.sh
```

### 4. Access databases locally (GUI tools)

Services run inside the cluster and are not exposed by default. Use `kubectl port-forward` to connect from DBeaver, Compass, or a browser:

```bash
# PostgreSQL → DBeaver (localhost:5432, user: admin, password: secretpass, db: master_registry)
kubectl port-forward -n pdf-service svc/pdf-service-postgres-service 5432:5432

# MongoDB → Compass (mongodb://localhost:27017)
kubectl port-forward -n pdf-service svc/pdf-service-mongodb-service 27017:27017

# MinIO console → http://localhost:9001 (user: minioadmin, password: minioadmin)
kubectl port-forward -n pdf-service svc/pdf-service-minio-service 9001:9001
```

> If a port is already in use on your machine, pick a different local port, e.g. `9091:9001`.

## Manual Deployment (raw K8s manifests)

```bash
kubectl apply -f deployments/k8s/namespace.yaml
kubectl apply -f deployments/k8s/postgres.yaml
kubectl apply -f deployments/k8s/mongodb.yaml
kubectl apply -f deployments/k8s/minio.yaml

# Edit deployments/k8s/app.yaml — set OPENAI_API_KEY in the Secret
kubectl apply -f deployments/k8s/app.yaml
```

## Terraform Path

```bash
cd deployments/terraform
terraform init
terraform apply          # provisions namespace + all StatefulSets/Services
```

Then deploy the app image separately via Helm or raw manifests.

## API Reference

All endpoints except `/health` and `/ready` require:
```
Authorization: Bearer <API_KEY>
```

### Upload a PDF

```
POST /api/v1/upload
Content-Type: multipart/form-data

Fields:
  file        PDF file
  tenantName  Alphanumeric string (letters, numbers, hyphens, underscores)
```

**Response `201`:**
```json
{
  "document_id":    "64f3a1...",
  "tenant_name":    "acme",
  "file_name":      "report.pdf",
  "summary":        "This document covers...",
  "file_reference": "tenant-acme/1234567_report.pdf",
  "page_count":     12,
  "is_new_tenant":  true
}
```

### List Tenants

```
GET /api/v1/tenants
```

### Get Tenant

```
GET /api/v1/tenants/:name
```

### Get Tenant Documents

```
GET /api/v1/tenants/:name/documents
```

### Delete Tenant

```
DELETE /api/v1/tenants/:name
```

Drops the MongoDB database, empties and deletes the MinIO bucket, soft-deletes the PostgreSQL record.

### Health / Readiness

```
GET /health   → {"status": "ok"}
GET /ready    → {"status": "ok", "components": {"postgres":"ok","mongodb":"ok","minio":"ok"}}
```

## Configuration

All settings are read from environment variables:

| Variable            | Default                     | Description                  |
|---------------------|-----------------------------|------------------------------|
| `SERVER_PORT`       | `8080`                      | HTTP listen port             |
| `POSTGRES_HOST`     | `localhost`                 | PostgreSQL host              |
| `POSTGRES_PORT`     | `5432`                      | PostgreSQL port              |
| `POSTGRES_USER`     | `admin`                     | PostgreSQL user              |
| `POSTGRES_PASSWORD` | `secretpass`                | PostgreSQL password          |
| `POSTGRES_DB`       | `master_registry`           | PostgreSQL database name     |
| `MONGO_URI`         | `mongodb://localhost:27017` | MongoDB connection URI       |
| `MINIO_ENDPOINT`    | `localhost:9000`            | MinIO endpoint               |
| `MINIO_ACCESS_KEY`  | `minioadmin`                | MinIO access key             |
| `MINIO_SECRET_KEY`  | `minioadmin`                | MinIO secret key             |
| `LLM_PROVIDER`      | `openai`                    | `openai` or `ollama`         |
| `OPENAI_API_KEY`    | _(empty)_                   | OpenAI API key               |
| `OPENAI_MODEL`      | `gpt-4o-mini`               | OpenAI model name            |
| `OLLAMA_BASE_URL`   | `http://localhost:11434`    | Ollama server URL            |
| `OLLAMA_MODEL`      | `llama3`                    | Ollama model name            |
| `API_KEY`           | `changeme`                  | Bearer token for API auth    |

## Using Ollama (no OpenAI key)

```bash
# Install and run Ollama locally
ollama pull llama3
ollama serve

# Set env before deploying
export LLM_PROVIDER=ollama
export OLLAMA_BASE_URL=http://host.docker.internal:11434
```

## Project Structure

```
multi-tenant-pdf-service/
├── cmd/server/main.go                # Entrypoint — wires all dependencies
├── internal/
│   ├── api/                          # Gin handlers and router
│   ├── config/config.go              # Env-based config
│   ├── database/                     # PostgreSQL + MongoDB clients
│   ├── middleware/auth.go            # Bearer token auth
│   ├── models/models.go              # Shared data structures
│   └── services/
│       ├── ai/summarizer.go          # OpenAI + Ollama LLM interface
│       ├── pdf/extractor.go          # PDF text extraction
│       ├── storage/minio.go          # MinIO object storage
│       └── tenant/manager.go         # Tenant lifecycle orchestration
├── deployments/
│   ├── docker/Dockerfile
│   ├── helm/pdf-service/             # Helm chart
│   ├── k8s/                          # Raw Kubernetes manifests
│   └── terraform/                    # Terraform infra-as-code
└── scripts/
    ├── setup-cluster.sh
    ├── deploy.sh
    └── test-api.sh
```
