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

# Run server locally (requires PostgreSQL)
go run ./cmd serve
```

## Architecture

Clean architecture with three layers:

- **Domain** (`internal/domain/`) - Business entities, interfaces (ports), and errors
  - `github/` - GitHub user entity, username validation
  - `publickeys/` - Public key entity and repository interface
  - `stats/` - Statistics entity and repository interface
  - `query/` - Query parser for search syntax

- **Infrastructure** (`internal/infra/`) - Repository implementations and external clients
  - `publickeys/postgres/` - PostgreSQL repository for public keys
  - `query/postgres/` - PostgreSQL query builder for search
  - `fetchers/github/` - HTTP fetcher for GitHub's public keys endpoint
  - `fetchers/gitlab/` - HTTP fetcher for GitLab's public keys endpoint

- **API** (`api/`) - OpenAPI-generated HTTP handlers
  - `public/` - Public API endpoints (search, stats)
  - `common/` - Shared schemas

## Key Patterns

- PostgreSQL for persistent storage with migrations (`db/migrations/`)
- Search uses custom query parser (`internal/domain/query/`) that generates SQL WHERE clauses
- Repository pattern with context-aware methods
- OpenAPI code generation with oapi-codegen

## Commit Convention

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification.

Format: `<type>: <description>`

Common types:
- `feat` - New feature
- `fix` - Bug fix
- `docs` - Documentation changes
- `chore` - Maintenance tasks (deps, CI, etc.)
- `refactor` - Code refactoring without behavior change
- `test` - Adding or updating tests

Examples:
```bash
git commit -m "feat: add user authentication endpoint"
git commit -m "fix: handle empty search query"
git commit -m "docs: update API documentation"
git commit -m "chore: bump chart version to 0.1.7"
```

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
