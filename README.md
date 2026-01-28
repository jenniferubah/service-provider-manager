# Service Provider Manager

DCM Service Provider Manager - Registration and management of Service Providers.

## Quick Start

### Prerequisites

- Go 1.25+
- Podman or Docker with Compose

### Run Locally

```bash
# Start the service with PostgreSQL
podman-compose up -d

# Check health
curl http://localhost:8080/api/v1alpha1/health
```

### Build

```bash
make build        # Build binary
make run          # Run locally (requires PostgreSQL)
```

### Test

```bash
make test         # Unit tests
make test-e2e     # E2E tests (requires running services)
```

### API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1alpha1/health` | Health check |
| POST | `/api/v1alpha1/providers` | Register provider (idempotent) |
| GET | `/api/v1alpha1/providers` | List providers |
| GET | `/api/v1alpha1/providers/{id}` | Get provider |
| PUT | `/api/v1alpha1/providers/{id}` | Update provider |
| DELETE | `/api/v1alpha1/providers/{id}` | Delete provider |

### Client Library

A Go client library is available for Service Providers to integrate with DCM:

```bash
go get github.com/dcm-project/service-provider-manager/pkg/client
```

See [pkg/client/README.md](pkg/client/README.md) for usage examples.

### Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `SVC_ADDRESS` | `:8080` | Service listen address |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `service-provider` | Database name |
| `DB_USER` | *(none)* | Database user (required for pgsql) |
| `DB_PASS` | *(none)* | Database password (required for pgsql) |

## License

Apache 2.0 - see [LICENSE](LICENSE)
