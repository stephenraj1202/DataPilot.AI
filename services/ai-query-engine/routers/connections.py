"""Router for database connection management endpoints."""
import logging
import uuid
from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Header, HTTPException, status
from pydantic import BaseModel

from services.ai_query_engine_config import get_settings
from services.crypto import decrypt, encrypt
from services.db import db_cursor
from services.schema_extractor import extract_schema

logger = logging.getLogger(__name__)
router = APIRouter(prefix="/query", tags=["connections"])

SUPPORTED_DB_TYPES = {"postgresql", "mysql", "mongodb", "sqlserver"}


class ConnectionRequest(BaseModel):
    account_id: Optional[str] = None
    connection_name: str
    db_type: str
    host: str
    port: int
    database_name: str
    username: str
    password: str
    ssl_enabled: bool = True


class ConnectionResponse(BaseModel):
    id: str
    account_id: str
    connection_name: str
    db_type: str
    host: str
    port: int
    database_name: str
    username: str
    ssl_enabled: bool
    status: str
    created_at: str


def _test_connection(db_type: str, host: str, port: int, database: str, username: str, password: str) -> None:
    """Test database connectivity. Raises an exception with a descriptive message on failure."""
    db_type = db_type.lower()
    try:
        if db_type == "postgresql":
            import psycopg2
            conn = psycopg2.connect(
                host=host, port=port, dbname=database, user=username, password=password,
                connect_timeout=5,
            )
            conn.close()

        elif db_type == "mysql":
            import pymysql
            conn = pymysql.connect(
                host=host, port=port, database=database, user=username, password=password,
                connect_timeout=5,
            )
            conn.close()

        elif db_type == "mongodb":
            from pymongo import MongoClient
            if username and password:
                uri = f"mongodb://{username}:{password}@{host}:{port}/{database}?authSource=admin"
            else:
                uri = f"mongodb://{host}:{port}/{database}"
            client = MongoClient(uri, serverSelectionTimeoutMS=5000)
            client.server_info()
            client.close()

        elif db_type == "sqlserver":
            import pyodbc
            conn_str = (
                f"DRIVER={{ODBC Driver 17 for SQL Server}};"
                f"SERVER={host},{port};DATABASE={database};"
                f"UID={username};PWD={password};Connection Timeout=5;"
            )
            conn = pyodbc.connect(conn_str, timeout=5)
            conn.close()

        else:
            raise ValueError(f"Unsupported database type: {db_type}")

    except ValueError:
        raise
    except Exception as e:
        raise ConnectionError(f"Connection to {db_type} at {host}:{port} failed: {str(e)}")


@router.get("/connections")
async def list_connections(x_account_id: str = Header(None)):
    """List all database connections for the account."""
    if not x_account_id:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="account_id is required")
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                SELECT id, account_id, connection_name, db_type, host, port,
                       database_name, username, ssl_enabled, status, created_at
                FROM database_connections
                WHERE account_id = %s AND deleted_at IS NULL
                ORDER BY created_at DESC
                """,
                (x_account_id,),
            )
            rows = cur.fetchall()
    except Exception as e:
        logger.error(f"Failed to list connections: {e!r}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Database error")

    return [
        ConnectionResponse(
            id=r["id"],
            account_id=r["account_id"],
            connection_name=r["connection_name"],
            db_type=r["db_type"],
            host=r["host"],
            port=r["port"],
            database_name=r["database_name"],
            username=r["username"],
            ssl_enabled=r["ssl_enabled"],
            status=r["status"],
            created_at=r["created_at"].isoformat() if hasattr(r["created_at"], "isoformat") else str(r["created_at"]),
        )
        for r in rows
    ]


@router.delete("/connections/{connection_id}", status_code=status.HTTP_204_NO_CONTENT)
async def delete_connection(connection_id: str, x_account_id: str = Header(None)):
    """Soft-delete a database connection."""
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                UPDATE database_connections
                SET deleted_at = %s
                WHERE id = %s AND (account_id = %s OR %s IS NULL) AND deleted_at IS NULL
                """,
                (datetime.utcnow(), connection_id, x_account_id, x_account_id),
            )
            if cur.rowcount == 0:
                raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Connection not found")
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to delete connection: {e!r}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Database error")


class UpdateConnectionRequest(BaseModel):
    connection_name: Optional[str] = None
    host: Optional[str] = None
    port: Optional[int] = None
    database_name: Optional[str] = None
    username: Optional[str] = None
    password: Optional[str] = None
    ssl_enabled: Optional[bool] = None


@router.put("/connections/{connection_id}")
async def update_connection(connection_id: str, req: UpdateConnectionRequest, x_account_id: str = Header(None)):
    """Update an existing database connection. Re-tests connectivity if host/port/credentials change."""
    try:
        with db_cursor() as cur:
            cur.execute(
                """
                SELECT db_type, host, port, database_name, username, encrypted_password, ssl_enabled
                FROM database_connections
                WHERE id = %s AND (account_id = %s OR %s IS NULL) AND deleted_at IS NULL
                """,
                (connection_id, x_account_id, x_account_id),
            )
            row = cur.fetchone()
    except Exception as e:
        logger.error(f"DB error fetching connection: {e!r}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Database error")

    if not row:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Connection not found")

    settings = get_settings()

    # Resolve final values (merge patch)
    new_host = req.host or row["host"]
    new_port = req.port or row["port"]
    new_db = req.database_name or row["database_name"]
    new_user = req.username or row["username"]
    new_ssl = req.ssl_enabled if req.ssl_enabled is not None else row["ssl_enabled"]
    new_name = req.connection_name or None

    # Resolve password
    if req.password:
        new_password = req.password
        new_enc_password = encrypt(req.password, settings.aes_key)
    else:
        new_enc_password = row["encrypted_password"]
        new_password = decrypt(row["encrypted_password"], settings.aes_key)

    # Re-test if connectivity-relevant fields changed
    connectivity_changed = any([req.host, req.port, req.database_name, req.username, req.password])
    if connectivity_changed:
        try:
            _test_connection(row["db_type"], new_host, new_port, new_db, new_user, new_password)
        except (ConnectionError, ValueError) as e:
            raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))

    # Invalidate schema cache
    from services.cache import invalidate_connection_cache
    invalidate_connection_cache(connection_id)

    try:
        with db_cursor() as cur:
            if new_name:
                cur.execute(
                    """
                    UPDATE database_connections
                    SET connection_name = %s, host = %s, port = %s, database_name = %s,
                        username = %s, encrypted_password = %s, ssl_enabled = %s, updated_at = %s
                    WHERE id = %s AND deleted_at IS NULL
                    """,
                    (new_name, new_host, new_port, new_db, new_user, new_enc_password, new_ssl,
                     datetime.utcnow(), connection_id),
                )
            else:
                cur.execute(
                    """
                    UPDATE database_connections
                    SET host = %s, port = %s, database_name = %s,
                        username = %s, encrypted_password = %s, ssl_enabled = %s, updated_at = %s
                    WHERE id = %s AND deleted_at IS NULL
                    """,
                    (new_host, new_port, new_db, new_user, new_enc_password, new_ssl,
                     datetime.utcnow(), connection_id),
                )
    except Exception as e:
        logger.error(f"Failed to update connection: {e!r}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Failed to update connection")

    return {"message": "connection updated"}


@router.post("/connections", response_model=ConnectionResponse, status_code=status.HTTP_201_CREATED)
async def add_connection(req: ConnectionRequest, x_account_id: str = Header(None)):
    """
    Add a new database connection.
    Tests connectivity before saving. Encrypts credentials using AES-256.
    Requirements: 8.1, 8.2, 8.3, 8.4, 8.6
    """
    # Resolve account_id from gateway-injected header if not in body
    account_id = req.account_id or x_account_id
    if not account_id:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="account_id is required")

    if req.db_type.lower() not in SUPPORTED_DB_TYPES:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Unsupported database type '{req.db_type}'. Supported: {', '.join(SUPPORTED_DB_TYPES)}",
        )

    # Test connectivity before saving (Requirement 8.2, 8.4)
    try:
        _test_connection(req.db_type, req.host, req.port, req.database_name, req.username, req.password)
    except ConnectionError as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))
    except ValueError as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))

    settings = get_settings()
    # Encrypt password using AES-256 (Requirement 8.3)
    encrypted_password = encrypt(req.password, settings.aes_key)

    connection_id = str(uuid.uuid4())
    now = datetime.utcnow()

    try:
        with db_cursor() as cur:
            cur.execute(
                """
                INSERT INTO database_connections
                    (id, account_id, db_type, connection_name, host, port, database_name,
                     username, encrypted_password, ssl_enabled, status, created_at, updated_at)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, 'active', %s, %s)
                """,
                (
                    connection_id, account_id, req.db_type.lower(), req.connection_name,
                    req.host, req.port, req.database_name, req.username,
                    encrypted_password, req.ssl_enabled, now, now,
                ),
            )
    except Exception as e:
        logger.error(f"Failed to save connection: {e!r}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Failed to save connection")

    return ConnectionResponse(
        id=connection_id,
        account_id=account_id,
        connection_name=req.connection_name,
        db_type=req.db_type.lower(),
        host=req.host,
        port=req.port,
        database_name=req.database_name,
        username=req.username,
        ssl_enabled=req.ssl_enabled,
        status="active",
        created_at=now.isoformat(),
    )


@router.get("/schema/{connection_id}")
async def get_schema(connection_id: str):
    """
    Extract and return schema metadata for a database connection.
    Requirements: 8.5, 8.6
    """
    settings = get_settings()

    try:
        with db_cursor() as cur:
            cur.execute(
                """
                SELECT id, db_type, host, port, database_name, username, encrypted_password
                FROM database_connections
                WHERE id = %s AND deleted_at IS NULL
                """,
                (connection_id,),
            )
            row = cur.fetchone()
    except Exception as e:
        logger.error(f"DB error fetching connection: {e}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Database error")

    if not row:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Connection not found")

    try:
        password = decrypt(row["encrypted_password"], settings.aes_key)
    except Exception as e:
        logger.error(f"Decryption error: {e}")
        raise HTTPException(status_code=status.HTTP_500_INTERNAL_SERVER_ERROR, detail="Credential decryption failed")

    try:
        tables = extract_schema(
            db_type=row["db_type"],
            host=row["host"],
            port=row["port"],
            database=row["database_name"],
            username=row["username"],
            password=password,
        )
    except Exception as e:
        logger.error(f"Schema extraction failed: {e}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Schema extraction failed: {str(e)}",
        )

    # Update last_schema_sync
    try:
        with db_cursor() as cur:
            cur.execute(
                "UPDATE database_connections SET last_schema_sync = %s WHERE id = %s",
                (datetime.utcnow(), connection_id),
            )
    except Exception:
        pass  # Non-critical

    # Invalidate MySQL schema cache for this connection
    from services.cache import invalidate_connection_cache
    invalidate_connection_cache(connection_id)

    return {"connection_id": connection_id, "tables": tables}
