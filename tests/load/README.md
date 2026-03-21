# Load Testing — SaaS FinOps Analytics Platform

Load tests are written with [Locust](https://locust.io/) and target the API Gateway
(`http://localhost:8080` by default).

---

## Prerequisites

- Python 3.10+
- All platform services running (see `docker-compose.yml` or `k8s/`)
- A test database seeded with at least one subscription plan record

Install Locust:

```bash
pip install locust
```

---

## Running the Tests

### Interactive web UI (recommended for exploration)

```bash
locust -f tests/load/locustfile.py --host http://localhost:8080
```

Open `http://localhost:8089` in your browser, set the number of users and spawn
rate, then click **Start swarming**.

### Headless / CI mode

```bash
locust -f tests/load/locustfile.py \
  --host http://localhost:8080 \
  --headless \
  --users 50 \
  --spawn-rate 5 \
  --run-time 2m \
  --csv results/load_test
```

Results are written to `results/load_test_stats.csv`,
`results/load_test_failures.csv`, and `results/load_test_stats_history.csv`.

---

## User Classes

### `FinOpsUser`

Simulates a typical analyst session. Task weights reflect realistic usage patterns:

| Task | Weight | Endpoint | SLA |
|---|---|---|---|
| Get cost summary | 5 | `GET /api/finops/costs/summary` | < 500 ms |
| Run AI query | 3 | `POST /api/query/execute` | < 3 s (cache hit < 100 ms) |
| Get anomalies | 2 | `GET /api/finops/anomalies` | < 500 ms |
| Get recommendations | 2 | `GET /api/finops/recommendations` | < 500 ms |
| Refresh token | 1 | `POST /api/auth/refresh` | < 200 ms |
| Burst (rate limit test) | 1 | `GET /api/finops/costs/summary` ×10 | Expects HTTP 429 |

Wait time between tasks: **1–5 seconds**.

### `BillingUser`

Simulates an Account_Owner managing their subscription. Lower frequency than
`FinOpsUser` — billing pages are visited less often.

| Task | Weight | Endpoint |
|---|---|---|
| Get subscription | 4 | `GET /api/billing/subscription` |
| List invoices | 3 | `GET /api/billing/invoices` |
| Get plan limits | 2 | `GET /api/billing/plans` |
| Attempt upgrade | 1 | `PUT /api/billing/subscription` |

Wait time between tasks: **5–15 seconds**.

---

## Recommended Test Scenarios

### Scenario 1 — Baseline throughput

Verify the system handles normal load without errors.

```bash
locust -f tests/load/locustfile.py \
  --host http://localhost:8080 \
  --headless \
  --users 20 \
  --spawn-rate 2 \
  --run-time 5m
```

Pass criteria: error rate < 1%, p95 < 500 ms for dashboard endpoints.

---

### Scenario 2 — Rate limiting under concurrent requests

Verify HTTP 429 is returned correctly when Free plan users exceed 100 req/min.

```bash
locust -f tests/load/locustfile.py \
  --host http://localhost:8080 \
  --headless \
  --users 5 \
  --spawn-rate 5 \
  --run-time 2m \
  --class-picker FinOpsUser
```

Observe the `[burst]` task entries in the output — expect HTTP 429 responses
with a `Retry-After` header.

---

### Scenario 3 — Database connection pool stress

Simulate many concurrent AI queries to stress the connection pool.

```bash
locust -f tests/load/locustfile.py \
  --host http://localhost:8080 \
  --headless \
  --users 100 \
  --spawn-rate 10 \
  --run-time 3m \
  --class-picker FinOpsUser
```

Monitor MySQL `SHOW STATUS LIKE 'Threads_connected'` and the AI Query Engine
logs for connection pool exhaustion errors.

---

### Scenario 4 — Redis cache performance

Run repeated identical queries to verify cache hit rate and sub-100 ms responses.

```bash
locust -f tests/load/locustfile.py \
  --host http://localhost:8080 \
  --headless \
  --users 30 \
  --spawn-rate 5 \
  --run-time 2m \
  --class-picker FinOpsUser
```

Check Redis with `redis-cli monitor` or `INFO stats` — `keyspace_hits` should
increase significantly relative to `keyspace_misses` after the first minute.

---

## Interpreting Results

The test prints a summary table on exit:

```
=== Load Test Summary ===
/api/finops/costs/summary   | reqs=  1200 | fails=    3 | p50=   45ms | p95=  210ms | p99=  480ms
/api/query/execute          | reqs=   720 | fails=    0 | p50=   80ms | p95=  950ms | p99= 2100ms
...
```

Key metrics to watch:

- **Error rate** — should be < 1% under normal load (429s from burst tests are expected)
- **p95 response time** — dashboard endpoints < 500 ms, AI queries < 3 s
- **p99 response time** — should not exceed 2× the p95 value

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `LOCUST_HOST` | `http://localhost:8080` | API Gateway base URL |
| `LOCUST_USERS` | 10 | Number of concurrent users (headless mode) |
| `LOCUST_SPAWN_RATE` | 2 | Users spawned per second |
| `LOCUST_RUN_TIME` | `5m` | Test duration (headless mode) |
