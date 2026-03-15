# SSHark API Helm Chart

SSH and GPG public key lookup service that fetches and caches keys from GitHub and GitLab.

## Installation

```bash
helm install sshark-api ./helm/sshark-api
```

## Configuration

### PostgreSQL Backend

The chart requires a PostgreSQL database for storage.

#### External PostgreSQL (Recommended)

```yaml
postgres:
  host: "postgres.example.com"
  port: 5432
  user: "sshark"
  database: "sshark"
  sslMode: "require"
  existingSecret: "postgres-credentials"
  existingSecretPasswordKey: "password"
```

#### Direct Configuration (Development)

```yaml
postgres:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "mysecret"
  database: "sshark"
  sslMode: "disable"
```

### Scraper

The scraper fetches public keys from GitHub/GitLab users with rate limiting and progress tracking.

#### Enable the Scraper

```yaml
scraper:
  enabled: true
  provider: "github"    # or "gitlab"
  batchSize: 100
  delay: "1s"
```

#### Scraper Resources

```yaml
scraper:
  resources:
    limits:
      cpu: 100m
      memory: 128Mi
    requests:
      cpu: 50m
      memory: 64Mi
```

#### API Tokens

For GitHub (optional but recommended for higher rate limits):

```yaml
scraper:
  githubToken: "ghp_xxxx"
  # Or use existing secret (recommended):
  existingSecret: "scraper-tokens"
  existingSecretGithubKey: "github-token"
```

For GitLab (required):

```yaml
scraper:
  gitlabToken: "glpat-xxxx"
  # Or use existing secret (recommended):
  existingSecret: "scraper-tokens"
  existingSecretGitlabKey: "gitlab-token"
```

### Grafana Dashboard

The chart includes an optional Grafana dashboard that works with [Grafana Operator](https://github.com/grafana-operator/grafana-operator).

```yaml
grafana:
  dashboard:
    enabled: true
    datasourceUID: "prometheus-uid"
    instanceSelector:
      matchLabels:
        dashboards: "grafana"
    folderName: "SSHark"
```

## Common Configurations

### Basic Installation

```bash
helm install sshark-api ./helm/sshark-api \
  --set postgres.host=postgres.default.svc \
  --set postgres.password=secret
```

### With Scraper Enabled

```bash
helm install sshark-api ./helm/sshark-api \
  --set postgres.host=postgres.default.svc \
  --set postgres.password=secret \
  --set scraper.enabled=true \
  --set scraper.provider=github
```

### With Ingress

```yaml
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: sshark.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: sshark-tls
      hosts:
        - sshark.example.com
```

### With Autoscaling (API Only)

```yaml
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 80
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `replicaCount` | int | `1` | Number of API replicas |
| `image.repository` | string | `"ghcr.io/merlindorin/sshark-api"` | Image repository |
| `image.tag` | string | `""` | Image tag (defaults to chart appVersion) |
| `postgres.host` | string | `"localhost"` | PostgreSQL host |
| `postgres.port` | int | `5432` | PostgreSQL port |
| `postgres.user` | string | `"postgres"` | PostgreSQL user |
| `postgres.password` | string | `""` | PostgreSQL password |
| `postgres.database` | string | `"sshark"` | PostgreSQL database |
| `postgres.sslMode` | string | `"disable"` | PostgreSQL SSL mode |
| `postgres.existingSecret` | string | `""` | Existing secret for PostgreSQL password |
| `scraper.enabled` | bool | `false` | Enable scraper deployment |
| `scraper.provider` | string | `"github"` | Provider to scrape (github/gitlab) |
| `scraper.batchSize` | int | `100` | Users to fetch per API call |
| `scraper.delay` | string | `"1s"` | Delay between batches |
| `grafana.dashboard.enabled` | bool | `false` | Enable Grafana dashboard |
| `grafana.dashboard.datasourceUID` | string | `""` | Prometheus datasource UID |
| `metrics.enabled` | bool | `true` | Enable Prometheus metrics endpoint |
| `migration.enabled` | bool | `true` | Enable database migration hook |

For a complete list of values, see [values.yaml](values.yaml).
