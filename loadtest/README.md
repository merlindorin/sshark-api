# Load Testing

Load tests for the sshark API using [k6](https://k6.io/).

## Install k6

```bash
# macOS
brew install k6

# Or download from https://k6.io/docs/get-started/installation/
```

## Run Load Test

```bash
# Default run (gradual ramp-up, ~6 minutes)
k6 run loadtest/k6-validate.js

# Quick smoke test (10 iterations)
k6 run --iterations 10 loadtest/k6-validate.js

# Higher load (more VUs)
k6 run --vus 50 --duration 5m loadtest/k6-validate.js

# Output to JSON for analysis
k6 run --out json=results.json loadtest/k6-validate.js
```

## Test Parameters

Edit `k6-validate.js` to adjust:

- `USERS_PER_REQUEST` - Number of usernames per API call (default: 5)
- `options.scenarios.load_test.stages` - Ramp-up/down pattern
- `sleep()` duration - Delay between requests

## Metrics

The test tracks:

- `http_req_duration` - Response time
- `errors` - Error rate
- `validate_duration` - Custom metric for validate endpoint

## Safety

The default configuration:

- Starts with 1 VU, gradually ramps to 20
- Includes 100-300ms sleep between requests
- Has a 6-minute total duration
- Ramps down gracefully

Adjust for your production capacity.