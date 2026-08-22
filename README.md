# SSHark API

[![Build Status](https://github.com/merlindorin/sshark-api/actions/workflows/golangci.yml/badge.svg)](https://github.com/merlindorin/sshark-api/actions/workflows/golangci.yml)
[![Test Status](https://github.com/merlindorin/sshark-api/actions/workflows/goreleaser.yml/badge.svg)](https://github.com/merlindorin/sshark-api/actions/workflows/goreleaser.yml)


> SSH and GPG public key lookup API service. Fetches and indexes
> public keys from GitHub and GitLab users, providing a searchable
> database via PostgreSQL.

## Table of Contents

* [Features](#features)
* [Requirements](#requirements)
* [Usage](#usage)
  * [Running the Server](#running-the-server)
  * [Configuration](#configuration)
* [API](#api)
  * [Search Query Syntax](#search-query-syntax)
* [Development](#development)
  * [Running Tests](#running-tests)
  * [Building](#building)
  * [Docker](#docker)
* [Architecture](#architecture)

## Features

- Fetch SSH and GPG public keys from GitHub and GitLab users
- Full-text search across keys using PostgreSQL
- User profiles with verified key ownership
- Background task queue for key refresh and revocation
- RESTful JSON API with authentication
- Health check endpoints for Kubernetes deployments
- OpenTelemetry metrics and Grafana dashboards

## Requirements

- Go 1.24+
- PostgreSQL 15+

## Usage

### Running the Server

```bash
# Start with defaults (localhost:8080, PostgreSQL on localhost:5432)
go run ./cmd serve

# Custom host and port
go run ./cmd serve --host 127.0.0.1 --port 3000

# Custom PostgreSQL connection
POSTGRES_HOST=db.example.com POSTGRES_USER=myuser POSTGRES_PASSWORD=secret go run ./cmd serve
```

### Configuration

Configuration can be set via CLI flags or environment variables.

| Flag               | Environment                  | Default     | Description                        |
|--------------------|------------------------------|-------------|------------------------------------|
| `--host`           | `HTTP_HOST`                  | `0.0.0.0`   | Host to bind the server            |
| `--port`           | `HTTP_PORT`                  | `8080`      | Port to bind the server            |
| -                  | `HTTP_READ_TIMEOUT`          | `30s`       | HTTP read timeout                  |
| -                  | `HTTP_READHEADER_TIMEOUT`    | `5s`        | HTTP read header timeout           |
| -                  | `HTTP_WRITE_TIMEOUT`         | `30s`       | HTTP write timeout                 |
| -                  | `HTTP_IDLE_TIMEOUT`          | `120s`      | HTTP idle timeout                  |
| -                  | `HTTP_MAX_HEADER_BYTES`      | `1048576`   | Maximum header size in bytes       |
| -                  | `HTTP_GRACEFUL_PERIOD`       | `5s`        | Graceful shutdown period           |
| -                  | `POSTGRES_HOST`              | `localhost` | PostgreSQL host                    |
| -                  | `POSTGRES_PORT`              | `5432`      | PostgreSQL port                    |
| -                  | `POSTGRES_USER`              | `postgres`  | PostgreSQL user                    |
| -                  | `POSTGRES_PASSWORD`          | -           | PostgreSQL password                |
| -                  | `POSTGRES_DATABASE`          | `sshark`    | PostgreSQL database                |
| -                  | `POSTGRES_SSL_MODE`          | `disable`   | PostgreSQL SSL mode                |
| -                  | `POSTGRES_MAX_CONNS`         | `10`        | Maximum PostgreSQL connections     |
| -                  | `POSTGRES_MIN_CONNS`         | `2`         | Minimum PostgreSQL connections     |
| -                  | `POSTGRES_MAX_CONN_LIFETIME` | `1h`        | Maximum connection lifetime        |
| -                  | `POSTGRES_MAX_CONN_IDLE_TIME`| `30m`       | Maximum connection idle time       |
| -                  | `CLERK_TOKEN`                | -           | Clerk authentication token         |
| -                  | `GITHUB_TOKEN`               | -           | GitHub API token (for key refresh) |
| -                  | `GITLAB_TOKEN`               | -           | GitLab API token (for key refresh) |
| -                  | `METRIC_PATH`                | `/metrics`  | Prometheus metrics endpoint path   |
| -                  | `OTEL_EXPORTER_OTLP_ENDPOINT`| -           | OpenTelemetry collector endpoint   |

## API

### Authentication

Protected endpoints require a bearer token from Clerk or an API key:

```bash
# Using Clerk session token
curl -H "Authorization: Bearer $CLERK_TOKEN" \
  http://localhost:8080/api/v1/me/keys

# Using API key
curl -H "Authorization: Bearer ak_your_key_here" \
  http://localhost:8080/api/v1/me/keys
```

### Public Endpoints

#### GET /users/{username}

Retrieve a public user profile:

```bash
curl http://localhost:8080/users/merlindorin
```

Returns connected accounts and published keys.

#### GET /users/{username}.keys

Download SSH public keys in `authorized_keys` format:

```bash
curl http://localhost:8080/users/merlindorin.keys >> ~/.ssh/authorized_keys
```

#### GET /users/{username}.gpg

Download GPG public keys in ASCII armor format:

```bash
curl http://localhost:8080/users/merlindorin.gpg | gpg --import
```

### Authenticated Endpoints

#### GET /api/v1/me/keys

List your published keys across all connected providers.

#### POST /api/v1/me/keys/refresh

Trigger an on-demand refresh of your keys from connected providers. Returns a task ID for tracking.

#### DELETE /api/v1/me/keys/{id}

Revoke a key by deleting it at the provider, then removing it from SShark.

#### GET /api/v1/me/username

Retrieve your claimed username.

#### PUT /api/v1/me/username

Claim or change your username (must be unique and not reserved).

#### GET /api/v1/me

Retrieve authenticated user information.

#### GET /api/v1/me/username/available

Check if a username is available for claiming.

#### DELETE /api/v1/me/profile

Delete your SShark profile and all associated data.

#### GET /api/v1/me/tasks

List background tasks (key refresh, revocation) for your account.

#### GET /api/v1/me/tasks/{id}

Get the status of a specific background task.

#### GET /api/v1/me/apikeys

List your API keys for programmatic access.

#### POST /api/v1/me/apikeys

Create a new API key. Returns the key secret (shown only once).

```bash
curl -X POST -H "Authorization: Bearer $CLERK_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"CI/CD Key","description":"Key for GitHub Actions","expiration":1735689600}' \
  http://localhost:8080/api/v1/me/apikeys
```

#### DELETE /api/v1/me/apikeys/{id}

Revoke an API key immediately.

### Public Search Endpoints

#### GET /api/v1/ssh/search

Search SSH public keys using basic or advanced query syntax.

#### GET /api/v1/gpg/search

Search GPG public keys using basic or advanced query syntax.

#### GET /api/v1/publickeys/{id}

Retrieve a specific public key by UUID.

#### GET /api/v1/sources

List recently indexed sources with pagination.

#### GET /api/v1/sources/{provider}/{username}

Get a source with all associated SSH and GPG keys.

#### GET /api/v1/stats

Get search statistics and facets for analytics.

### Health Endpoints

#### GET /liveness

Kubernetes liveness probe endpoint.

#### GET /readiness

Kubernetes readiness probe endpoint.

### Search Query Syntax

The API supports two search modes: **basic** and **advanced**.

#### Basic Mode

Use `query` and `fields` parameters for simple OR searches:

```bash
# Search across default fields (source.username, source.provider)
curl "http://localhost:8080/api/v1/ssh/search?query=torvalds"

# Search specific fields
curl "http://localhost:8080/api/v1/ssh/search?query=torvalds&fields=source.username,algorithm"
```

#### Advanced Mode

Use the `q` parameter with structured query syntax:

```
@field:{value}              Exact match
@field:{prefix*}            Prefix wildcard
@field:{*contains*}         Contains wildcard
@field:{a} | @field:{b}     OR operator
@field:{a} & @field:{b}     AND operator
(@field:{a} | @field:{b})   Grouping
```

**Examples:**

```bash
# Exact username match
curl "http://localhost:8080/api/v1/ssh/search?q=@source.username:{torvalds}"

# Prefix wildcard
curl "http://localhost:8080/api/v1/ssh/search?q=@source.username:{linus*}"

# Contains wildcard
curl "http://localhost:8080/api/v1/ssh/search?q=@source.username:{*torv*}"

# OR query
curl "http://localhost:8080/api/v1/ssh/search?q=@source.provider:{github}|@source.provider:{gitlab}"

# AND query
curl "http://localhost:8080/api/v1/ssh/search?q=@source.username:{torvalds}%26@algorithm:{ed25519}"

# Complex grouped query
curl "http://localhost:8080/api/v1/ssh/search?q=@source.username:{linus*}%26(@source.provider:{github}|@source.provider:{gitlab})"
```

#### SSH Search Fields

| Field              | Description                    |
|--------------------|--------------------------------|
| `id`               | Public key UUID                |
| `fingerprint`      | Key fingerprint                |
| `algorithm`        | Key algorithm (RSA, Ed25519)   |
| `comment`          | Key comment                    |
| `key_bits`         | Key size in bits               |
| `source.username`  | Username from provider         |
| `source.provider`  | Provider (github, gitlab)      |
| `source.user_id`   | User ID from provider          |
| `source.uri`       | Source URI                     |

#### GPG Search Fields

| Field              | Description                    |
|--------------------|--------------------------------|
| `id`               | Public key UUID                |
| `fingerprint`      | Key fingerprint                |
| `algorithm`        | Key algorithm (RSA, DSA, etc.) |
| `key_bits`         | Key size in bits               |
| `user_ids`         | Array of user IDs (email/name) |
| `capabilities`     | Key capabilities (sign, cert)  |
| `source.username`  | Username from provider         |
| `source.provider`  | Provider (github, gitlab)      |
| `source.user_id`   | User ID from provider          |
| `source.uri`       | Source URI                     |

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
# Build binary
go build -o sshark ./cmd

# Build with version info
go build -ldflags="-X main.version=1.0.0 -X main.commit=$(git rev-parse HEAD)" -o sshark ./cmd
```

### Docker

```bash
# Build image
docker build -t sshark-api .

# Run container
docker run -p 8080:8080 \
  -e POSTGRES_HOST=host.docker.internal \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=secret \
  sshark-api
```

## Background Tasks

The API uses a persistent queue backed by PostgreSQL to handle long-running operations:

- **Profile refresh** — Scheduled daily re-fetch of keys for active profiles
- **On-demand refresh** — User-triggered key updates via `/api/v1/me/keys/refresh`
- **Key revocation** — Delete keys at the provider before removing locally

Task status can be tracked through the Activity table in user profiles on the web interface.

## Observability

### Metrics

The service exports OpenTelemetry metrics for:

- HTTP request rate, latency, and errors (golden signals)
- Database connection pool usage
- Background task queue depth and processing time
- Authentication success/failure rates

### Grafana Dashboard

A pre-built dashboard is included at `grafana/dashboards/sshark-overview.json` with panels for:

- Request throughput and P95 latency
- Error rates by endpoint
- Database pool health
- Task queue backlog

Import the dashboard into your Grafana instance and configure it to read from your OTLP collector.

## Architecture

The project follows a clean architecture pattern:

- **Domain Layer** (`internal/domain/`) - Business logic and interfaces
- **Infrastructure Layer** (`internal/infra/`) - PostgreSQL repositories, external clients, task queue
- **API Layer** (`internal/api/`) - HTTP handlers using Gin framework, authentication middleware
