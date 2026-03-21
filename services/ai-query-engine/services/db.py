"""Primary MySQL database connection management."""
import logging
from contextlib import contextmanager
from typing import Generator

import pymysql
import pymysql.cursors

from services.ai_query_engine_config import get_settings

logger = logging.getLogger(__name__)

_pool = None


def get_connection() -> pymysql.connections.Connection:
    """Get a new connection to the primary MySQL database."""
    settings = get_settings()
    return pymysql.connect(
        host=settings.mysql_host,
        port=settings.mysql_port,
        user=settings.mysql_user,
        password=settings.mysql_password,
        database=settings.mysql_database,
        charset="utf8mb4",
        cursorclass=pymysql.cursors.DictCursor,
        autocommit=False,
        connect_timeout=5,
    )


@contextmanager
def db_cursor() -> Generator[pymysql.cursors.DictCursor, None, None]:
    """Context manager that yields a cursor and commits/rolls back automatically."""
    conn = get_connection()
    try:
        with conn.cursor() as cursor:
            yield cursor
        conn.commit()
    except Exception:
        conn.rollback()
        raise
    finally:
        conn.close()
