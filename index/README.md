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
            │   ├── kibana.yaml
            │   └── bootstrap-job.yaml
            └── stage/                # Stage environment (GKE)
                ├── elasticsearch.yaml
                ├── kibana.yaml
                ├── max-map-count-daemonset.yaml
                ├── es-service-user-setup-job.yaml
                ├── bootstrap-job.yaml
                └── templates/
                    ├── posts-index-template.yaml
                    └── posts-alias.yaml
```

## Infrastructure Overview

The indexing infrastructure uses **Elastic Cloud on Kubernetes (ECK)** to deploy and manage Elasticsearch clusters across different environments.

### Technology Stack
- **Elasticsearch 9.0.0**: Search engine and document store
- **Kibana 9.0.0**: Web UI for Elasticsearch management and visualization
- **ECK 3.1.0**: Kubernetes operator for Elasticsearch lifecycle management
- **Kubernetes**: Container orchestration (local: minikube, stage/prod: cloud)
- **Google Kubernetes Engine (GKE)**: Temporary cloud platform for stage testing (migrating to Azure)
- **Azure Kubernetes Service (AKS)**: Target production platform (future)

### Environment-Specific Configurations

#### Local Development
- **Elasticsearch**: Single-node cluster optimized for laptop resources
  - 2GB memory allocation with 1GB JVM heap
  - 5GB storage for testing
  - Security disabled for simple access
  - Resource requests: 2GB RAM, 500m CPU
- **Kibana**: Single instance web UI
  - 1GB memory allocation
  - No persistent storage needed
  - Security disabled (matching Elasticsearch)
  - Resource requests: 1GB RAM, 500m CPU

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

### 3. Deploy Kibana

```bash
# Deploy Kibana web UI
kubectl apply -f deploy/k8s/environments/local/kibana.yaml
```

### 4. Deploy Index Templates and Bootstrap Schema

```bash
# Deploy ConfigMaps for index templates and aliases
kubectl apply -f deploy/k8s/templates/

# Deploy bootstrap job to apply templates to Elasticsearch
kubectl apply -f deploy/k8s/environments/local/bootstrap-job.yaml
```

### 5. Monitor Deployment

```bash
# Check cluster status
kubectl get elasticsearch -n greenearth-local

# Check Kibana status
kubectl get kibana -n greenearth-local

# Check pod status
kubectl get pods -n greenearth-local

# Check bootstrap job status
kubectl get jobs -n greenearth-local

# View Elasticsearch logs if needed
kubectl logs greenearth-es-local-es-default-0 -n greenearth-local

# View Kibana logs if needed
kubectl logs -l kibana.k8s.elastic.co/name=greenearth-kibana-local -n greenearth-local

# View bootstrap job logs
kubectl logs -l job-name=elasticsearch-bootstrap -n greenearth-local
```

Wait for status to show:
- **Elasticsearch HEALTH**: `green`
- **Elasticsearch PHASE**: `Ready`
- **Kibana HEALTH**: `green`
- **Kibana PHASE**: `Ready`
- **Bootstrap Job COMPLETIONS**: `1/1`

## Testing Local Infrastructure

### 1. Access Kibana Web UI

```bash
# Port-forward to access Kibana locally
kubectl port-forward service/greenearth-kibana-local-kb-http 5601 -n greenearth-local
```

Then open your browser to: **http://localhost:5601**

Kibana provides:
- **Dev Tools Console**: Interactive API testing at `/app/dev_tools#/console`
- **Index Management**: View and manage indices at `/app/management/data/index_management`
- **Stack Management**: Configure settings at `/app/management`
- **Discover**: Explore your data at `/app/discover`

### 2. Access Elasticsearch API

```bash
# Port-forward to access locally (in a separate terminal)
kubectl port-forward service/greenearth-es-local-es-http 9200 -n greenearth-local
```

### 3. Test API Endpoints

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

### 4. Health Check Verification

A healthy local deployment should show:
- ✅ Elasticsearch cluster status: `green`
- ✅ Elasticsearch nodes: `1`
- ✅ Kibana status: `green`
- ✅ Kibana accessible at http://localhost:5601
- ✅ Bootstrap job completed: `1/1`
- ✅ Posts index template applied
- ✅ Posts alias configured: `posts` → `posts_v1`
- ✅ API responding with version `9.0.0`

## Cleanup

```bash
# Remove Kibana
kubectl delete kibana greenearth-kibana-local -n greenearth-local

# Remove Elasticsearch cluster
kubectl delete elasticsearch greenearth-es-local -n greenearth-local

# Remove namespace (this will delete all resources)
kubectl delete namespace greenearth-local
```

## Stage Environment Setup (GKE Autopilot)

The stage environment provides a production-like deployment on Google Kubernetes Engine (GKE) Autopilot with security enabled, proper memory configuration, and service account authentication.

**Note**: GKE is temporary for initial testing. Future deployments will use Azure Kubernetes Service (AKS).

### Prerequisites
- Google Cloud CLI (`gcloud`) installed and authenticated
- **Kubernetes Engine Admin** IAM role for ECK operator installation
  - Grant via GCP Console → IAM & Admin → Add role `roles/container.clusterAdmin`
- GKE Autopilot cluster (created automatically in deployment steps)
- kubectl installed locally

### Stage vs Local Configuration

**Security:**
- ✅ TLS enabled (self-signed certificates via ECK)
- ✅ Authentication required (native realm)
- ✅ Service account with RBAC for bootstrap operations
- ✅ Elastic superuser credentials in Kubernetes secrets

**Performance:**
- ✅ mmap enabled with DaemonSet for vm.max_map_count
- ✅ Single-node Elasticsearch: 1.5 CPU, 12GB memory (6GB JVM heap)
- ✅ Kibana: 0.5 CPU, 2GB memory
- ✅ Total resources: 2 CPU, 14GB memory

**Reliability:**
- ✅ 1 shard with 0 replicas (single node cluster)
- ✅ Persistent storage: 20GB

**Deployment:**
- ✅ GKE Autopilot (pay-per-pod, fully managed)
- ✅ Auto-scaling and auto-provisioning

### 1. Create GKE Autopilot Cluster

```bash
gcloud container clusters create-auto greenearth-stage-cluster \
  --region=us-east1 \
  --project=YOUR_PROJECT_ID
```

This automatically configures kubectl to use the new cluster.

**Verify connection:**
```bash
kubectl config current-context
# Should show: gke_PROJECT_ID_us-east1_greenearth-stage-cluster
```

### 2. Install ECK Operator

**Install CRDs:**
```bash
kubectl create -f https://download.elastic.co/downloads/eck/3.1.0/crds.yaml
```

**Install Operator:**
```bash
kubectl apply -f https://download.elastic.co/downloads/eck/3.1.0/operator.yaml
```

**Verify ECK is running:**
```bash
kubectl get pods -n elastic-system
# Wait for elastic-operator to show STATUS: Running
```

### 3. Create Namespace

```bash
kubectl create namespace greenearth-stage
```

### 4. Deploy DaemonSet for Virtual Memory

**IMPORTANT**: Must be deployed before Elasticsearch to set `vm.max_map_count`.

```bash
kubectl apply -f deploy/k8s/environments/stage/max-map-count-daemonset.yaml
```

Wait ~30 seconds for DaemonSet to complete.

### 5. Deploy Elasticsearch Cluster

```bash
kubectl apply -f deploy/k8s/environments/stage/elasticsearch.yaml
```

This deployment includes:
- Single-node cluster (master + data + ingest roles)
- mmap enabled (DaemonSet sets vm.max_map_count)
- TLS enabled with ECK-managed certificates
- Security enabled by default
- 1.5 CPU, 12GB memory, 20GB storage

### 6. Deploy Kibana

```bash
kubectl apply -f deploy/k8s/environments/stage/kibana.yaml
```

### 7. Wait for Elasticsearch and Kibana to be Ready

```bash
# Check Elasticsearch status
kubectl get elasticsearch -n greenearth-stage

# Check Kibana status
kubectl get kibana -n greenearth-stage

# Check pod status
kubectl get pods -n greenearth-stage
```

Wait for:
- **Elasticsearch HEALTH**: `green`
- **Elasticsearch PHASE**: `Ready`
- **Kibana HEALTH**: `green`
- **Kibana PHASE**: `Ready`

This may take 5-10 minutes for first deployment.

### 8. Deploy ConfigMaps for Templates

```bash
kubectl apply -f deploy/k8s/environments/stage/templates/
```

### 9. Create Service User Password Secret

**IMPORTANT**: This secret must be created before running the service user setup job.

```bash
# Generate a secure random password
ES_SERVICE_PASSWORD=$(openssl rand -base64 32)

# Create the Kubernetes secret
kubectl create secret generic es-service-user-password \
  --from-literal=password="$ES_SERVICE_PASSWORD" \
  -n greenearth-stage
```

**Note**: This secret is NOT committed to source control and must be created manually in each environment. Store the password securely (e.g., in a password manager or external secret management system).

### 10. Create Service Account User

```bash
kubectl apply -f deploy/k8s/environments/stage/es-service-user-setup-job.yaml
```

This job:
- Waits for Elasticsearch to be ready
- Creates `es_service_role` with index template and posts index permissions
- Creates `es-service-user` with the role
- Stores credentials in `es-service-user-secret` Kubernetes secret

Monitor the job:
```bash
kubectl get jobs -n greenearth-stage
kubectl logs -l job-name=es-service-user-setup -n greenearth-stage
```

### 11. Run Bootstrap Job

```bash
kubectl apply -f deploy/k8s/environments/stage/bootstrap-job.yaml
```

This job uses the `es-service-user` credentials to:
- Apply index templates
- Create initial `posts_v1` index
- Configure `posts` alias

Monitor the job:
```bash
kubectl get jobs -n greenearth-stage
kubectl logs -l job-name=elasticsearch-bootstrap -n greenearth-stage
```

## Accessing Stage Environment

### Access Kibana

```bash
# Port-forward Kibana
kubectl port-forward service/greenearth-kibana-stage-kb-http 5601 -n greenearth-stage
```

Browse to: **https://localhost:5601**

**Note**: You'll get a certificate warning (self-signed cert) - this is expected.

Get the elastic user password:
```bash
kubectl get secret greenearth-es-stage-es-elastic-user -o go-template='{{.data.elastic | base64decode}}' -n greenearth-stage
```

Login with:
- **Username**: `elastic`
- **Password**: (from command above)

### Access Elasticsearch API

```bash
# Port-forward Elasticsearch
kubectl port-forward service/greenearth-es-stage-es-http 9200 -n greenearth-stage
```

Get credentials:
```bash
# Elastic superuser (full access)
kubectl get secret greenearth-es-stage-es-elastic-user -o go-template='{{.data.elastic | base64decode}}' -n greenearth-stage

# Service user (limited to posts indices)
kubectl get secret es-service-user-secret -o go-template='{{.data.password | base64decode}}' -n greenearth-stage
```

Test API:
```bash
# Using elastic user
curl -k -u "elastic:PASSWORD" https://localhost:9200/

# Using service user
curl -k -u "es-service-user:PASSWORD" https://localhost:9200/_cluster/health
```

## Stage Environment Cleanup

```bash
# Remove Kibana
kubectl delete kibana greenearth-kibana-stage -n greenearth-stage

# Remove Elasticsearch
kubectl delete elasticsearch greenearth-es-stage -n greenearth-stage

# Remove all resources
kubectl delete namespace greenearth-stage
```

## Azure Deployment (Future)

The final production environment will be deployed on Azure Kubernetes Service (AKS). This section will be populated once the GKE stage environment is validated and Azure infrastructure is set up.

**Planned Changes:**
- Migrate from GKE Autopilot to AKS
- Update deployment scripts for Azure CLI
- Configure Azure-specific networking and security
- Adapt DaemonSet/init containers for AKS node configuration

## Production Deployment

(TODO - will be copied from stage/Azure once validated)

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