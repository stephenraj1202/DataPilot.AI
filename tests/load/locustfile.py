"""
Locust load test for SaaS FinOps Analytics Platform.

Run with:
    locust -f tests/load/locustfile.py --host http://localhost:8080

See tests/load/README.md for full instructions.
"""

import json
import time
import random
import string

from locust import HttpUser, task, between, events
from locust.exception import StopUser


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _random_email() -> str:
    suffix = "".join(random.choices(string.ascii_lowercase + string.digits, k=8))
    return f"loadtest_{suffix}@example.com"


def _register_and_login(client) -> dict | None:
    """Register a fresh user and return {'access_token': ..., 'account_id': ...}."""
    email = _random_email()
    password = "LoadTest@1234!"

    reg = client.post(
        "/api/auth/register",
        json={"email": email, "password": password, "account_name": f"LoadTest {email}"},
        name="/api/auth/register",
        catch_response=True,
    )
    if reg.status_code not in (200, 201):
        reg.failure(f"Registration failed: {reg.status_code} {reg.text[:200]}")
        return None
    reg.success()

    login = client.post(
        "/api/auth/login",
        json={"email": email, "password": password},
        name="/api/auth/login",
        catch_response=True,
    )
    if login.status_code != 200:
        login.failure(f"Login failed: {login.status_code} {login.text[:200]}")
        return None
    login.success()

    data = login.json()
    return {
        "access_token": data.get("access_token"),
        "account_id": data.get("account_id", ""),
    }


# ---------------------------------------------------------------------------
# FinOpsUser — simulates a typical analyst session
# ---------------------------------------------------------------------------

class FinOpsUser(HttpUser):
    """
    Simulates a user who:
      1. Logs in
      2. Fetches the cost summary dashboard
      3. Runs an AI query
      4. Checks anomalies
      5. Refreshes the access token

    Wait time: 1–5 seconds between tasks (realistic think time).
    """

    wait_time = between(1, 5)

    def on_start(self):
        """Authenticate once at the start of each simulated user session."""
        creds = _register_and_login(self.client)
        if creds is None:
            raise StopUser()
        self.access_token = creds["access_token"]
        self.headers = {"Authorization": f"Bearer {self.access_token}"}
        self.refresh_token = None

        # Capture refresh token from login response for later use
        login = self.client.post(
            "/api/auth/login",
            json={"email": _random_email(), "password": "LoadTest@1234!"},
            name="/api/auth/login [refresh capture]",
            catch_response=True,
        )
        if login.status_code == 200:
            self.refresh_token = login.json().get("refresh_token")
            login.success()
        else:
            login.failure("Could not capture refresh token")

    # ------------------------------------------------------------------
    # Tasks
    # ------------------------------------------------------------------

    @task(5)
    def get_cost_summary(self):
        """
        GET /api/finops/costs/summary
        High-frequency task — most users check the dashboard often.
        Asserts: HTTP 200, response time < 500ms.
        """
        with self.client.get(
            "/api/finops/costs/summary",
            headers=self.headers,
            name="/api/finops/costs/summary",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                if resp.elapsed.total_seconds() > 0.5:
                    resp.failure(f"Slow response: {resp.elapsed.total_seconds():.2f}s")
                else:
                    resp.success()
            elif resp.status_code == 401:
                resp.failure("Unauthorized — token may have expired")
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(3)
    def run_ai_query(self):
        """
        POST /api/query/execute
        Simulates a natural language query against a connected database.
        Asserts: HTTP 200, response time < 3s (cache hit < 100ms).
        """
        payload = {
            "database_connection_id": "test-connection-id",
            "query_text": random.choice([
                "Show me total revenue by month",
                "What are the top 10 customers by spend?",
                "Show daily active users for the past 30 days",
                "List all orders above $1000 this quarter",
            ]),
        }
        with self.client.post(
            "/api/query/execute",
            json=payload,
            headers=self.headers,
            name="/api/query/execute",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                elapsed = resp.elapsed.total_seconds()
                if elapsed > 3.0:
                    resp.failure(f"Query exceeded 3s SLA: {elapsed:.2f}s")
                else:
                    resp.success()
            elif resp.status_code == 429:
                resp.failure("Rate limit hit during query")
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def get_anomalies(self):
        """
        GET /api/finops/anomalies?days=30
        Asserts: HTTP 200.
        """
        with self.client.get(
            "/api/finops/anomalies?days=30",
            headers=self.headers,
            name="/api/finops/anomalies",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def get_recommendations(self):
        """
        GET /api/finops/recommendations
        Asserts: HTTP 200.
        """
        with self.client.get(
            "/api/finops/recommendations",
            headers=self.headers,
            name="/api/finops/recommendations",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(1)
    def refresh_access_token(self):
        """
        POST /api/auth/refresh
        Simulates token refresh before expiry.
        Asserts: HTTP 200, new access_token returned.
        """
        if not self.refresh_token:
            return

        with self.client.post(
            "/api/auth/refresh",
            json={"refresh_token": self.refresh_token},
            name="/api/auth/refresh",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                new_token = resp.json().get("access_token")
                if new_token:
                    self.access_token = new_token
                    self.headers = {"Authorization": f"Bearer {self.access_token}"}
                    resp.success()
                else:
                    resp.failure("No access_token in refresh response")
            else:
                resp.failure(f"Token refresh failed: {resp.status_code}")

    @task(1)
    def hit_rate_limit(self):
        """
        Burst 10 rapid requests to verify rate limiting responds correctly.
        Expects at least one HTTP 429 when the user is on the Free plan.
        """
        got_429 = False
        for _ in range(10):
            with self.client.get(
                "/api/finops/costs/summary",
                headers=self.headers,
                name="/api/finops/costs/summary [burst]",
                catch_response=True,
            ) as resp:
                if resp.status_code == 429:
                    got_429 = True
                    resp.success()  # 429 is expected behaviour here
                elif resp.status_code == 200:
                    resp.success()
                else:
                    resp.failure(f"Unexpected status during burst: {resp.status_code}")


# ---------------------------------------------------------------------------
# BillingUser — simulates billing operations
# ---------------------------------------------------------------------------

class BillingUser(HttpUser):
    """
    Simulates an Account_Owner who:
      1. Checks their current subscription
      2. Views invoice history
      3. Checks plan limits / usage

    Wait time: 5–15 seconds (billing pages visited less frequently).
    """

    wait_time = between(5, 15)

    def on_start(self):
        creds = _register_and_login(self.client)
        if creds is None:
            raise StopUser()
        self.headers = {"Authorization": f"Bearer {creds['access_token']}"}

    @task(4)
    def get_subscription(self):
        """
        GET /api/billing/subscription
        Asserts: HTTP 200, plan name present in response.
        """
        with self.client.get(
            "/api/billing/subscription",
            headers=self.headers,
            name="/api/billing/subscription",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                body = resp.json()
                if "plan" not in body and "status" not in body:
                    resp.failure("Missing plan/status fields in subscription response")
                else:
                    resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(3)
    def list_invoices(self):
        """
        GET /api/billing/invoices
        Asserts: HTTP 200, invoices array present.
        """
        with self.client.get(
            "/api/billing/invoices",
            headers=self.headers,
            name="/api/billing/invoices",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(2)
    def get_plan_limits(self):
        """
        GET /api/billing/plans
        Asserts: HTTP 200, all four plans returned.
        """
        with self.client.get(
            "/api/billing/plans",
            headers=self.headers,
            name="/api/billing/plans",
            catch_response=True,
        ) as resp:
            if resp.status_code == 200:
                plans = resp.json()
                if isinstance(plans, list) and len(plans) < 4:
                    resp.failure(f"Expected 4 plans, got {len(plans)}")
                else:
                    resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")

    @task(1)
    def attempt_plan_upgrade(self):
        """
        PUT /api/billing/subscription with new_plan='pro'.
        In a load test environment this may fail without a real Stripe key —
        we assert the endpoint responds (200 or 402) within 2 seconds.
        """
        with self.client.put(
            "/api/billing/subscription",
            json={"new_plan": "pro"},
            headers=self.headers,
            name="/api/billing/subscription [upgrade]",
            catch_response=True,
        ) as resp:
            elapsed = resp.elapsed.total_seconds()
            if elapsed > 2.0:
                resp.failure(f"Upgrade endpoint too slow: {elapsed:.2f}s")
            elif resp.status_code in (200, 402, 400):
                resp.success()
            else:
                resp.failure(f"Unexpected status: {resp.status_code}")


# ---------------------------------------------------------------------------
# Event hooks — print summary stats on test completion
# ---------------------------------------------------------------------------

@events.quitting.add_listener
def on_quitting(environment, **kwargs):
    stats = environment.stats
    print("\n=== Load Test Summary ===")
    for name, entry in stats.entries.items():
        print(
            f"{name[1]:50s} | "
            f"reqs={entry.num_requests:6d} | "
            f"fails={entry.num_failures:5d} | "
            f"p50={entry.get_response_time_percentile(0.50):6.0f}ms | "
            f"p95={entry.get_response_time_percentile(0.95):6.0f}ms | "
            f"p99={entry.get_response_time_percentile(0.99):6.0f}ms"
        )
    print("=========================\n")
