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
- RESTful JSON API
- Health check endpoints for Kubernetes deployments

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

| Flag               | Environment          | Default     | Description               |
|--------------------|----------------------|-------------|---------------------------|
| `--host`           | `HOST`               | `0.0.0.0`   | Host to bind the server   |
| `--port`           | `PORT`               | `8080`      | Port to bind the server   |
| `--timeout`        | -                    | `5s`        | HTTP request timeout      |
| -                  | `POSTGRES_HOST`      | `localhost` | PostgreSQL host           |
| -                  | `POSTGRES_PORT`      | `5432`      | PostgreSQL port           |
| -                  | `POSTGRES_USER`      | `postgres`  | PostgreSQL user           |
| -                  | `POSTGRES_PASSWORD`  | -           | PostgreSQL password       |
| -                  | `POSTGRES_DATABASE`  | `sshark`    | PostgreSQL database       |
| -                  | `POSTGRES_SSL_MODE`  | `disable`   | PostgreSQL SSL mode       |

## API

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

## Architecture

The project follows a clean architecture pattern:

- **Domain Layer** (`internal/domain/`) - Business logic and interfaces
- **Infrastructure Layer** (`internal/infra/`) - PostgreSQL repositories, external clients
- **API Layer** (`internal/api/`) - HTTP handlers using Gin framework
