"""Unit tests for Redis-based query result caching.
Requirements: 20.1, 20.2, 20.3
"""
import json
import pytest
from unittest.mock import MagicMock, patch
from services.cache import (
    make_cache_key,
    get_cached_result,
    set_cached_result,
    set_cached_result_with_index,
    invalidate_connection_cache,
)


class TestMakeCacheKey:
    """Tests for cache key generation."""

    def test_returns_string(self):
        key = make_cache_key("SELECT * FROM t", "conn-123")
        assert isinstance(key, str)

    def test_starts_with_query_prefix(self):
        key = make_cache_key("SELECT * FROM t", "conn-123")
        assert key.startswith("query:")

    def test_same_inputs_same_key(self):
        k1 = make_cache_key("SELECT * FROM t", "conn-123")
        k2 = make_cache_key("SELECT * FROM t", "conn-123")
        assert k1 == k2

    def test_different_queries_different_keys(self):
        k1 = make_cache_key("SELECT * FROM t", "conn-123")
        k2 = make_cache_key("SELECT COUNT(*) FROM t", "conn-123")
        assert k1 != k2

    def test_different_connections_different_keys(self):
        k1 = make_cache_key("SELECT * FROM t", "conn-111")
        k2 = make_cache_key("SELECT * FROM t", "conn-222")
        assert k1 != k2

    def test_case_insensitive_query(self):
        k1 = make_cache_key("SELECT * FROM t", "conn-123")
        k2 = make_cache_key("select * from t", "conn-123")
        assert k1 == k2

    def test_whitespace_stripped(self):
        k1 = make_cache_key("SELECT * FROM t", "conn-123")
        k2 = make_cache_key("  SELECT * FROM t  ", "conn-123")
        assert k1 == k2


class TestGetCachedResult:
    """Tests for get_cached_result with mocked Redis."""

    def test_returns_none_when_redis_unavailable(self):
        with patch("services.cache.get_redis_client", return_value=None):
            result = get_cached_result("query", "conn-1")
            assert result is None

    def test_returns_none_on_cache_miss(self):
        mock_client = MagicMock()
        mock_client.get.return_value = None
        with patch("services.cache.get_redis_client", return_value=mock_client):
            result = get_cached_result("query", "conn-1")
            assert result is None

    def test_returns_parsed_json_on_cache_hit(self):
        cached_data = {"chartType": "metric", "data": [42], "labels": ["total"]}
        mock_client = MagicMock()
        mock_client.get.return_value = json.dumps(cached_data)
        with patch("services.cache.get_redis_client", return_value=mock_client):
            result = get_cached_result("query", "conn-1")
            assert result == cached_data

    def test_returns_none_on_redis_error(self):
        mock_client = MagicMock()
        mock_client.get.side_effect = Exception("Redis error")
        with patch("services.cache.get_redis_client", return_value=mock_client):
            result = get_cached_result("query", "conn-1")
            assert result is None


class TestSetCachedResult:
    """Tests for set_cached_result with mocked Redis."""

    def test_does_nothing_when_redis_unavailable(self):
        with patch("services.cache.get_redis_client", return_value=None):
            # Should not raise
            set_cached_result("query", "conn-1", {"data": 1}, ttl_seconds=300)

    def test_calls_setex_with_correct_ttl(self):
        mock_client = MagicMock()
        with patch("services.cache.get_redis_client", return_value=mock_client):
            data = {"chartType": "table", "data": []}
            set_cached_result("query", "conn-1", data, ttl_seconds=300)
            mock_client.setex.assert_called_once()
            args = mock_client.setex.call_args[0]
            assert args[1] == 300  # TTL
            assert json.loads(args[2]) == data

    def test_handles_redis_error_gracefully(self):
        mock_client = MagicMock()
        mock_client.setex.side_effect = Exception("Redis error")
        with patch("services.cache.get_redis_client", return_value=mock_client):
            # Should not raise
            set_cached_result("query", "conn-1", {"data": 1})


class TestSetCachedResultWithIndex:
    """Tests for set_cached_result_with_index."""

    def test_stores_key_in_connection_set(self):
        mock_client = MagicMock()
        with patch("services.cache.get_redis_client", return_value=mock_client):
            set_cached_result_with_index("query", "conn-1", {"data": 1}, ttl_seconds=300)
            # Should call sadd to track the key
            mock_client.sadd.assert_called_once()
            set_key_arg = mock_client.sadd.call_args[0][0]
            assert "conn-1" in set_key_arg


class TestInvalidateConnectionCache:
    """Tests for invalidate_connection_cache."""

    def test_returns_zero_when_redis_unavailable(self):
        with patch("services.cache.get_redis_client", return_value=None):
            result = invalidate_connection_cache("conn-1")
            assert result == 0

    def test_deletes_tracked_keys(self):
        mock_client = MagicMock()
        mock_client.smembers.return_value = {"query:abc123", "query:def456"}
        mock_client.delete.return_value = 2
        with patch("services.cache.get_redis_client", return_value=mock_client):
            result = invalidate_connection_cache("conn-1")
            assert result == 2

    def test_returns_zero_when_no_cached_keys(self):
        mock_client = MagicMock()
        mock_client.smembers.return_value = set()
        with patch("services.cache.get_redis_client", return_value=mock_client):
            result = invalidate_connection_cache("conn-1")
            assert result == 0
