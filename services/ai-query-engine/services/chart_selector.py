"""Determine appropriate chart type and format query results for visualization."""
import re
from typing import Any, Dict, List, Optional


# Column name patterns that suggest time series data
_TIME_PATTERNS = re.compile(
    r"\b(date|time|month|year|week|day|hour|period|timestamp|created_at|updated_at)\b",
    re.IGNORECASE,
)

# Column name patterns that suggest categorical/label data
_CATEGORY_PATTERNS = re.compile(
    r"\b(name|category|type|label|group|region|provider|service|status|country|city)\b",
    re.IGNORECASE,
)

# Column name patterns that suggest numeric/metric data
_NUMERIC_PATTERNS = re.compile(
    r"\b(count|total|sum|amount|cost|revenue|value|price|quantity|avg|average|percent|rate)\b",
    re.IGNORECASE,
)


def _is_numeric(value: Any) -> bool:
    """Check if a value is numeric."""
    if isinstance(value, (int, float)):
        return True
    if isinstance(value, str):
        try:
            float(value)
            return True
        except (ValueError, TypeError):
            return False
    return False


def select_chart_type(rows: List[Dict[str, Any]]) -> str:
    """
    Determine the most appropriate chart type based on result structure.

    Rules:
    - Single row, single numeric column → "metric"
    - Time-series column + numeric column → "line"
    - Category column + numeric column → "bar"
    - Exactly 2 columns (label + numeric) with ≤ 10 rows → "pie"
    - Everything else → "table"
    """
    if not rows:
        return "table"

    columns = list(rows[0].keys())
    num_cols = len(columns)

    # Single value → metric
    if len(rows) == 1 and num_cols == 1:
        val = list(rows[0].values())[0]
        if _is_numeric(val):
            return "metric"

    # Single row with multiple numeric columns → metric (first value)
    if len(rows) == 1 and num_cols <= 3:
        numeric_cols = [c for c in columns if _is_numeric(rows[0].get(c))]
        if len(numeric_cols) >= 1:
            return "metric"

    # Identify column roles
    time_cols = [c for c in columns if _TIME_PATTERNS.search(c)]
    category_cols = [c for c in columns if _CATEGORY_PATTERNS.search(c)]
    numeric_cols = [c for c in columns if _NUMERIC_PATTERNS.search(c)]

    # Fallback: detect numeric columns by value type
    if not numeric_cols:
        numeric_cols = [
            c for c in columns
            if all(_is_numeric(row.get(c)) for row in rows[:5] if row.get(c) is not None)
        ]

    # Time series → line chart
    if time_cols and numeric_cols:
        return "line"

    # Two columns: one label + one numeric → pie (if small dataset) or bar
    if num_cols == 2:
        label_col = columns[0]
        value_col = columns[1]
        if all(_is_numeric(row.get(value_col)) for row in rows[:5]):
            if len(rows) <= 10:
                return "pie"
            return "bar"

    # Category + numeric → bar
    if (category_cols or num_cols >= 2) and numeric_cols:
        # Only use bar if there's actually a non-numeric label column alongside numeric data
        non_numeric_cols = [c for c in columns if c not in numeric_cols]
        if non_numeric_cols and len(numeric_cols) >= 1 and num_cols == 2:
            return "bar"

    return "table"


def format_response(
    rows: List[Dict[str, Any]],
    chart_type: str,
    generated_sql: str,
    execution_time_ms: int,
) -> Dict[str, Any]:
    """
    Format query results into the standard response structure.

    Returns: {chartType, labels, data, rawData, generatedSql, executionTimeMs}
    """
    if not rows:
        return {
            "chartType": chart_type,
            "labels": [],
            "data": [],
            "rawData": [],
            "generatedSql": generated_sql,
            "executionTimeMs": execution_time_ms,
        }

    columns = list(rows[0].keys())

    if chart_type == "metric":
        # Return the first numeric value
        first_row = rows[0]
        value = list(first_row.values())[0]
        return {
            "chartType": "metric",
            "labels": [columns[0]],
            "data": [value],
            "rawData": rows,
            "generatedSql": generated_sql,
            "executionTimeMs": execution_time_ms,
        }

    if chart_type in ("line", "bar", "pie"):
        # First column as labels, second (or first numeric) as data
        label_col = columns[0]
        # Find the first numeric column (prefer second column)
        data_col = columns[1] if len(columns) > 1 else columns[0]
        for col in columns[1:]:
            if all(_is_numeric(row.get(col)) for row in rows[:5] if row.get(col) is not None):
                data_col = col
                break

        labels = [str(row.get(label_col, "")) for row in rows]
        data = [row.get(data_col) for row in rows]

        return {
            "chartType": chart_type,
            "labels": labels,
            "data": data,
            "rawData": rows,
            "generatedSql": generated_sql,
            "executionTimeMs": execution_time_ms,
        }

    # table: return all data
    return {
        "chartType": "table",
        "labels": columns,
        "data": [[row.get(c) for c in columns] for row in rows],
        "rawData": rows,
        "generatedSql": generated_sql,
        "executionTimeMs": execution_time_ms,
    }
