"""MySQL-based query result and schema caching (replaces Redis)."""
import json
import logging
from datetime import date, datetime
from typing import Any, Optional

from services.db import db_cursor

logger = logging.getLogger(__name__)


class _SafeEncoder(json.JSONEncoder):
    def default(self, obj: Any) -> Any:
        if isinstance(obj, bytes):
            return obj.decode("utf-8", errors="replace")
        if isinstance(obj, (datetime, date)):
            return obj.isoformat()
        return super().default(obj)


def _dumps(obj: Any) -> str:
    return json.dumps(obj, cls=_SafeEncoder)


# ---------------------------------------------------------------------------
# Schema cache (query_schemas table)
# ---------------------------------------------------------------------------

def get_cached_schema(connection_id: str) -> Optional[str]:
    """Return cached schema text for a connection, or None."""
    try:
        with db_cursor() as cur:
            cur.execute(
                "SELECT schema_text FROM query_schemas WHERE connection_id = %s",
                (connection_id,),
            )
            row = cur.fetchone()
            return row["schema_text"] if row else None
    except Exception as e:
        logger.warning(f"Schema cache get error: {e}")
        return None


def set_cached_schema(connection_id: str, schema_text: str) -> None:
    """Upsert schema text for a connection."""
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                INSERT INTO query_schemas (id, connection_id, schema_text, extracted_at)
                VALUES (UUID(), %s, %s, NOW())
                ON DUPLICATE KEY UPDATE schema_text = VALUES(schema_text), extracted_at = NOW()
                """,
                (connection_id, schema_text),
            )
    except Exception as e:
        logger.warning(f"Schema cache set error: {e}")


def invalidate_connection_cache(connection_id: str) -> None:
    """Remove cached schema for a connection (called on schema refresh)."""
    try:
        with db_cursor() as cur:
            cur.execute(
                "DELETE FROM query_schemas WHERE connection_id = %s",
                (connection_id,),
            )
    except Exception as e:
        logger.warning(f"Schema cache invalidation error: {e}")


# ---------------------------------------------------------------------------
# Result cache (query_results table, keyed by query_log_id)
# ---------------------------------------------------------------------------

def save_query_result(query_log_id: str, result: dict) -> None:
    """Persist query result to query_results table."""
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                INSERT INTO query_results
                    (id, query_log_id, chart_type, labels, data, raw_data, created_at)
                VALUES (UUID(), %s, %s, %s, %s, %s, NOW())
                """,
                (
                    query_log_id,
                    result.get("chart_type", "table"),
                    _dumps(result.get("labels", [])),
                    _dumps(result.get("data", [])),
                    _dumps(result.get("raw_data", [])),
                ),
            )
    except Exception as e:
        logger.warning(f"Result save error: {e}")


def get_query_result(query_log_id: str) -> Optional[dict]:
    """Fetch persisted result for a query log entry."""
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                SELECT chart_type, labels, data, raw_data
                FROM query_results WHERE query_log_id = %s
                """,
                (query_log_id,),
            )
            row = cur.fetchone()
            if not row:
                return None
            return {
                "chart_type": row["chart_type"],
                "labels": json.loads(row["labels"]),
                "data": json.loads(row["data"]),
                "raw_data": json.loads(row["raw_data"]),
            }
    except Exception as e:
        logger.warning(f"Result fetch error: {e}")
        return None
