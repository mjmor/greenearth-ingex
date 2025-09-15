# Ingest Service

Go service that connects to BlueSky's TurboStream websocket to capture real-time events and index them in Elasticsearch.

## Features

- Real-time BlueSky event streaming via TurboStream websocket
- Elasticsearch indexing with bulk operations
- Graceful shutdown and error handling
- Google Cloud Run deployment ready

## Architecture

```
BlueSky TurboStream → Go Service → Elasticsearch
```

## Development

```bash
# Run locally
go run main.go

# Run tests
go test ./...

# Build
go build -o ingest
```

## Environment Variables

- `ELASTICSEARCH_URL` - Elasticsearch cluster endpoint
- `TURBOSTREAM_URL` - BlueSky TurboStream websocket URL
- `PORT` - Service port (default: 8080)