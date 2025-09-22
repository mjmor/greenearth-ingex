# Green Earth Ingex

A data ingestion and indexing system for BlueSky content. This project provides real-time streaming capabilities to capture, process, and search BlueSky posts and interactions through an ElasticSearch backend.

## System Architecture

### Data Ingestion
- **Real-time Streaming**: Go-based service connects to [BlueSky TurboStream websocket](https://www.graze.social/docs/graze-turbostream) for live event streaming
- **Event Processing**: Handles posts, reposts, likes, follows, and other BlueSky events
- **Runtime**: Deployed on [Google Cloud Run](https://cloud.google.com/run/docs) for automatic scaling and serverless execution
- **Client Library**: [go-elasticsearch](https://pkg.go.dev/github.com/elastic/go-elasticsearch/v9) for connecting to ES and data indexing

### (TODO) Search & Indexing
- **Search Engine**: [Elasticsearch](https://www.elastic.co/docs/solutions/search) for full-text search and analytics
- **Infrastructure**: [Elastic Cloud on Kubernetes (ECK)](https://www.elastic.co/docs/deploy-manage/deploy/cloud-on-k8s#eck-overview) running on [Google Kubernetes Engine (GKE)](https://cloud.google.com/kubernetes-engine/docs?hl=en_US)

## Development & Deployment

### Repository Structure

- `/ingest` - All code related to the Go-based ingestion service.
- `/index` - All code related to the Elastic Search index and query service.

### Continuous Integration (Github Actions)
- **Testing**: (TODO) Go test suites on all pull requests
- **Quality Assurance**: (TODO) Automated linting, formatting, and security checks

### Continuous Deployment
- **Infrastructure as Code**: (TODO) [Terraform](https://developer.hashicorp.com/terraform/intro/use-cases) manages all cloud resources
- **Multi-Environment Support**: (TODO) Separate staging and production deployments with proper promotion workflows
- **Service Orchestration**: Coordinated deployment of ingestion and indexing services in the correct dependency order
- **Multi-Cloud Management**:
  - **GCP**: Kubernetes clusters, Cloud Run services, networking
  - **Kubernetes**: Resource definitions, service meshes, and dependencies

## Getting Started

**TODO**

### Prerequisites
- Go 1.21+
- Docker
- Terraform
- kubectl
- Access to GCP accounts

### Local Development
```bash
# Clone the repository
git clone https://github.com/your-org/greenearth-ingex.git
cd greenearth-ingex

# Run tests
go test ./...
```

#### Testing with Live BlueSky Data

You can connect directly to the BlueSky TurboStream websocket to capture live data for testing the ingestion service:

```bash
# Install websocat if not already available
# brew install websocat  # macOS
# sudo apt install websocat  # Ubuntu

# Connect to websocket and capture all events
websocat "wss://api.graze.social/app/api/v1/turbostream/turbostream"

# Filter out posts to focus on other event types
websocat "wss://api.graze.social/app/api/v1/turbostream/turbostream" | grep -v '"collection": "app.bsky.feed.post"'

# Filter out multiple event types (posts, identity, account events)
websocat "wss://api.graze.social/app/api/v1/turbostream/turbostream" | grep -v -E '"collection": "app.bsky.feed.post"|"kind": "identity"|"kind": "account"'

# Save filtered data to file for testing
websocat "wss://api.graze.social/app/api/v1/turbostream/turbostream" | grep -v '"collection": "app.bsky.feed.post"' > test_data.jsonl

# Format specific lines for inspection
sed -n '2p' test_data.jsonl | python -m json.tool
```

This live data can be used to test different scenarios and edge cases in the ingestion pipeline.