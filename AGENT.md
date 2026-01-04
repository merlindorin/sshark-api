# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Run all checks (lint, build) - also runs via pre-commit hook
task

# Run linter
task golangci:run

# Fix lint issues
task golangci:fix

# Build
go build ./...

# Run tests
go test ./...

# Run single test
go test -run TestName ./path/to/package

# Run server locally (requires Redis with RediSearch)
go run ./cmd serve
```

## Architecture

Clean architecture with three layers:

- **Domain** (`internal/domain/`) - Business entities, interfaces (ports), and errors
  - `github/` - GitHub user entity, username validation
  - `sshkeys/` - SSH key entity and repository interface
  - `ingester/` - Service orchestrating GitHub fetch → SSH key storage
  - `stats/` - Statistics entity and repository interface
  - `query/` - Query explainer interface for RediSearch validation

- **Infrastructure** (`internal/infra/`) - Repository implementations and external clients
  - `sshkeys/redis/` - Redis repository for SSH keys with RediSearch indexing
  - `github/redis/` - Redis repository for GitHub user cache
  - `github/` - HTTP fetcher for GitHub's public keys endpoint

- **API** (`internal/api/`) - Gin HTTP handlers
  - `search/` - Full-text search endpoint
  - `sshkeys/` - CRUD operations for SSH keys
  - `validate/` - Query validation via FT.EXPLAIN
  - `stats/` - Aggregated statistics endpoint
  - `probe/` - Health check endpoints (liveness/readiness)

## Key Patterns

- Repositories use RESP3 protocol with Redis Stack (RediSearch + RedisJSON)
- Search uses `FT.SEARCH` with custom RESP3 result parsing (`internal/infra/utils.go`)
- Statistics use `FT.AGGREGATE` with GROUPBY for counts
- Repository methods hydrate result objects passed by reference (e.g., `GetStats(ctx, *Stats) error`)

## Release Workflow

1. **Commit changes** (pre-commit hook runs `task` automatically)
   ```bash
   git add <files> && git commit -m "feat/fix: message"
   ```

2. **Update Helm chart** (`helm/sshark-api/Chart.yaml`)
   - Bump `version` and `appVersion` to match the new version

3. **Commit chart update**
   ```bash
   git add helm/sshark-api/Chart.yaml && git commit -m "chore: bump chart version to 0.x.x"
   ```

4. **Tag and push**
   ```bash
   git tag v0.x.x && git push && git push --tags
   ```

5. **Goreleaser** handles the release from CI
