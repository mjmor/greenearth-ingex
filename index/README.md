# Elasticsearch Index Infrastructure

This directory contains all infrastructure and deployment configurations for the Elasticsearch indexing layer of the Green Earth Ingex system.

## Directory Structure

```
index/
├── README.md                           # This file
└── deploy/                            # Deployment configurations
    ├── terraform/                     # Terraform IaC (TODO)
    └── k8s/                          # Kubernetes manifests
        ├── templates/                # Index templates and aliases
        │   ├── posts-index-template.yaml
        │   └── posts-alias.yaml
        └── environments/
            ├── local/                # Local development environment
            │   ├── elasticsearch.yaml
            │   └── bootstrap-job.yaml
            ├── staging/              # Staging environment (TODO)
            └── production/           # Production environment
                └── elasticsearch.yaml
```

## Infrastructure Overview

The indexing infrastructure uses **Elastic Cloud on Kubernetes (ECK)** to deploy and manage Elasticsearch clusters across different environments.

### Technology Stack
- **Elasticsearch 9.0.0**: Search engine and document store
- **ECK 3.1.0**: Kubernetes operator for Elasticsearch lifecycle management
- **Kubernetes**: Container orchestration (local: minikube, cloud: AKS)
- **Azure Kubernetes Service (AKS)**: Production Kubernetes platform

### Environment-Specific Configurations

#### Local Development
- **Single-node cluster** optimized for laptop resources
- **2GB memory allocation** with 1GB JVM heap
- **5GB storage** for testing
- **Security disabled** (TLS off) for simple access
- **Resource requests**: 2GB RAM, 500m CPU

#### Production
- **Multi-node cluster** (1 master + 2 data nodes) - TODO: upgrade cluster sizes depending on scale
- **Higher resource allocation** for production workloads
- **50GB storage per data node**
- **TODO**: Proper virtual memory configuration via init containers

## Local Development Setup

### Prerequisites
- Docker
- minikube or other local Kubernetes cluster
- kubectl
- ECK operator installed

### 1. Install ECK Operator

```bash
# Install ECK 3.1.0
kubectl apply -f https://download.elastic.co/downloads/eck/3.1.0/crds.yaml
kubectl apply -f https://download.elastic.co/downloads/eck/3.1.0/operator.yaml
```

### 2. Deploy Local Elasticsearch

```bash
# Create namespace
kubectl create namespace greenearth-local

# Deploy Elasticsearch cluster
kubectl apply -f deploy/k8s/environments/local/elasticsearch.yaml
```

### 3. Deploy Index Templates and Bootstrap Schema

```bash
# Deploy ConfigMaps for index templates and aliases
kubectl apply -f deploy/k8s/templates/

# Deploy bootstrap job to apply templates to Elasticsearch
kubectl apply -f deploy/k8s/environments/local/bootstrap-job.yaml
```

### 4. Monitor Deployment

```bash
# Check cluster status
kubectl get elasticsearch -n greenearth-local

# Check pod status
kubectl get pods -n greenearth-local

# Check bootstrap job status
kubectl get jobs -n greenearth-local

# View Elasticsearch logs if needed
kubectl logs greenearth-es-local-es-default-0 -n greenearth-local

# View bootstrap job logs
kubectl logs -l job-name=elasticsearch-bootstrap -n greenearth-local
```

Wait for status to show:
- **Elasticsearch HEALTH**: `green`
- **Elasticsearch PHASE**: `Ready`
- **Bootstrap Job COMPLETIONS**: `1/1`

## Testing Local Infrastructure

### 1. Access Elasticsearch API

```bash
# Port-forward to access locally
kubectl port-forward service/greenearth-es-local-es-http 9200 -n greenearth-local
```

### 2. Test API Endpoints

```bash
# Test basic connectivity (no authentication required in local)
curl -X GET "localhost:9200/"

# Check cluster health
curl -X GET "localhost:9200/_cluster/health"

# Verify index templates and aliases are applied
curl -X GET "localhost:9200/_index_template/posts_template"
curl -X GET "localhost:9200/_alias/posts"
```

Expected responses:
- **Basic connectivity**: Elasticsearch version info and tagline
- **Cluster health**: `status: "green"`, `number_of_nodes: 1`
- **Index template**: Shows posts_template configuration with schema
- **Alias**: Shows `posts` alias pointing to `posts_v1` index

### 3. Health Check Verification

A healthy local deployment should show:
- ✅ Cluster status: `green`
- ✅ Number of nodes: `1`
- ✅ Number of data nodes: `1`
- ✅ Bootstrap job completed: `1/1`
- ✅ Posts index template applied
- ✅ Posts alias configured: `posts` → `posts_v1`
- ✅ API responding with version `9.0.0`

## Cleanup

```bash
# Remove Elasticsearch cluster
kubectl delete elasticsearch greenearth-es-local -n greenearth-local

# Remove namespace
kubectl delete namespace greenearth-local
```

## Production Deployment

(TODO)

## Troubleshooting

### Common Issues

**Pod in CrashLoopBackOff**
- Check logs: `kubectl logs POD_NAME -n greenearth-local`
- Common causes: Memory limits, configuration conflicts

**OOMKilled Errors**
- Reduce JVM heap size in manifest
- Increase memory limits if resources allow

**Configuration Conflicts**
- Avoid mixing `discovery.type: single-node` with ECK auto-configuration
- Let ECK handle single-node setup automatically

**Port-forward Issues**
- Ensure service exists: `kubectl get svc -n greenearth-local`
- Check if port 9200 is already in use locally