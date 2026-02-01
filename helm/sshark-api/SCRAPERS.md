# Multi-Provider Scrapers

The sshark-api Helm chart supports deploying multiple scrapers, one for each provider (GitHub, GitLab).

## Configuration

Each provider can be independently enabled and configured under the `scrapers` section in `values.yaml`:

```yaml
scrapers:
  github:
    enabled: true
    rateLimit: 2.0
    batchSize: 100
    resources:
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 50m
        memory: 64Mi

  gitlab:
    enabled: false
    rateLimit: 5.0
    batchSize: 100
    token: ""
    existingSecret: ""
    existingSecretKey: "gitlab-token"
    resources:
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 50m
        memory: 64Mi
```

## Deployment Strategy

Each enabled scraper will deploy as a separate Deployment:
- `<release>-scraper-github` for GitHub scraping
- `<release>-scraper-gitlab` for GitLab scraping

This allows:
- Independent scaling per provider
- Different resource limits per provider
- Different rate limits per provider
- Independent node placement via nodeSelector/affinity

## GitLab Token Configuration

GitLab scraping requires an API token with `read_api` scope.

### Option 1: Direct Token (Development)

Set the token directly in values.yaml:

```yaml
scrapers:
  gitlab:
    enabled: true
    token: "glpat-xxxxxxxxxxxxxxxxxxxx"
```

The chart will automatically create a secret named `<release>-gitlab-token`.

**⚠️ Warning:** Do not commit tokens to version control!

### Option 2: Existing Secret (Production)

Create a secret manually:

```bash
kubectl create secret generic gitlab-token \
  --from-literal=gitlab-token=glpat-xxxxxxxxxxxxxxxxxxxx
```

Then reference it in values.yaml:

```yaml
scrapers:
  gitlab:
    enabled: true
    existingSecret: "gitlab-token"
    existingSecretKey: "gitlab-token"
```

### Option 3: External Secret Operator

Use external-secrets operator to sync from a secret manager:

```yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: gitlab-token
spec:
  secretStoreRef:
    name: vault
    kind: SecretStore
  target:
    name: gitlab-token
  data:
    - secretKey: gitlab-token
      remoteRef:
        key: sshark-api/gitlab-token
```

Then reference it:

```yaml
scrapers:
  gitlab:
    enabled: true
    existingSecret: "gitlab-token"
```

## Examples

### Enable Both Scrapers

```yaml
scrapers:
  github:
    enabled: true
    rateLimit: 2.0
    batchSize: 100

  gitlab:
    enabled: true
    rateLimit: 5.0
    batchSize: 100
    existingSecret: "gitlab-token"
```

### GitLab Only

```yaml
scrapers:
  github:
    enabled: false

  gitlab:
    enabled: true
    rateLimit: 10.0
    batchSize: 200
    existingSecret: "gitlab-token"
    resources:
      limits:
        cpu: 200m
        memory: 256Mi
```

### Different Node Placement

```yaml
scrapers:
  github:
    enabled: true
    nodeSelector:
      scraper: github
    tolerations:
      - key: "scraper"
        operator: "Equal"
        value: "github"
        effect: "NoSchedule"

  gitlab:
    enabled: true
    existingSecret: "gitlab-token"
    nodeSelector:
      scraper: gitlab
    tolerations:
      - key: "scraper"
        operator: "Equal"
        value: "gitlab"
        effect: "NoSchedule"
```

## Monitoring

Each scraper deployment has labels for identification:
- `app.kubernetes.io/component: scraper`
- `app.kubernetes.io/provider: <provider>`

Use these labels for monitoring:

```bash
# View GitHub scraper logs
kubectl logs -l app.kubernetes.io/provider=github -f

# View GitLab scraper logs
kubectl logs -l app.kubernetes.io/provider=gitlab -f

# List all scrapers
kubectl get deployments -l app.kubernetes.io/component=scraper
```

## Rate Limits

Default rate limits are conservative to avoid hitting API limits:

| Provider | Default Rate | API Limit | Recommended |
|----------|-------------|-----------|-------------|
| GitHub   | 2.0 req/s   | 60 req/h (unauth) | 2.0 req/s |
| GitLab   | 5.0 req/s   | 600 req/min (auth) | 5.0-10.0 req/s |

Adjust based on your quota and needs:

```yaml
scrapers:
  github:
    rateLimit: 1.0  # Slower, more conservative

  gitlab:
    rateLimit: 10.0  # Faster, uses more quota
```

## Troubleshooting

### GitLab Scraper Not Starting

Check if the secret exists and contains the token:

```bash
kubectl get secret <release>-gitlab-token
kubectl get secret <release>-gitlab-token -o jsonpath='{.data.gitlab-token}' | base64 -d
```

### Token Validation Error

Verify the token has correct permissions:

```bash
curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" https://gitlab.com/api/v4/user
```

Should return user information. If 401, the token is invalid.

### Rate Limit Errors

If seeing "429 Too Many Requests" errors, reduce the rate limit:

```yaml
scrapers:
  gitlab:
    rateLimit: 2.0  # Reduce from 5.0
```

### Progress Tracking

Each provider has independent progress tracking in Redis:
- GitHub: `sshark:scraper:last_user_id`
- GitLab: `sshark:scraper:gitlab:last_page`

To reset progress:

```bash
# Reset GitHub progress
kubectl exec -it <redis-pod> -- redis-cli DEL sshark:scraper:last_user_id

# Reset GitLab progress
kubectl exec -it <redis-pod> -- redis-cli DEL sshark:scraper:gitlab:last_page
```
