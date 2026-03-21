"""Execute SQL queries against external databases with timeout enforcement."""
import logging
import signal
import time
from contextlib import contextmanager
from typing import Any, Dict, List, Optional, Tuple

logger = logging.getLogger(__name__)


class QueryTimeoutError(Exception):
    """Raised when a query exceeds the configured timeout."""
    pass


@contextmanager
def _timeout_context(seconds: int):
    """Unix-only signal-based timeout context manager."""
    def _handler(signum, frame):
        raise QueryTimeoutError(f"Query exceeded {seconds}s timeout")

    old_handler = signal.signal(signal.SIGALRM, _handler)
    signal.alarm(seconds)
    try:
        yield
    finally:
        signal.alarm(0)
        signal.signal(signal.SIGALRM, old_handler)


def _execute_with_thread_timeout(fn, timeout_seconds: int):
    """Execute fn() in a thread with timeout. Returns (result, elapsed_ms)."""
    import threading

    result_holder = [None]
    error_holder = [None]

    def target():
        try:
            result_holder[0] = fn()
        except Exception as e:
            error_holder[0] = e

    t = threading.Thread(target=target, daemon=True)
    start = time.time()
    t.start()
    t.join(timeout=timeout_seconds)
    elapsed_ms = int((time.time() - start) * 1000)

    if t.is_alive():
        raise QueryTimeoutError(f"Query exceeded {timeout_seconds}s timeout")
    if error_holder[0]:
        raise error_holder[0]
    return result_holder[0], elapsed_ms


def execute_postgresql(sql: str, host: str, port: int, database: str, username: str, password: str, timeout: int) -> Tuple[List[Dict], int]:
    import psycopg2
    import psycopg2.extras

    def run():
        conn = psycopg2.connect(
            host=host, port=port, dbname=database, user=username, password=password,
            connect_timeout=timeout, options=f"-c statement_timeout={timeout * 1000}",
        )
        try:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute(sql)
                return [dict(r) for r in cur.fetchall()]
        finally:
            conn.close()

    return _execute_with_thread_timeout(run, timeout)


def execute_mysql(sql: str, host: str, port: int, database: str, username: str, password: str, timeout: int) -> Tuple[List[Dict], int]:
    import pymysql
    import pymysql.cursors

    def run():
        conn = pymysql.connect(
            host=host, port=port, database=database, user=username, password=password,
            charset="utf8mb4", cursorclass=pymysql.cursors.DictCursor, connect_timeout=timeout,
        )
        try:
            with conn.cursor() as cur:
                cur.execute(f"SET SESSION MAX_EXECUTION_TIME={timeout * 1000}")
                cur.execute(sql)
                return cur.fetchall()
        finally:
            conn.close()

    return _execute_with_thread_timeout(run, timeout)


def execute_mongodb(query_json: str, host: str, port: int, database: str, username: str, password: str, timeout: int) -> Tuple[List[Dict], int]:
    """Execute a MongoDB aggregation pipeline (JSON string)."""
    import json
    from pymongo import MongoClient

    def run():
        if username and password:
            uri = f"mongodb://{username}:{password}@{host}:{port}/{database}?authSource=admin"
        else:
            uri = f"mongodb://{host}:{port}/{database}"
        client = MongoClient(uri, serverSelectionTimeoutMS=timeout * 1000)
        try:
            db = client[database]
            # query_json should be: {"collection": "name", "pipeline": [...]}
            payload = json.loads(query_json)
            collection_name = payload.get("collection", "")
            pipeline = payload.get("pipeline", [])
            results = list(db[collection_name].aggregate(pipeline, maxTimeMS=timeout * 1000))
            # Convert ObjectId etc. to strings
            return [
                {k: str(v) if not isinstance(v, (str, int, float, bool, type(None))) else v
                 for k, v in doc.items()}
                for doc in results
            ]
        finally:
            client.close()

    return _execute_with_thread_timeout(run, timeout)


def execute_sqlserver(sql: str, host: str, port: int, database: str, username: str, password: str, timeout: int) -> Tuple[List[Dict], int]:
    import pyodbc

    def run():
        conn_str = (
            f"DRIVER={{ODBC Driver 17 for SQL Server}};"
            f"SERVER={host},{port};DATABASE={database};"
            f"UID={username};PWD={password};Connection Timeout={timeout};"
        )
        conn = pyodbc.connect(conn_str, timeout=timeout)
        try:
            cursor = conn.cursor()
            cursor.execute(sql)
            columns = [desc[0] for desc in cursor.description]
            return [dict(zip(columns, row)) for row in cursor.fetchall()]
        finally:
            conn.close()

    return _execute_with_thread_timeout(run, timeout)


def execute_query(
    db_type: str,
    sql: str,
    host: str,
    port: int,
    database: str,
    username: str,
    password: str,
    timeout: int = 30,
) -> Tuple[List[Dict[str, Any]], int]:
    """
    Execute a query against the specified database type.
    Returns (rows, elapsed_ms).
    Raises QueryTimeoutError if timeout exceeded.
    """
    executors = {
        "postgresql": execute_postgresql,
        "mysql": execute_mysql,
        "mongodb": execute_mongodb,
        "sqlserver": execute_sqlserver,
    }
    executor = executors.get(db_type.lower())
    if not executor:
        raise ValueError(f"Unsupported database type: {db_type}")

    return executor(sql, host, port, database, username, password, timeout)
