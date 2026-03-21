"""Router for AI query execution, bookmarks, and email reports."""
import json
import logging
import smtplib
import ssl
import uuid
from datetime import date, datetime
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from typing import Any, Optional

from fastapi import APIRouter, Header, HTTPException
from pydantic import BaseModel

from services.ai_query_engine_config import get_settings
from services.cache import (
    get_cached_schema,
    invalidate_connection_cache,
    save_query_result,
    set_cached_schema,
)
from services.chart_selector import format_response, select_chart_type
from services.crypto import decrypt
from services.db import db_cursor
from services.query_executor import QueryTimeoutError, execute_query
from services.schema_extractor import extract_schema, schema_to_text
from services.sql_generator import generate_sql
from routers.trained_queries import find_trained_match

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/query", tags=["query"])


# ---------------------------------------------------------------------------
# Pydantic models
# ---------------------------------------------------------------------------

class QueryRequest(BaseModel):
    database_connection_id: str
    query_text: str
    user_id: Optional[str] = None
    account_id: Optional[str] = None


class BookmarkRequest(BaseModel):
    title: str
    connection_id: str
    query_text: str
    generated_sql: str
    chart_type: str
    labels: list
    data: list
    raw_data: list


class EmailReportRequest(BaseModel):
    recipient_email: str


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

class _SafeEncoder(json.JSONEncoder):
    def default(self, obj: Any) -> Any:
        if isinstance(obj, bytes):
            return obj.decode("utf-8", errors="replace")
        if isinstance(obj, (datetime, date)):
            return obj.isoformat()
        return super().default(obj)


def _dumps(obj: Any) -> str:
    return json.dumps(obj, cls=_SafeEncoder)


def _log_query(
    user_id: str,
    connection_id: str,
    query_text: str,
    generated_sql: Optional[str],
    execution_time_ms: Optional[int],
    result_count: Optional[int],
    query_status: str,
    error_message: Optional[str] = None,
) -> Optional[str]:
    log_id = str(uuid.uuid4())
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                INSERT INTO query_logs
                    (id, user_id, database_connection_id, query_text, generated_sql,
                     execution_time_ms, result_count, status, error_message, created_at)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                """,
                (log_id, user_id, connection_id, query_text, generated_sql,
                 execution_time_ms, result_count, query_status, error_message, datetime.utcnow()),
            )
        return log_id
    except Exception as e:
        logger.warning(f"Failed to log query: {e}")
        return None


def _get_connection(connection_id: str) -> dict:
    try:
        with db_cursor() as cur:
            cur.execute(
                """SELECT id, db_type, host, port, database_name, username, encrypted_password
                   FROM database_connections WHERE id = %s AND deleted_at IS NULL""",
                (connection_id,),
            )
            row = cur.fetchone()
    except Exception as e:
        logger.error(f"DB error fetching connection: {e}")
        raise HTTPException(status_code=500, detail="Database error")
    if not row:
        raise HTTPException(status_code=404, detail="Database connection not found")
    return row


def _send_email(to_email: str, subject: str, body_html: str) -> None:
    import configparser, os
    cfg = configparser.ConfigParser()
    for p in [os.path.join(os.path.dirname(__file__), "..", "..", "..", "config.ini"), "config.ini"]:
        if os.path.exists(p):
            cfg.read(p)
            break
    host = cfg.get("mail", "default_smtp_host", fallback="smtp.gmail.com")
    port = cfg.getint("mail", "default_smtp_port", fallback=587)
    username = cfg.get("mail", "smtp_username", fallback="")
    password = cfg.get("mail", "smtp_password", fallback="")
    from_email = cfg.get("mail", "default_from_email", fallback=username)

    msg = MIMEMultipart("alternative")
    msg["Subject"] = subject
    msg["From"] = from_email
    msg["To"] = to_email
    msg.attach(MIMEText(body_html, "html"))

    context = ssl.create_default_context()
    with smtplib.SMTP(host, port) as server:
        server.ehlo()
        server.starttls(context=context)
        if username:
            server.login(username, password)
        server.sendmail(from_email, [to_email], msg.as_string())


# ---------------------------------------------------------------------------
# Query execution
# ---------------------------------------------------------------------------

@router.post("/execute")
async def execute_query_endpoint(req: QueryRequest, x_user_id: str = Header(None), x_account_id: str = Header(None)):
    """Execute a natural language query against an external database."""
    settings = get_settings()
    user_id = req.user_id or x_user_id
    account_id = req.account_id or x_account_id

    conn_row = _get_connection(req.database_connection_id)
    try:
        password = decrypt(conn_row["encrypted_password"], settings.aes_key)
    except Exception as e:
        raise HTTPException(status_code=500, detail="Credential decryption failed")

    db_type = conn_row["db_type"]
    host, port = conn_row["host"], conn_row["port"]
    database, username = conn_row["database_name"], conn_row["username"]

    # Schema: MySQL cache first, then extract
    schema_text = get_cached_schema(req.database_connection_id) or ""
    if not schema_text:
        try:
            tables = extract_schema(db_type, host, port, database, username, password)
            schema_text = schema_to_text(tables)
            if schema_text:
                set_cached_schema(req.database_connection_id, schema_text)
        except Exception as e:
            logger.warning(f"Schema extraction failed: {e}")

    # Check trained queries first — skip Gemini if a match is found
    trained_used = False
    trained_match = find_trained_match(account_id or "", req.database_connection_id, req.query_text) if account_id else None

    # Generate SQL
    if trained_match:
        generated_sql = trained_match["sql_query"]
        trained_used = True
        logger.info(f"Using trained query for: {req.query_text!r}")
    else:
        try:
            generated_sql = generate_sql(req.query_text, schema_text, db_type, settings.gemini_api_key)
        except Exception as e:
            _log_query(user_id, req.database_connection_id, req.query_text, None, None, None, "error", str(e))
            raise HTTPException(status_code=422, detail=f"SQL generation failed: {str(e)}")

    # Execute
    try:
        rows, elapsed_ms = execute_query(
            db_type=db_type, sql=generated_sql, host=host, port=port,
            database=database, username=username, password=password,
            timeout=settings.query_timeout_seconds,
        )
    except QueryTimeoutError as e:
        _log_query(user_id, req.database_connection_id, req.query_text,
                   generated_sql, settings.query_timeout_seconds * 1000, None, "timeout", str(e))
        raise HTTPException(status_code=408, detail=f"Query timed out after {settings.query_timeout_seconds}s.")
    except Exception as e:
        logger.error(f"Query execution failed: {e}")
        _log_query(user_id, req.database_connection_id, req.query_text,
                   generated_sql, None, None, "error", str(e))
        raise HTTPException(status_code=500, detail=f"Query execution failed: {str(e)}")

    chart_type = select_chart_type(rows)
    response = format_response(rows, chart_type, generated_sql, elapsed_ms)
    response["cached"] = False
    response["trained"] = trained_used

    if user_id:
        log_id = _log_query(user_id, req.database_connection_id, req.query_text,
                            generated_sql, elapsed_ms, len(rows), "success")
        if log_id:
            save_query_result(log_id, response)
            response["query_id"] = log_id

    return response


# ---------------------------------------------------------------------------
# Query history
# ---------------------------------------------------------------------------

@router.get("/history")
async def get_query_history(limit: int = 20, x_user_id: str = Header(None)):
    try:
        with db_cursor() as cur:
            if x_user_id:
                cur.execute(
                    """SELECT id, user_id, database_connection_id, query_text, generated_sql,
                              execution_time_ms, result_count, status, error_message, created_at
                       FROM query_logs WHERE user_id = %s ORDER BY created_at DESC LIMIT %s""",
                    (x_user_id, limit),
                )
            else:
                cur.execute(
                    """SELECT id, user_id, database_connection_id, query_text, generated_sql,
                              execution_time_ms, result_count, status, error_message, created_at
                       FROM query_logs ORDER BY created_at DESC LIMIT %s""",
                    (limit,),
                )
            rows = cur.fetchall()
    except Exception as e:
        raise HTTPException(status_code=500, detail="Database error")
    for r in rows:
        if hasattr(r.get("created_at"), "isoformat"):
            r["created_at"] = r["created_at"].isoformat()
    return rows


# ---------------------------------------------------------------------------
# Bookmarks
# ---------------------------------------------------------------------------

@router.post("/bookmarks", status_code=201)
async def create_bookmark(req: BookmarkRequest, x_user_id: str = Header(None)):
    if not x_user_id:
        raise HTTPException(status_code=401, detail="User ID required")
    bookmark_id = str(uuid.uuid4())
    try:
        with db_cursor() as cur:
            cur.execute(
                """INSERT INTO query_bookmarks
                   (id, user_id, connection_id, title, query_text, generated_sql,
                    chart_type, labels, data, raw_data, created_at, updated_at)
                   VALUES (%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,NOW(),NOW())""",
                (bookmark_id, x_user_id, req.connection_id, req.title,
                 req.query_text, req.generated_sql, req.chart_type,
                 _dumps(req.labels), _dumps(req.data), _dumps(req.raw_data)),
            )
    except Exception as e:
        logger.error(f"Failed to save bookmark: {e}")
        raise HTTPException(status_code=500, detail="Failed to save bookmark")
    return {"id": bookmark_id, "title": req.title, "created_at": datetime.utcnow().isoformat()}


@router.get("/bookmarks")
async def list_bookmarks(x_user_id: str = Header(None)):
    if not x_user_id:
        raise HTTPException(status_code=401, detail="User ID required")
    try:
        with db_cursor() as cur:
            cur.execute(
                """SELECT id, title, query_text, generated_sql, chart_type,
                          labels, data, raw_data, connection_id, created_at
                   FROM query_bookmarks WHERE user_id = %s ORDER BY created_at DESC""",
                (x_user_id,),
            )
            rows = cur.fetchall()
    except Exception as e:
        raise HTTPException(status_code=500, detail="Database error")

    result = []
    for r in rows:
        result.append({
            "id": r["id"],
            "title": r["title"],
            "query_text": r["query_text"],
            "generated_sql": r["generated_sql"],
            "chart_type": r["chart_type"],
            "labels": json.loads(r["labels"]) if isinstance(r["labels"], str) else r["labels"],
            "data": json.loads(r["data"]) if isinstance(r["data"], str) else r["data"],
            "raw_data": json.loads(r["raw_data"]) if isinstance(r["raw_data"], str) else r["raw_data"],
            "connection_id": r["connection_id"],
            "created_at": r["created_at"].isoformat() if hasattr(r["created_at"], "isoformat") else str(r["created_at"]),
        })
    return result


@router.get("/bookmarks/{bookmark_id}/refresh")
async def refresh_bookmark(bookmark_id: str, x_user_id: str = Header(None)):
    """Re-execute the bookmark query and return live data."""
    if not x_user_id:
        raise HTTPException(status_code=401, detail="User ID required")
    try:
        with db_cursor() as cur:
            cur.execute(
                "SELECT query_text, connection_id FROM query_bookmarks WHERE id = %s AND user_id = %s",
                (bookmark_id, x_user_id),
            )
            row = cur.fetchone()
    except Exception:
        raise HTTPException(status_code=500, detail="Database error")
    if not row:
        raise HTTPException(status_code=404, detail="Bookmark not found")

    req = QueryRequest(
        database_connection_id=row["connection_id"],
        query_text=row["query_text"],
        user_id=x_user_id,
    )
    return await execute_query_endpoint(req, x_user_id=x_user_id)


@router.delete("/bookmarks/{bookmark_id}", status_code=204)
async def delete_bookmark(bookmark_id: str, x_user_id: str = Header(None)):
    if not x_user_id:
        raise HTTPException(status_code=401, detail="User ID required")
    try:
        with db_cursor() as cur:
            cur.execute(
                "DELETE FROM query_bookmarks WHERE id = %s AND user_id = %s",
                (bookmark_id, x_user_id),
            )
    except Exception:
        raise HTTPException(status_code=500, detail="Database error")


# ---------------------------------------------------------------------------
# Email report
# ---------------------------------------------------------------------------

@router.post("/bookmarks/{bookmark_id}/send-report")
async def send_bookmark_report(
    bookmark_id: str,
    req: EmailReportRequest,
    x_user_id: str = Header(None),
):
    """Send a bookmark result as an HTML email report."""
    if not x_user_id:
        raise HTTPException(status_code=401, detail="User ID required")
    try:
        with db_cursor() as cur:
            cur.execute(
                "SELECT title, query_text, generated_sql, chart_type, raw_data FROM query_bookmarks WHERE id = %s AND user_id = %s",
                (bookmark_id, x_user_id),
            )
            bm = cur.fetchone()
    except Exception:
        raise HTTPException(status_code=500, detail="Database error")
    if not bm:
        raise HTTPException(status_code=404, detail="Bookmark not found")

    raw_data = json.loads(bm["raw_data"]) if isinstance(bm["raw_data"], str) else bm["raw_data"]

    if raw_data:
        cols = list(raw_data[0].keys())
        header_html = "".join(
            f"<th style='padding:8px;border:1px solid #ddd;background:#f0f4ff;text-align:left'>{c}</th>"
            for c in cols
        )
        rows_html = "".join(
            "<tr>" + "".join(
                f"<td style='padding:8px;border:1px solid #ddd'>{row.get(c, '')}</td>" for c in cols
            ) + "</tr>"
            for row in raw_data[:100]
        )
        table_html = (
            f"<table style='border-collapse:collapse;width:100%;font-size:13px'>"
            f"<thead><tr>{header_html}</tr></thead><tbody>{rows_html}</tbody></table>"
        )
    else:
        table_html = "<p>No data available.</p>"

    body_html = f"""<html><body style='font-family:Arial,sans-serif;color:#333;max-width:900px;margin:auto'>
      <h2 style='color:#2563eb'>Query Report: {bm['title']}</h2>
      <p><strong>Query:</strong> {bm['query_text']}</p>
      <p><strong>Chart type:</strong> {bm['chart_type']}</p>
      <details><summary style='cursor:pointer;color:#666;margin-bottom:8px'>Generated SQL</summary>
        <pre style='background:#f5f5f5;padding:12px;border-radius:4px;overflow:auto'>{bm['generated_sql']}</pre>
      </details>
      <h3>Results (up to 100 rows)</h3>{table_html}
      <p style='color:#999;font-size:11px;margin-top:24px'>Sent from FinOps Platform</p>
    </body></html>"""

    try:
        _send_email(req.recipient_email, f"Query Report: {bm['title']}", body_html)
    except Exception as e:
        logger.error(f"Email send failed: {e}")
        raise HTTPException(status_code=500, detail=f"Failed to send email: {str(e)}")

    return {"message": f"Report sent to {req.recipient_email}"}


# ---------------------------------------------------------------------------
# Suggested questions
# ---------------------------------------------------------------------------

@router.get("/suggestions/{connection_id}")
async def get_suggestions(connection_id: str, x_user_id: str = Header(None)):
    """
    Return 5 suggested natural-language questions based on the cached schema.
    Each suggestion carries a hint about the expected chart type.
    Falls back to generic suggestions when no schema is available.
    """
    schema_text = get_cached_schema(connection_id) or ""

    # Parse table names from schema text  (format: tableName(col type, ...))
    import re
    table_names = re.findall(r"^(\w+)\(", schema_text, re.MULTILINE)

    if not table_names:
        # Generic fallback suggestions
        return [
            {"question": "Show me the total count of all records", "chart_hint": "metric"},
            {"question": "Show distribution by type or category", "chart_hint": "pie"},
            {"question": "Show top 10 records by most recent date", "chart_hint": "table"},
            {"question": "Show count grouped by status", "chart_hint": "bar"},
            {"question": "Show trend over time", "chart_hint": "line"},
        ]

    # Build schema-aware suggestions using table and column names
    suggestions = []
    used = set()

    # Helper to pick a table not yet used
    def pick(preferred=None):
        if preferred and preferred in table_names and preferred not in used:
            used.add(preferred)
            return preferred
        for t in table_names:
            if t not in used:
                used.add(t)
                return t
        return table_names[0]

    # Look for tables with common naming patterns
    time_tables = [t for t in table_names if any(k in t.lower() for k in ("log", "event", "order", "transaction", "history", "audit", "cost"))]
    category_tables = [t for t in table_names if any(k in t.lower() for k in ("user", "account", "customer", "product", "service", "plan", "role"))]
    count_tables = [t for t in table_names if any(k in t.lower() for k in ("subscription", "invoice", "billing", "payment", "recommendation"))]

    # 1. Pie chart — distribution by category
    t = pick(category_tables[0] if category_tables else None)
    suggestions.append({
        "question": f"Show distribution of {t} by type or status as a pie chart",
        "chart_hint": "pie",
    })

    # 2. Bar chart — top N grouped
    t = pick(category_tables[1] if len(category_tables) > 1 else None)
    suggestions.append({
        "question": f"Show top 10 {t} records grouped by category with counts",
        "chart_hint": "bar",
    })

    # 3. Line chart — trend over time
    t = pick(time_tables[0] if time_tables else None)
    suggestions.append({
        "question": f"Show the count of {t} created per month over the last year",
        "chart_hint": "line",
    })

    # 4. Table — full detail view
    t = pick(count_tables[0] if count_tables else None)
    suggestions.append({
        "question": f"List all {t} records with their details and status",
        "chart_hint": "table",
    })

    # 5. Metric — single aggregate
    t = pick()
    suggestions.append({
        "question": f"What is the total count of {t}?",
        "chart_hint": "metric",
    })

    return suggestions
