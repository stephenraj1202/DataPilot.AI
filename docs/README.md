# SaaS FinOps Analytics Platform – API Reference

## Overview

The SaaS FinOps Analytics Platform exposes a REST API across five microservices:

| Service | Port | Responsibility |
|---|---|---|
| API Gateway | 8080 | Auth middleware, rate limiting, routing |
| Auth Service | 8081 | Registration, login, JWT, API keys |
| Billing Service | 8082 | Stripe subscriptions, invoices, webhooks |
| FinOps Service | 8083 | Cloud cost aggregation, anomaly detection |
| AI Query Engine | 8084 | Natural language → SQL, schema extraction |

In production all traffic flows through the **API Gateway** on port 8080. Direct service ports are for local development only.

---

## Authentication

### JWT Flow

1. **Register** – `POST /auth/register` with email, password, and account name.
2. **Verify email** – Click the link in the verification email (`GET /auth/verify-email?token=...`).
3. **Login** – `POST /auth/login` returns an `access_token` (15 min) and `refresh_token` (7 days).
4. **Use the token** – Add `Authorization: Bearer <access_token>` to every request.
5. **Refresh** – When the access token expires, call `POST /auth/refresh` with the refresh token.

```python
import requests

# Login
resp = requests.post("http://localhost:8080/auth/login", json={
    "email": "alice@acme.com",
    "password": "SecurePass123!",
})
tokens = resp.json()
access_token = tokens["access_token"]

# Authenticated request
headers = {"Authorization": f"Bearer {access_token}"}
costs = requests.get("http://localhost:8080/finops/costs/summary",
                     params={"start_date": "2024-01-01", "end_date": "2024-01-31"},
                     headers=headers)
print(costs.json())
```

### API Key Usage

Generate a key once and reuse it for programmatic/CI access:

```bash
# Create a key (requires JWT auth)
curl -X POST http://localhost:8080/auth/api-keys \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "CI Pipeline", "expires_in_days": 365}'

# Use the key
curl http://localhost:8080/finops/costs/summary \
  -H "X-API-Key: sk_live_abcdefghijklmnopqrstuvwxyz123456" \
  -G -d "start_date=2024-01-01" -d "end_date=2024-01-31"
```

> **Security note:** API keys are shown only once at creation. Store them in a secrets manager.

---

## Rate Limiting

Rate limits are enforced per user per minute based on subscription tier:

| Plan | Price | Requests/min | Cloud Accounts | Databases |
|---|---|---|---|---|
| Free | $0 | 100 | 1 | 2 |
| Base | $10/mo | 500 | 3 | 5 |
| Pro | $20/mo | 2,000 | 10 | Unlimited |
| Enterprise | $50/mo | 10,000 | Unlimited | Unlimited |

When the limit is exceeded the API returns `429 Too Many Requests` with a `Retry-After` header indicating seconds until reset.

---

## Interactive Documentation

The AI Query Engine (FastAPI) hosts interactive API documentation:

| URL | Description |
|---|---|
| `http://localhost:8084/docs` | Swagger UI – try endpoints in the browser |
| `http://localhost:8084/redoc` | ReDoc – clean reference documentation |
| `http://localhost:8084/openapi.yaml` | Raw OpenAPI 3.0 YAML spec |

---

## Quick Start Examples

### Python

```python
import requests

BASE = "http://localhost:8080"

# 1. Login
tokens = requests.post(f"{BASE}/auth/login", json={
    "email": "alice@acme.com",
    "password": "SecurePass123!",
}).json()

headers = {"Authorization": f"Bearer {tokens['access_token']}"}

# 2. Get cost summary
summary = requests.get(f"{BASE}/finops/costs/summary",
    params={"start_date": "2024-01-01", "end_date": "2024-01-31"},
    headers=headers,
).json()
print(f"Total cost: ${summary['total_cost']:.2f}")

# 3. Run a natural language query
result = requests.post(f"{BASE}/query/execute",
    json={
        "database_connection_id": "<your-connection-id>",
        "query_text": "Show me monthly revenue for the last 6 months",
    },
    headers=headers,
).json()
print(f"Chart: {result['chart_type']}")
print(f"SQL: {result['generated_sql']}")
```

### JavaScript

```javascript
const BASE = 'http://localhost:8080';

// 1. Login
const { access_token } = await fetch(`${BASE}/auth/login`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: 'alice@acme.com', password: 'SecurePass123!' }),
}).then(r => r.json());

const headers = { Authorization: `Bearer ${access_token}` };

// 2. Get cost summary
const summary = await fetch(
  `${BASE}/finops/costs/summary?start_date=2024-01-01&end_date=2024-01-31`,
  { headers }
).then(r => r.json());
console.log('Total cost:', summary.total_cost);

// 3. Run a natural language query
const result = await fetch(`${BASE}/query/execute`, {
  method: 'POST',
  headers: { ...headers, 'Content-Type': 'application/json' },
  body: JSON.stringify({
    database_connection_id: '<your-connection-id>',
    query_text: 'Show me monthly revenue for the last 6 months',
  }),
}).then(r => r.json());
console.log('Chart type:', result.chart_type);
console.log('Generated SQL:', result.generated_sql);
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

const base = "http://localhost:8080"

func main() {
    // 1. Login
    loginBody, _ := json.Marshal(map[string]string{
        "email":    "alice@acme.com",
        "password": "SecurePass123!",
    })
    resp, _ := http.Post(base+"/auth/login", "application/json", bytes.NewBuffer(loginBody))
    var tokens map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&tokens)
    token := tokens["access_token"].(string)
    resp.Body.Close()

    // 2. Get cost summary
    req, _ := http.NewRequest("GET",
        base+"/finops/costs/summary?start_date=2024-01-01&end_date=2024-01-31", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, _ = http.DefaultClient.Do(req)
    var summary map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&summary)
    fmt.Printf("Total cost: $%.2f\n", summary["total_cost"])
    resp.Body.Close()

    // 3. Run a natural language query
    queryBody, _ := json.Marshal(map[string]string{
        "database_connection_id": "<your-connection-id>",
        "query_text":             "Show me monthly revenue for the last 6 months",
    })
    req, _ = http.NewRequest("POST", base+"/query/execute", bytes.NewBuffer(queryBody))
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")
    resp, _ = http.DefaultClient.Do(req)
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    fmt.Printf("Chart type: %v\n", result["chart_type"])
    fmt.Printf("Generated SQL: %v\n", result["generated_sql"])
    resp.Body.Close()
}
```

---

## OpenAPI Specification

The full OpenAPI 3.0 specification is at [`docs/openapi.yaml`](./openapi.yaml).

It covers all endpoints across all services with:
- Request/response schemas
- Authentication requirements
- Rate limiting notes
- Stripe webhook payload examples
- SDK code samples (Python, JavaScript, Go) for `POST /query/execute`
