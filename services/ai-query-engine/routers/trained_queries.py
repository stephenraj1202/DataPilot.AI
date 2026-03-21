"""Admin-managed trained queries: map natural language questions to pre-written SQL."""
import logging
import uuid
from datetime import datetime
from difflib import SequenceMatcher
from typing import Optional

from fastapi import APIRouter, Header, HTTPException
from pydantic import BaseModel

from services.db import db_cursor

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/query/trained", tags=["trained-queries"])


# ── Models ────────────────────────────────────────────────────────────────────

class TrainedQueryCreate(BaseModel):
    connection_id: str
    question: str
    sql_query: str
    description: Optional[str] = None


class TrainedQueryUpdate(BaseModel):
    question: Optional[str] = None
    sql_query: Optional[str] = None
    description: Optional[str] = None
    is_active: Optional[bool] = None


# ── Similarity helper ─────────────────────────────────────────────────────────

def _similarity(a: str, b: str) -> float:
    """Return 0-1 similarity ratio between two strings (case-insensitive)."""
    return SequenceMatcher(None, a.lower().strip(), b.lower().strip()).ratio()


def find_trained_match(
    account_id: str,
    connection_id: str,
    question: str,
    threshold: float = 0.75,
) -> Optional[dict]:
    """
    Look up a trained query that closely matches the given question.
    Returns the best match above `threshold`, or None.
    """
    try:
        with db_cursor() as cur:
            cur.execute(
                """SELECT id, question, sql_query, description
                   FROM trained_queries
                   WHERE account_id = %s AND connection_id = %s
                     AND is_active = TRUE AND deleted_at IS NULL""",
                (account_id, connection_id),
            )
            rows = cur.fetchall()
    except Exception as e:
        logger.warning(f"Trained query lookup failed: {e}")
        return None

    best_score = 0.0
    best_row = None
    for row in rows:
        score = _similarity(question, row["question"])
        if score > best_score:
            best_score = score
            best_row = row

    if best_row and best_score >= threshold:
        logger.info(f"Trained query matched (score={best_score:.2f}): {best_row['question']!r}")
        # Increment match counter
        try:
            with db_cursor() as cur:
                cur.execute(
                    "UPDATE trained_queries SET match_count = match_count + 1 WHERE id = %s",
                    (best_row["id"],),
                )
        except Exception:
            pass
        return best_row

    return None


# ── Admin CRUD endpoints ──────────────────────────────────────────────────────

@router.post("", status_code=201)
async def create_trained_query(
    req: TrainedQueryCreate,
    x_account_id: str = Header(None),
    x_user_id: str = Header(None),
):
    """Admin: create a trained query mapping."""
    if not x_account_id or not x_user_id:
        raise HTTPException(status_code=401, detail="Authentication required")

    tq_id = str(uuid.uuid4())
    try:
        with db_cursor() as cur:
            cur.execute(
                """INSERT INTO trained_queries
                   (id, account_id, connection_id, question, sql_query, description,
                    created_by, is_active, created_at, updated_at)
                   VALUES (%s,%s,%s,%s,%s,%s,%s,TRUE,%s,%s)""",
                (tq_id, x_account_id, req.connection_id, req.question.strip(),
                 req.sql_query.strip(), req.description, x_user_id,
                 datetime.utcnow(), datetime.utcnow()),
            )
    except Exception as e:
        logger.error(f"Failed to create trained query: {e}")
        raise HTTPException(status_code=500, detail="Failed to save trained query")

    return {"id": tq_id, "message": "Trained query created"}


@router.get("")
async def list_trained_queries(
    connection_id: Optional[str] = None,
    x_account_id: str = Header(None),
):
    """Admin: list all trained queries for the account."""
    if not x_account_id:
        raise HTTPException(status_code=401, detail="Authentication required")

    try:
        with db_cursor() as cur:
            if connection_id:
                cur.execute(
                    """SELECT id, connection_id, question, sql_query, description,
                              is_active, match_count, created_at, updated_at
                       FROM trained_queries
                       WHERE account_id = %s AND connection_id = %s AND deleted_at IS NULL
                       ORDER BY created_at DESC""",
                    (x_account_id, connection_id),
                )
            else:
                cur.execute(
                    """SELECT id, connection_id, question, sql_query, description,
                              is_active, match_count, created_at, updated_at
                       FROM trained_queries
                       WHERE account_id = %s AND deleted_at IS NULL
                       ORDER BY created_at DESC""",
                    (x_account_id,),
                )
            rows = cur.fetchall()
    except Exception as e:
        raise HTTPException(status_code=500, detail="Database error")

    return [
        {
            **row,
            "created_at": row["created_at"].isoformat() if hasattr(row["created_at"], "isoformat") else str(row["created_at"]),
            "updated_at": row["updated_at"].isoformat() if hasattr(row["updated_at"], "isoformat") else str(row["updated_at"]),
        }
        for row in rows
    ]


@router.put("/{tq_id}")
async def update_trained_query(
    tq_id: str,
    req: TrainedQueryUpdate,
    x_account_id: str = Header(None),
):
    """Admin: update a trained query."""
    if not x_account_id:
        raise HTTPException(status_code=401, detail="Authentication required")

    fields, values = [], []
    if req.question is not None:
        fields.append("question = %s"); values.append(req.question.strip())
    if req.sql_query is not None:
        fields.append("sql_query = %s"); values.append(req.sql_query.strip())
    if req.description is not None:
        fields.append("description = %s"); values.append(req.description)
    if req.is_active is not None:
        fields.append("is_active = %s"); values.append(req.is_active)

    if not fields:
        raise HTTPException(status_code=400, detail="No fields to update")

    fields.append("updated_at = %s"); values.append(datetime.utcnow())
    values.extend([tq_id, x_account_id])

    try:
        with db_cursor() as cur:
            cur.execute(
                f"UPDATE trained_queries SET {', '.join(fields)} WHERE id = %s AND account_id = %s AND deleted_at IS NULL",
                values,
            )
            if cur.rowcount == 0:
                raise HTTPException(status_code=404, detail="Trained query not found")
    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(status_code=500, detail="Failed to update trained query")

    return {"message": "Updated"}


@router.delete("/{tq_id}", status_code=204)
async def delete_trained_query(tq_id: str, x_account_id: str = Header(None)):
    """Admin: soft-delete a trained query."""
    if not x_account_id:
        raise HTTPException(status_code=401, detail="Authentication required")
    try:
        with db_cursor() as cur:
            cur.execute(
                "UPDATE trained_queries SET deleted_at = %s WHERE id = %s AND account_id = %s AND deleted_at IS NULL",
                (datetime.utcnow(), tq_id, x_account_id),
            )
    except Exception:
        raise HTTPException(status_code=500, detail="Database error")
