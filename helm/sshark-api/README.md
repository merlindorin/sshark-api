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

For a complete list of values, see [values.yaml](values.yaml).