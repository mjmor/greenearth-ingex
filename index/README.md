# Elasticsearch Index Infrastructure

This directory contains all infrastructure and deployment configurations for the Elasticsearch indexing layer of the Green Earth Ingex system.

## Directory Structure

```
index/
├── README.md                           # This file
└── deploy/                            # Deployment configurations
    ├── terraform/                     # Terraform IaC (TODO)
    └── k8s/                          # Kubernetes manifests
        └── environments/
            ├── local/                # Local development environment
            │   └── elasticsearch.yaml
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

### 3. Monitor Deployment

```bash
# Check cluster status
kubectl get elasticsearch -n greenearth-local

# Check pod status
kubectl get pods -n greenearth-local

# View logs if needed
kubectl logs greenearth-es-local-es-default-0 -n greenearth-local
```

Wait for status to show:
- **HEALTH**: `green`
- **PHASE**: `Ready`

## Testing Local Infrastructure

### 1. Access Elasticsearch API

```bash
# Port-forward to access locally
kubectl port-forward service/greenearth-es-local-es-http 9200 -n greenearth-local
```

### 2. Get Authentication Credentials

```bash
# Get the auto-generated elastic user password
kubectl get secret greenearth-es-local-es-elastic-user -o go-template='{{.data.elastic | base64decode}}' -n greenearth-local
```

### 3. Test API Endpoints

```bash
# Test basic connectivity
curl -u "elastic:YOUR_PASSWORD" -X GET "localhost:9200/"

# Check cluster health
curl -u "elastic:YOUR_PASSWORD" -X GET "localhost:9200/_cluster/health"
```

Expected responses:
- **Basic connectivity**: Elasticsearch version info and tagline
- **Cluster health**: `status: "green"`, `number_of_nodes: 1`

### 4. Health Check Verification

A healthy local deployment should show:
- ✅ Cluster status: `green`
- ✅ Number of nodes: `1`
- ✅ Number of data nodes: `1`
- ✅ Active shards: `3` (system indices)
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