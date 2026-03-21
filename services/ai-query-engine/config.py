"""Backward-compatible config shim - delegates to services.ai_query_engine_config."""
from services.ai_query_engine_config import Settings, get_settings

# Re-export for any code that imports from config directly
settings = get_settings()

__all__ = ["Settings", "get_settings", "settings"]
