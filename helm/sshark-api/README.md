# SSHark API Helm Chart

SSH public key lookup service that fetches and caches GitHub SSH keys.

## Installation

```bash
helm install sshark-api ./helm/sshark-api
```

## Configuration

### Redis Backend

The chart supports three Redis backend options:

#### 1. Redis Stack (Default)

```yaml
redis-stack:
  enabled: true
```

**Warning:** The redis-stack subchart does NOT support persistence by default. Data will be lost on pod restart.

#### 2. Dragonfly

Requires [dragonfly-operator](https://github.com/dragonflydb/dragonfly-operator) to be installed.

```yaml
redis-stack:
  enabled: false

dragonfly:
  enabled: true
  replicas: 1
  storage:
    enabled: true
    size: 1Gi
```

#### 3. External Redis

```yaml
redis-stack:
  enabled: false

dragonfly:
  enabled: false

redis:
  host: "my-redis.example.com"
  port: 6379
  password: "secret"
  db: 0
```

### GitHub Scraper

The scraper progressively fetches all GitHub users and their SSH keys with rate limiting and progress tracking.

#### Enable the Scraper

```yaml
scraper:
  enabled: true
  rateLimit: 2.0    # Requests per second to GitHub API
  batchSize: 100    # Number of users to fetch per API call
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

#### How It Works

1. **Progressive Scraping**: Fetches users from GitHub's `/users` API endpoint with pagination
2. **Rate Limiting**: Respects configurable rate limits (default: 2 req/s)
3. **Progress Tracking**: Stores last processed user ID in Redis (`sshark:scraper:last_user_id`)
4. **Resumable**: Automatically resumes from last position after restart
5. **Graceful Shutdown**: Handles SIGTERM/SIGINT and saves progress before exiting

The scraper runs as a single replica deployment with `Recreate` strategy to ensure only one instance is running at a time.

### Grafana Dashboard

The chart includes an optional Grafana dashboard that works with [Grafana Operator](https://github.com/grafana-operator/grafana-operator).

#### Prerequisites

1. Grafana Operator installed in your cluster
2. A Grafana instance (Grafana CR) running
3. Prometheus datasource configured in Grafana
4. ServiceMonitor enabled to scrape metrics

#### Enable the Dashboard

1. Find your Prometheus datasource UID in Grafana:
   - Go to: **Settings → Data Sources → Prometheus**
   - Copy the **UID** (e.g., `eed2ceb1-51e5-471e-950f-beab8421a126`)

2. Enable the dashboard:

```yaml
grafana:
  dashboard:
    enabled: true
    datasourceUID: "eed2ceb1-51e5-471e-950f-beab8421a126"  # Your Prometheus UID
    instanceSelector:
      matchLabels:
        dashboards: "grafana"  # Labels matching your Grafana CR
    folderName: "SSHark"  # Optional: organize in a folder
```

3. Install or upgrade:

```bash
helm upgrade --install sshark-api ./helm/sshark-api \
  --set grafana.dashboard.enabled=true \
  --set grafana.dashboard.datasourceUID=YOUR-DATASOURCE-UID
```

#### Dashboard Features

- **Statistics**: Total keys, providers, and usernames
- **Scraper Activity**: Users processed/ingested rates, position tracking
- **Error Tracking**: Fetch and ingest error rates
- **Performance**: Duration, rate limit wait time, batch size (p50, p95, p99)

Auto-refresh: 30 seconds | Default range: Last 6 hours

## Common Configurations

### Basic Installation

```bash
helm install sshark-api ./helm/sshark-api
```

### With Scraper Enabled

```bash
helm install sshark-api ./helm/sshark-api \
  --set scraper.enabled=true \
  --set scraper.rateLimit=1.5
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
| `scraper.enabled` | bool | `false` | Enable GitHub scraper deployment |
| `scraper.rateLimit` | float | `2.0` | GitHub API rate limit (req/s) |
| `scraper.batchSize` | int | `100` | Users to fetch per API call |
| `redis-stack.enabled` | bool | `true` | Enable Redis Stack subchart |
| `dragonfly.enabled` | bool | `false` | Enable Dragonfly |
| `redis.host` | string | `""` | External Redis host |
| `redis.port` | int | `6379` | Redis port |
| `redis.password` | string | `""` | Redis password |
| `redis.db` | int | `0` | Redis database number |
| `grafana.dashboard.enabled` | bool | `false` | Enable Grafana dashboard |
| `grafana.dashboard.datasourceUID` | string | `""` | Prometheus datasource UID in Grafana |
| `metrics.enabled` | bool | `true` | Enable Prometheus metrics endpoint |

For a complete list of values, see [values.yaml](values.yaml).