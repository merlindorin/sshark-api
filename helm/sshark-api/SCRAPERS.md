# Multi-Provider Scrapers

The sshark-api Helm chart supports deploying scrapers for multiple providers (GitHub, GitLab).

## Configuration

Configure the scraper in `values.yaml`:

```yaml
scraper:
  enabled: true
  provider: "github"    # or "gitlab"
  batchSize: 100
  delay: "1s"
  githubToken: ""
  gitlabToken: ""
  existingSecret: ""
  existingSecretGithubKey: "github-token"
  existingSecretGitlabKey: "gitlab-token"
  resources:
    limits:
      cpu: 100m
      memory: 128Mi
    requests:
      cpu: 50m
      memory: 64Mi
```

## Token Configuration

### GitHub Token (Optional)

GitHub scraping works without authentication but with lower rate limits. For production use:

```yaml
scraper:
  enabled: true
  provider: "github"
  githubToken: "ghp_xxxxxxxxxxxxxxxxxxxx"
```

Or use an existing secret (recommended):

```bash
kubectl create secret generic scraper-tokens \
  --from-literal=github-token=ghp_xxxxxxxxxxxxxxxxxxxx
```

```yaml
scraper:
  enabled: true
  provider: "github"
  existingSecret: "scraper-tokens"
  existingSecretGithubKey: "github-token"
```

### GitLab Token (Required)

GitLab scraping requires an API token with `read_api` scope:

```yaml
scraper:
  enabled: true
  provider: "gitlab"
  gitlabToken: "glpat-xxxxxxxxxxxxxxxxxxxx"
```

Or use an existing secret:

```bash
kubectl create secret generic scraper-tokens \
  --from-literal=gitlab-token=glpat-xxxxxxxxxxxxxxxxxxxx
```

```yaml
scraper:
  enabled: true
  provider: "gitlab"
  existingSecret: "scraper-tokens"
  existingSecretGitlabKey: "gitlab-token"
```

## Examples

### GitHub Scraper

```yaml
scraper:
  enabled: true
  provider: "github"
  batchSize: 100
  delay: "1s"
```

### GitLab Scraper

```yaml
scraper:
  enabled: true
  provider: "gitlab"
  batchSize: 100
  delay: "2s"
  existingSecret: "gitlab-token"
```

## Monitoring

The scraper deployment has labels for identification:

```bash
# View scraper logs
kubectl logs -l app.kubernetes.io/component=scraper -f

# Get scraper deployment
kubectl get deployments -l app.kubernetes.io/component=scraper
```

## Rate Limits

Default configurations are conservative to avoid hitting API limits:

| Provider | Default Delay | API Limit | Notes |
|----------|--------------|-----------|-------|
| GitHub   | 1s           | 60 req/h (unauth), 5000/h (auth) | Token recommended |
| GitLab   | 1s           | 120 req/min (auth) | Token required |

Adjust based on your quota:

```yaml
scraper:
  delay: "500ms"  # Faster, but watch rate limits
```

## Troubleshooting

### GitLab Scraper Not Starting

Check if the secret exists and contains the token:

```bash
kubectl get secret scraper-tokens
kubectl get secret scraper-tokens -o jsonpath='{.data.gitlab-token}' | base64 -d
```

### Token Validation Error

Verify the token has correct permissions:

```bash
curl -H "PRIVATE-TOKEN: $GITLAB_TOKEN" https://gitlab.com/api/v4/user
```

Should return user information. If 401, the token is invalid.

### Rate Limit Errors

If seeing "429 Too Many Requests" errors, increase the delay:

```yaml
scraper:
  delay: "2s"
```
