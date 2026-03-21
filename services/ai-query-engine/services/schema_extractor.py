"""Extract table/column metadata from external databases."""
import logging
from typing import Any, Dict, List

logger = logging.getLogger(__name__)


def extract_postgresql_schema(host: str, port: int, database: str, username: str, password: str) -> List[Dict]:
    """Extract schema from PostgreSQL database."""
    import psycopg2
    import psycopg2.extras

    conn = psycopg2.connect(
        host=host, port=port, dbname=database, user=username, password=password,
        connect_timeout=10,
    )
    try:
        with conn.cursor(cursor_factory=psycopg2.extras.DictCursor) as cur:
            cur.execute("""
                SELECT
                    t.table_name,
                    c.column_name,
                    c.data_type,
                    c.is_nullable,
                    c.character_maximum_length
                FROM information_schema.tables t
                JOIN information_schema.columns c
                    ON t.table_name = c.table_name AND t.table_schema = c.table_schema
                WHERE t.table_schema = 'public'
                  AND t.table_type = 'BASE TABLE'
                ORDER BY t.table_name, c.ordinal_position
            """)
            rows = cur.fetchall()
    finally:
        conn.close()

    return _group_columns(rows)


def extract_mysql_schema(host: str, port: int, database: str, username: str, password: str) -> List[Dict]:
    """Extract schema from MySQL database."""
    import pymysql
    import pymysql.cursors

    conn = pymysql.connect(
        host=host, port=port, database=database, user=username, password=password,
        charset="utf8mb4", cursorclass=pymysql.cursors.DictCursor, connect_timeout=10,
    )
    try:
        with conn.cursor() as cur:
            cur.execute("""
                SELECT
                    t.TABLE_NAME AS table_name,
                    c.COLUMN_NAME AS column_name,
                    c.DATA_TYPE AS data_type,
                    c.IS_NULLABLE AS is_nullable,
                    c.CHARACTER_MAXIMUM_LENGTH AS character_maximum_length
                FROM information_schema.TABLES t
                JOIN information_schema.COLUMNS c
                    ON t.TABLE_NAME = c.TABLE_NAME AND t.TABLE_SCHEMA = c.TABLE_SCHEMA
                WHERE t.TABLE_SCHEMA = %s
                  AND t.TABLE_TYPE = 'BASE TABLE'
                ORDER BY t.TABLE_NAME, c.ORDINAL_POSITION
            """, (database,))
            rows = cur.fetchall()
    finally:
        conn.close()

    return _group_columns(rows)


def extract_mongodb_schema(host: str, port: int, database: str, username: str, password: str) -> List[Dict]:
    """Extract collection/field metadata from MongoDB by sampling documents."""
    from pymongo import MongoClient

    if username and password:
        uri = f"mongodb://{username}:{password}@{host}:{port}/{database}?authSource=admin"
    else:
        uri = f"mongodb://{host}:{port}/{database}"

    client = MongoClient(uri, serverSelectionTimeoutMS=10000)
    try:
        db = client[database]
        tables = []
        for collection_name in db.list_collection_names():
            sample = db[collection_name].find_one()
            columns = []
            if sample:
                for field, value in sample.items():
                    columns.append({
                        "name": field,
                        "type": type(value).__name__,
                        "nullable": True,
                    })
            tables.append({"name": collection_name, "columns": columns})
        return tables
    finally:
        client.close()


def extract_sqlserver_schema(host: str, port: int, database: str, username: str, password: str) -> List[Dict]:
    """Extract schema from SQL Server database."""
    import pyodbc

    conn_str = (
        f"DRIVER={{ODBC Driver 17 for SQL Server}};"
        f"SERVER={host},{port};DATABASE={database};"
        f"UID={username};PWD={password};Connection Timeout=10;"
    )
    conn = pyodbc.connect(conn_str)
    try:
        cursor = conn.cursor()
        cursor.execute("""
            SELECT
                t.name AS table_name,
                c.name AS column_name,
                tp.name AS data_type,
                c.is_nullable,
                c.max_length AS character_maximum_length
            FROM sys.tables t
            JOIN sys.columns c ON t.object_id = c.object_id
            JOIN sys.types tp ON c.user_type_id = tp.user_type_id
            ORDER BY t.name, c.column_id
        """)
        rows = [
            {
                "table_name": row[0],
                "column_name": row[1],
                "data_type": row[2],
                "is_nullable": "YES" if row[3] else "NO",
                "character_maximum_length": row[4],
            }
            for row in cursor.fetchall()
        ]
    finally:
        conn.close()

    return _group_columns(rows)


def _group_columns(rows: List[Dict]) -> List[Dict]:
    """Group flat table/column rows into nested table objects."""
    tables: Dict[str, Dict] = {}
    for row in rows:
        tname = row["table_name"]
        if tname not in tables:
            tables[tname] = {"name": tname, "columns": []}
        col_type = row["data_type"]
        max_len = row.get("character_maximum_length")
        if max_len:
            col_type = f"{col_type}({max_len})"
        tables[tname]["columns"].append({
            "name": row["column_name"],
            "type": col_type,
            "nullable": row.get("is_nullable", "YES") == "YES",
        })
    return list(tables.values())


def extract_schema(db_type: str, host: str, port: int, database: str, username: str, password: str) -> List[Dict]:
    """Dispatch schema extraction to the appropriate database handler."""
    extractors = {
        "postgresql": extract_postgresql_schema,
        "mysql": extract_mysql_schema,
        "mongodb": extract_mongodb_schema,
        "sqlserver": extract_sqlserver_schema,
    }
    extractor = extractors.get(db_type.lower())
    if not extractor:
        raise ValueError(f"Unsupported database type: {db_type}")
    return extractor(host, port, database, username, password)


def schema_to_text(tables: List[Dict]) -> str:
    """Convert schema list to a compact text representation for LLM prompts."""
    lines = []
    for table in tables:
        col_defs = ", ".join(
            f"{c['name']} {c['type']}{'?' if c.get('nullable') else ''}"
            for c in table["columns"]
        )
        lines.append(f"{table['name']}({col_defs})")
    return "\n".join(lines)
