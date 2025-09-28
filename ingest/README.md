# Ingest Service

Go-based data ingestion service that connects to BlueSky's TurboStream websocket to capture real-time events and index them in Elasticsearch for the Green Earth Ingex system.

## Overview

The ingest service is a core component that handles real-time streaming of BlueSky content through their TurboStream websocket API, then indexes content into Elasticsearch for search.

## Features

- **Real-time Event Streaming**: Connects to [BlueSky TurboStream websocket](https://www.graze.social/docs/graze-turbostream) for live event capture
- **Event Processing**: Handles posts, reposts, likes, follows, and other BlueSky interactions
- **Elasticsearch Integration**: Uses [go-elasticsearch](https://pkg.go.dev/github.com/elastic/go-elasticsearch/v9) for data indexing
- **Graceful Shutdown**: Proper SIGTERM handling and context cancellation
- **Worker Thread Architecture**: Separate workers for WebSocket reading and Elasticsearch indexing
- **Message Processing Pipeline**: Channel-based event processing with backpressure handling
- **Comprehensive Testing**: Full test coverage for all components

## Architecture

```
BlueSky TurboStream → WebSocket Client → Message Pipeline → Elasticsearch Client → Elasticsearch
                              ↓                    ↓                      ↓
                         Worker Threads    Channel Processing      Bulk Operations
```

### Core Components

- **WebSocket Client**: Manages connection to TurboStream with reconnection logic
- **Message Pipeline**: Processes events through Go channels with worker pools
- **Elasticsearch Client**: Handles indexing with bulk operations and retry logic
- **Configuration Management**: Environment-based config with validation
- **Logger Interface**: Structured logging with multiple implementations

## Local Development

### Prerequisites
- Go 1.21+
- Access to Elasticsearch cluster (see [../index/README.md](../index/README.md) for local setup)

### Quick Start

```bash
# Navigate to ingest directory
cd ingest

# Install dependencies
go mod download

# Run tests
go test -v

# Build the service
go build -o ingest

# Run locally (requires environment variables)
./ingest
```

### Development Commands

```bash
# Run with live reloading during development
go run main.go

# Run specific tests
go test ./... -v

# Build for production
go build -o ingest

# Run linting (if available)
# golangci-lint run
```

## Testing with Live BlueSky Data

You can test the ingestion pipeline using live BlueSky data from their TurboStream:

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

## Configuration

### Environment Variables

- `ELASTICSEARCH_URL` - Elasticsearch cluster endpoint (required)
- `TURBOSTREAM_URL` - BlueSky TurboStream websocket URL (default: wss://api.graze.social/app/api/v1/turbostream/turbostream)
- `PORT` - Service HTTP port (default: 8080)
- `LOG_LEVEL` - Logging level (default: info)

### Example Configuration

```bash
export ELASTICSEARCH_URL="http://localhost:9200"
export TURBOSTREAM_URL="wss://api.graze.social/app/api/v1/turbostream/turbostream"
export PORT="8080"
export LOG_LEVEL="debug"
```

## Deployment

### Local Testing
Run against local Elasticsearch cluster (see [../index/README.md](../index/README.md)):

```bash
# Start port-forward to local Elasticsearch
kubectl port-forward service/greenearth-es-local-es-http 9200 -n greenearth-local

# Set environment and run
export ELASTICSEARCH_URL="http://localhost:9200"
go run main.go
```

### Production Deployment
- **Target Platform**: (TODO) Azure Kubernetes Service (AKS)
- **Container Runtime**: (TODO) Docker with multi-stage builds
- **Deployment Method**: (TODO)) Kubernetes manifests via Terraform
- **Monitoring**: (TODO) Add health checks and metrics endpoints