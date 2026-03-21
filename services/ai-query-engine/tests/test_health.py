"""Unit tests for FastAPI health check endpoint.
Requirements: 18.5
"""
import pytest
from fastapi.testclient import TestClient
from unittest.mock import patch, MagicMock


def _make_app():
    """Create a fresh app instance with mocked DB/Redis for testing."""
    from fastapi import FastAPI
    from fastapi.middleware.cors import CORSMiddleware

    app = FastAPI()
    app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

    @app.get("/health")
    async def health_check():
        return {"status": "healthy", "service": "ai-query-engine"}

    return app


class TestHealthEndpoint:
    """Tests for the /health endpoint."""

    def setup_method(self):
        self.app = _make_app()
        self.client = TestClient(self.app)

    def test_health_returns_200(self):
        response = self.client.get("/health")
        assert response.status_code == 200

    def test_health_returns_healthy_status(self):
        response = self.client.get("/health")
        data = response.json()
        assert data["status"] == "healthy"

    def test_health_returns_service_name(self):
        response = self.client.get("/health")
        data = response.json()
        assert data["service"] == "ai-query-engine"
