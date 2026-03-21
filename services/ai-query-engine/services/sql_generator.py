"""Natural language to SQL conversion using Gemini via LangChain."""
import logging
import re
from typing import Optional

try:
    from langchain_google_genai import ChatGoogleGenerativeAI
    from langchain_core.messages import HumanMessage, SystemMessage
except ImportError:
    ChatGoogleGenerativeAI = None  # type: ignore
    HumanMessage = None  # type: ignore
    SystemMessage = None  # type: ignore

logger = logging.getLogger(__name__)

DIALECT_HINTS = {
    "postgresql": "Use PostgreSQL syntax. Use double quotes for identifiers if needed.",
    "mysql": "Use MySQL syntax. Use backticks for identifiers if needed.",
    "mongodb": "Generate a MongoDB aggregation pipeline as a JSON array. Do not generate SQL.",
    "sqlserver": "Use Microsoft SQL Server T-SQL syntax. Use square brackets for identifiers if needed.",
}


def _build_prompt(query_text: str, schema_text: str, db_type: str) -> str:
    dialect_hint = DIALECT_HINTS.get(db_type.lower(), "Use standard SQL syntax.")
    return (
        f"You are an expert SQL query generator.\n"
        f"Database type: {db_type}\n"
        f"{dialect_hint}\n\n"
        f"Database schema:\n{schema_text}\n\n"
        f"Convert the following natural language query to a valid SQL query.\n"
        f"Return ONLY the SQL query, no explanation, no markdown code blocks.\n\n"
        f"Query: {query_text}"
    )


def generate_sql_gemini(query_text: str, schema_text: str, db_type: str, api_key: str) -> str:
    """Generate SQL using Gemini via LangChain."""
    llm = ChatGoogleGenerativeAI(
        model="gemini-2.0-flash-001",
        google_api_key=api_key,
        temperature=0,
    )
    prompt = _build_prompt(query_text, schema_text, db_type)
    messages = [
        SystemMessage(content="You are an expert SQL query generator. Return only the SQL query."),
        HumanMessage(content=prompt),
    ]
    response = llm.invoke(messages)
    sql = response.content.strip()
    # Strip markdown code fences if present
    sql = re.sub(r"^```[a-z]*\n?", "", sql, flags=re.IGNORECASE)
    sql = re.sub(r"\n?```$", "", sql)
    return sql.strip()


def generate_sql_mock(query_text: str, schema_text: str, db_type: str) -> str:
    """Mock SQL generator used when GEMINI_API_KEY is not set."""
    query_lower = query_text.lower()

    first_table = "data"
    if schema_text:
        first_line = schema_text.split("\n")[0]
        match = re.match(r"(\w+)\(", first_line)
        if match:
            first_table = match.group(1)

    if "count" in query_lower or "how many" in query_lower:
        return f"SELECT COUNT(*) AS total FROM {first_table}"
    elif "sum" in query_lower or "total" in query_lower:
        return f"SELECT SUM(amount) AS total FROM {first_table}"
    elif "average" in query_lower or "avg" in query_lower:
        return f"SELECT AVG(amount) AS average FROM {first_table}"
    elif "top" in query_lower or "limit" in query_lower:
        limit = 10
        m = re.search(r"top\s+(\d+)", query_lower)
        if m:
            limit = int(m.group(1))
        return f"SELECT * FROM {first_table} LIMIT {limit}"
    elif "group by" in query_lower or "by " in query_lower:
        return f"SELECT category, COUNT(*) AS count FROM {first_table} GROUP BY category ORDER BY count DESC"
    else:
        return f"SELECT * FROM {first_table} LIMIT 100"


def generate_sql(query_text: str, schema_text: str, db_type: str, api_key: Optional[str] = None) -> str:
    """
    Generate SQL from natural language.
    Uses Gemini via LangChain if api_key is provided, otherwise falls back to mock generator.
    """
    if api_key and "your-gemini-key" not in api_key and len(api_key) > 10:
        try:
            return generate_sql_gemini(query_text, schema_text, db_type, api_key)
        except Exception as e:
            logger.warning(f"Gemini SQL generation failed, using mock: {e}")

    logger.info("Using mock SQL generator (no valid GEMINI_API_KEY)")
    return generate_sql_mock(query_text, schema_text, db_type)
