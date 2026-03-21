"""Unit tests for AI Query Engine configuration loader.
Requirements: 13.1
"""
import pytest
from unittest.mock import patch
from services.ai_query_engine_config import Settings, get_settings


class TestSettings:
    """Tests for Settings dataclass defaults."""

    def test_default_port(self):
        s = Settings()
        assert s.port == 8084

    def test_default_query_timeout(self):
        s = Settings()
        assert s.query_timeout_seconds == 30

    def test_default_cache_ttl(self):
        s = Settings()
        assert s.cache_ttl_seconds == 300

    def test_default_redis_port(self):
        s = Settings()
        assert s.redis_port == 6379

    def test_default_mysql_port(self):
        s = Settings()
        assert s.mysql_port == 3306


class TestGetSettings:
    """Tests for get_settings() with environment variable overrides."""

    def test_returns_settings_instance(self):
        # Clear lru_cache to get fresh settings
        get_settings.cache_clear()
        s = get_settings()
        assert isinstance(s, Settings)

    def test_env_override_gemini_key(self):
        get_settings.cache_clear()
        with patch.dict("os.environ", {"GEMINI_API_KEY": "test-gemini-key"}):
            get_settings.cache_clear()
            s = get_settings()
            assert s.gemini_api_key == "test-gemini-key"
        get_settings.cache_clear()

    def test_env_override_redis_host(self):
        get_settings.cache_clear()
        with patch.dict("os.environ", {"REDIS_HOST": "redis-server"}):
            get_settings.cache_clear()
            s = get_settings()
            assert s.redis_host == "redis-server"
        get_settings.cache_clear()

    def test_env_override_mysql_host(self):
        get_settings.cache_clear()
        with patch.dict("os.environ", {"MYSQL_HOST": "db-server"}):
            get_settings.cache_clear()
            s = get_settings()
            assert s.mysql_host == "db-server"
        get_settings.cache_clear()

    def test_env_override_aes_key(self):
        get_settings.cache_clear()
        with patch.dict("os.environ", {"AES_KEY": "my-custom-aes-key"}):
            get_settings.cache_clear()
            s = get_settings()
            assert s.aes_key == "my-custom-aes-key"
        get_settings.cache_clear()
