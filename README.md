# SSHark API

[![Build Status](https://github.com/merlindorin/sshark-api/actions/workflows/golangci.yml/badge.svg)](https://github.com/merlindorin/sshark-api/actions/workflows/golangci.yml)
[![Test Status](https://github.com/merlindorin/sshark-api/actions/workflows/goreleaser.yml/badge.svg)](https://github.com/merlindorin/sshark-api/actions/workflows/goreleaser.yml)


> SSH public key lookup API service. Fetches and indexes
> SSH public keys from GitHub users, providing a searchable
> database via Redis.

## Table of Contents

* [Features](#features)
* [Requirements](#requirements)
* [Usage](#usage)
  * [Running the Server](#running-the-server)
  * [Configuration](#configuration)
* [Development](#development)
  * [Running Tests](#running-tests)
  * [Building](#building)
  * [Docker](#docker)
* [Architecture](#architecture)

## Features

- Fetch SSH public keys from GitHub users on-demand
- Full-text search across SSH keys using Redis Search
- RESTful JSON API
- Health check endpoints for Kubernetes deployments

## Requirements

- Go 1.24+
- Redis 7+ (with RediSearch module)

## Usage

### Running the Server

```bash
# Start with defaults (localhost:8080, Redis on localhost:6379)
go run ./cmd serve

# Custom host and port
go run ./cmd serve --host 127.0.0.1 --port 3000

# Custom Redis connection
go run ./cmd serve --redis-host redis.example.com --redis-port 6379 --redis-password secret
```

### Configuration

Configuration can be set via CLI flags, environment variables, or config files.

| Flag               | Environment      | Default     | Description             |
|--------------------|------------------|-------------|-------------------------|
| `--host`           | `HOST`           | `0.0.0.0`   | Host to bind the server |
| `--port`           | `PORT`           | `8080`      | Port to bind the server |
| `--timeout`        | -                | `5s`        | HTTP request timeout    |
| `--redis-host`     | `REDIS_HOST`     | `localhost` | Redis host              |
| `--redis-port`     | `REDIS_PORT`     | `6379`      | Redis port              |
| `--redis-password` | `REDIS_PASSWORD` | -           | Redis password          |
| `--redis-db`       | `REDIS_DB`       | `0`         | Redis database number   |

Config files are loaded from:

- `/etc/sshark/config.yaml`
- `~/.config/sshark/config.yaml`

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
docker run -p 8080:8080 -e REDIS_HOST=host.docker.internal sshark-api
```

## Architecture

The project follows a clean architecture pattern:

- **Domain Layer** (`internal/domain/`) - Business logic and interfaces
- **Infrastructure Layer** (`internal/infra/`) - Redis repositories, external clients
- **API Layer** (`internal/api/`) - HTTP handlers using Gin framework