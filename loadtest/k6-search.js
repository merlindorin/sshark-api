import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const searchDuration = new Trend('search_duration');

// Configuration
const BASE_URL = 'https://sshark.app';
const USERS_PER_REQUEST = 5;

// Load test options - adjust these for your needs
export const options = {
  scenarios: {
    // Gradual ramp-up to avoid overwhelming production
    load_test: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '30s', target: 5 },   // Ramp up to 5 VUs
        { duration: '1m', target: 10 },   // Ramp up to 10 VUs
        { duration: '2m', target: 20 },   // Ramp up to 20 VUs
        { duration: '2m', target: 20 },   // Stay at 20 VUs
        { duration: '30s', target: 0 },   // Ramp down
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    errors: ['rate<0.1'],              // Error rate under 10%
  },
};

// Load GitHub usernames from file (5,800 real usernames)
const ALL_USERNAMES = open('./users.txt').trim().split('\n');

// Build query string for N usernames
function buildQuery(usernames) {
  // Format: @username:"user1" | @username:"user2" | ...
  return usernames.map(u => `@username:{"${u}"}`).join(' | ');
}

// Get a batch of usernames for this iteration
function getUsernameBatch(iteration, batchSize) {
  const startIdx = (iteration * batchSize) % ALL_USERNAMES.length;
  const batch = [];
  for (let i = 0; i < batchSize; i++) {
    batch.push(ALL_USERNAMES[(startIdx + i) % ALL_USERNAMES.length]);
  }
  return batch;
}

export default function () {
  // Get unique batch based on VU ID and iteration
  const batchId = (__VU * 10000) + __ITER;
  const usernames = getUsernameBatch(batchId, USERS_PER_REQUEST);
  const query = buildQuery(usernames);
  const encodedQuery = encodeURIComponent(query);

  const url = `${BASE_URL}/api/v1/search/${encodedQuery}?limit=10&offset=0`;

  const params = {
    headers: {
      'User-Agent': 'k6-loadtest/1.0',
      'Accept': '*/*',
      'Accept-Language': 'en-US,en;q=0.5',
      'Accept-Encoding': 'gzip, deflate',
    },
    tags: { name: 'search' },
  };

  const start = Date.now();
  const res = http.get(url, params);
  const duration = Date.now() - start;

  searchDuration.add(duration);

  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'response has results': (r) => {
      try {
        const body = JSON.parse(r.body);
        return Array.isArray(body.results);
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!success);

  // Small sleep between requests to be nice to production
  sleep(0.1 + Math.random() * 0.2); // 100-300ms between requests
}

// Summary at the end
export function handleSummary(data) {
  const totalRequests = data.metrics.http_reqs?.values?.count || 0;
  const usersSearched = totalRequests * USERS_PER_REQUEST;

  console.log(`\n=== Load Test Summary ===`);
  console.log(`Total requests: ${totalRequests}`);
  console.log(`Users searched: ${usersSearched}`);
  console.log(`Avg response time: ${data.metrics.http_req_duration?.values?.avg?.toFixed(2)}ms`);
  console.log(`P95 response time: ${data.metrics.http_req_duration?.values['p(95)']?.toFixed(2)}ms`);
  console.log(`Error rate: ${(data.metrics.errors?.values?.rate * 100)?.toFixed(2)}%`);

  return {
    'stdout': JSON.stringify(data, null, 2),
  };
}