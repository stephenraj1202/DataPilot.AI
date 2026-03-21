"""Configuration loader for AI Query Engine - reads from .env or environment variables."""
import configparser
import os
from functools import lru_cache
from dataclasses import dataclass

from dotenv import load_dotenv

# Load .env from the service root (two levels up from this file)
_env_path = os.path.join(os.path.dirname(__file__), "..", ".env")
load_dotenv(dotenv_path=_env_path, override=False)  # config.ini takes precedence via explicit reads


@dataclass
class Settings:
    port: int = 8084
    gemini_api_key: str = ""
    mysql_host: str = "localhost"
    mysql_port: int = 3306
    mysql_user: str = "root"
    mysql_password: str = ""
    mysql_database: str = "finops_platform"
    aes_key: str = "default-32-byte-aes-key-change-me"
    query_timeout_seconds: int = 30


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Load settings from config.ini (primary) then environment variables (override)."""
    s = Settings()

    # Try to load from config.ini (two levels up from this file)
    config_paths = [
        os.path.join(os.path.dirname(__file__), "..", "..", "..", "config.ini"),
        os.path.join(os.path.dirname(__file__), "..", "config.ini"),
        "config.ini",
    ]
    cfg = configparser.ConfigParser()
    for path in config_paths:
        if os.path.exists(path):
            cfg.read(path)
            break

    if cfg.has_section("database"):
        s.mysql_host = cfg.get("database", "host", fallback=s.mysql_host)
        s.mysql_port = cfg.getint("database", "port", fallback=s.mysql_port)
        s.mysql_user = cfg.get("database", "username", fallback=s.mysql_user)
        s.mysql_password = cfg.get("database", "password", fallback=s.mysql_password)
        s.mysql_database = cfg.get("database", "database_name", fallback=s.mysql_database)

    if cfg.has_section("ai"):
        s.gemini_api_key = cfg.get("ai", "gemini_api_key", fallback=s.gemini_api_key)
        s.query_timeout_seconds = cfg.getint("ai", "timeout_seconds", fallback=s.query_timeout_seconds)

    if cfg.has_section("encryption"):
        s.aes_key = cfg.get("encryption", "aes_key", fallback=s.aes_key)

    # Environment variable overrides
    s.gemini_api_key = os.getenv("GEMINI_API_KEY", s.gemini_api_key)
    s.mysql_host = os.getenv("MYSQL_HOST", s.mysql_host)
    s.mysql_port = int(os.getenv("MYSQL_PORT", str(s.mysql_port)))
    s.mysql_user = os.getenv("MYSQL_USER", s.mysql_user)
    s.mysql_password = os.getenv("MYSQL_PASSWORD", s.mysql_password)
    s.mysql_database = os.getenv("MYSQL_DATABASE", s.mysql_database)
    s.aes_key = os.getenv("AES_KEY", s.aes_key)
    s.port = int(os.getenv("PORT", str(s.port)))

    return s
