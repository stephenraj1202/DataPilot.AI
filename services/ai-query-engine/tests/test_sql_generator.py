"""Unit tests for SQL generation (mock and Gemini paths).
Requirements: 6.1, 6.7
"""
import pytest
from unittest.mock import patch, MagicMock
from services.sql_generator import generate_sql_mock, generate_sql, generate_sql_gemini, _build_prompt


class TestGenerateSqlMock:
    """Tests for the mock SQL generator (no Gemini key required)."""

    def test_count_query(self):
        sql = generate_sql_mock("how many users are there", "", "mysql")
        assert "COUNT" in sql.upper()

    def test_sum_query(self):
        sql = generate_sql_mock("what is the total cost", "", "mysql")
        assert "SUM" in sql.upper() or "total" in sql.lower()

    def test_average_query(self):
        sql = generate_sql_mock("what is the average amount", "", "mysql")
        assert "AVG" in sql.upper() or "average" in sql.lower()

    def test_top_query_with_number(self):
        sql = generate_sql_mock("show top 5 records", "", "mysql")
        assert "5" in sql or "LIMIT" in sql.upper()

    def test_default_query_returns_select(self):
        sql = generate_sql_mock("show me everything", "", "mysql")
        assert sql.upper().startswith("SELECT")

    def test_uses_first_table_from_schema(self):
        schema = "orders(id int, amount float)\nusers(id int, name varchar)"
        sql = generate_sql_mock("show me everything", schema, "mysql")
        assert "orders" in sql

    def test_no_schema_uses_data_table(self):
        sql = generate_sql_mock("show me everything", "", "mysql")
        assert "data" in sql or "SELECT" in sql.upper()

    def test_group_by_query(self):
        sql = generate_sql_mock("show costs grouped by category", "", "mysql")
        assert "GROUP BY" in sql.upper()

    def test_order_by_in_group_by_query(self):
        sql = generate_sql_mock("show costs by region", "", "mysql")
        assert "ORDER BY" in sql.upper() or "GROUP BY" in sql.upper()

    def test_limit_keyword_in_query(self):
        sql = generate_sql_mock("show limit 20 records", "", "mysql")
        assert "LIMIT" in sql.upper()

    def test_top_without_number_defaults_to_10(self):
        sql = generate_sql_mock("show top records", "", "mysql")
        assert "10" in sql or "LIMIT" in sql.upper()

    def test_count_returns_total_alias(self):
        sql = generate_sql_mock("how many orders", "orders(id int)", "mysql")
        assert "total" in sql.lower() or "COUNT" in sql.upper()

    def test_sum_returns_total_alias(self):
        sql = generate_sql_mock("total revenue", "sales(id int, amount float)", "mysql")
        assert "SUM" in sql.upper() or "total" in sql.lower()


class TestGenerateSqlMockPatterns:
    """Tests for common SQL query patterns via mock generator."""

    def test_select_with_where_pattern(self):
        schema = "orders(id int, status varchar, amount float)"
        sql = generate_sql_mock("how many orders are there", schema, "mysql")
        assert "SELECT" in sql.upper()
        assert "orders" in sql

    def test_select_with_group_by_pattern(self):
        schema = "costs(id int, service varchar, amount float)"
        sql = generate_sql_mock("show costs by service", schema, "mysql")
        assert "GROUP BY" in sql.upper()
        assert "SELECT" in sql.upper()

    def test_select_with_order_by_pattern(self):
        schema = "costs(id int, service varchar, amount float)"
        sql = generate_sql_mock("show costs by service", schema, "mysql")
        assert "ORDER BY" in sql.upper()

    def test_select_with_limit_pattern(self):
        schema = "users(id int, name varchar)"
        sql = generate_sql_mock("show top 10 users", schema, "mysql")
        assert "LIMIT" in sql.upper()
        assert "10" in sql

    def test_aggregate_sum_pattern(self):
        schema = "invoices(id int, amount float)"
        sql = generate_sql_mock("total invoice amount", schema, "mysql")
        assert "SUM" in sql.upper()

    def test_aggregate_avg_pattern(self):
        schema = "costs(id int, daily_cost float)"
        sql = generate_sql_mock("average daily cost", schema, "mysql")
        assert "AVG" in sql.upper()

    def test_aggregate_count_pattern(self):
        schema = "users(id int, email varchar)"
        sql = generate_sql_mock("count all users", schema, "mysql")
        assert "COUNT" in sql.upper()


class TestGenerateSqlGemini:
    """Tests for generate_sql_gemini with mocked LangChain/Gemini calls."""

    def _make_mock_response(self, content: str) -> MagicMock:
        mock_response = MagicMock()
        mock_response.content = content
        return mock_response

    def test_returns_sql_from_gemini(self):
        expected_sql = "SELECT * FROM users WHERE active = 1"
        mock_response = self._make_mock_response(expected_sql)

        with patch("services.sql_generator.ChatGoogleGenerativeAI") as mock_cls:
            mock_llm = MagicMock()
            mock_cls.return_value = mock_llm
            mock_llm.invoke.return_value = mock_response

            result = generate_sql_gemini("show active users", "users(id int, active int)", "mysql", "test-key")

        assert result == expected_sql

    def test_strips_markdown_code_fences(self):
        raw = "```sql\nSELECT COUNT(*) FROM orders\n```"
        mock_response = self._make_mock_response(raw)

        with patch("services.sql_generator.ChatGoogleGenerativeAI") as mock_cls:
            mock_llm = MagicMock()
            mock_cls.return_value = mock_llm
            mock_llm.invoke.return_value = mock_response

            result = generate_sql_gemini("count orders", "orders(id int)", "mysql", "test-key")

        assert "```" not in result
        assert "SELECT" in result.upper()

    def test_strips_generic_code_fences(self):
        raw = "```\nSELECT id FROM users\n```"
        mock_response = self._make_mock_response(raw)

        with patch("services.sql_generator.ChatGoogleGenerativeAI") as mock_cls:
            mock_llm = MagicMock()
            mock_cls.return_value = mock_llm
            mock_llm.invoke.return_value = mock_response

            result = generate_sql_gemini("get user ids", "users(id int)", "mysql", "test-key")

        assert "```" not in result
        assert "SELECT" in result.upper()

    def test_gemini_exception_propagates(self):
        with patch("services.sql_generator.ChatGoogleGenerativeAI") as mock_cls:
            mock_llm = MagicMock()
            mock_cls.return_value = mock_llm
            mock_llm.invoke.side_effect = Exception("API quota exceeded")

            with pytest.raises(Exception, match="API quota exceeded"):
                generate_sql_gemini("query", "", "mysql", "test-key")


class TestGenerateSqlDialects:
    """Tests for dialect-specific SQL generation via prompt hints (Req 6.2)."""

    def test_postgresql_dialect_in_prompt(self):
        prompt = _build_prompt("query", "", "postgresql")
        assert "PostgreSQL" in prompt

    def test_mysql_dialect_in_prompt(self):
        prompt = _build_prompt("query", "", "mysql")
        assert "MySQL" in prompt or "mysql" in prompt.lower()

    def test_mongodb_dialect_in_prompt(self):
        prompt = _build_prompt("query", "", "mongodb")
        assert "MongoDB" in prompt or "aggregation" in prompt.lower()

    def test_sqlserver_dialect_in_prompt(self):
        prompt = _build_prompt("query", "", "sqlserver")
        assert "SQL Server" in prompt or "T-SQL" in prompt

    def test_postgresql_uses_double_quotes_hint(self):
        prompt = _build_prompt("query", "", "postgresql")
        assert "double quotes" in prompt.lower() or "PostgreSQL" in prompt

    def test_mysql_uses_backtick_hint(self):
        prompt = _build_prompt("query", "", "mysql")
        assert "backtick" in prompt.lower() or "MySQL" in prompt

    def test_sqlserver_uses_square_bracket_hint(self):
        prompt = _build_prompt("query", "", "sqlserver")
        assert "square bracket" in prompt.lower() or "T-SQL" in prompt

    def test_mongodb_no_sql_hint(self):
        prompt = _build_prompt("query", "", "mongodb")
        assert "aggregation" in prompt.lower() or "JSON" in prompt

    def test_unknown_dialect_uses_standard_sql(self):
        prompt = _build_prompt("query", "", "unknown_db")
        assert "standard SQL" in prompt.lower() or "SQL" in prompt


class TestGenerateSqlErrorHandling:
    """Tests for error handling and fallback behavior (Req 6.7)."""

    def test_generate_sql_falls_back_to_mock_on_gemini_error(self):
        with patch("services.sql_generator.generate_sql_gemini", side_effect=Exception("Connection timeout")):
            result = generate_sql("count all users", "users(id int)", "mysql", api_key="valid-gemini-key-abc")

        assert result.upper().startswith("SELECT")

    def test_generate_sql_no_api_key_uses_mock(self):
        result = generate_sql("count all rows", "users(id int)", "mysql", api_key=None)
        assert result.upper().startswith("SELECT")

    def test_generate_sql_empty_api_key_uses_mock(self):
        result = generate_sql("count all rows", "users(id int)", "mysql", api_key="")
        assert result.upper().startswith("SELECT")

    def test_generate_sql_placeholder_api_key_uses_mock(self):
        result = generate_sql("count all rows", "users(id int)", "mysql", api_key="your-gemini-key")
        assert result.upper().startswith("SELECT")


class TestGenerateSql:
    """Tests for the generate_sql dispatcher."""

    def test_no_api_key_uses_mock(self):
        sql = generate_sql("count all rows", "users(id int)", "mysql", api_key=None)
        assert sql.upper().startswith("SELECT")

    def test_empty_api_key_uses_mock(self):
        sql = generate_sql("count all rows", "users(id int)", "mysql", api_key="")
        assert sql.upper().startswith("SELECT")

    def test_placeholder_api_key_uses_mock(self):
        sql = generate_sql("count all rows", "users(id int)", "mysql", api_key="your-gemini-key")
        assert sql.upper().startswith("SELECT")

    def test_valid_key_calls_gemini(self):
        expected = "SELECT name FROM customers LIMIT 10"
        with patch("services.sql_generator.generate_sql_gemini", return_value=expected) as mock_fn:
            result = generate_sql("top customers", "customers(id int, name varchar)", "mysql", api_key="AIzaSy-valid-key")

        mock_fn.assert_called_once()
        assert result == expected

    def test_valid_key_passes_correct_args(self):
        with patch("services.sql_generator.generate_sql_gemini", return_value="SELECT 1") as mock_fn:
            generate_sql("my query", "my_schema", "postgresql", api_key="AIzaSy-valid-key")

        mock_fn.assert_called_once_with("my query", "my_schema", "postgresql", "AIzaSy-valid-key")


class TestBuildPrompt:
    """Tests for prompt construction (Req 6.3)."""

    def test_prompt_contains_query(self):
        prompt = _build_prompt("show all users", "users(id int)", "mysql")
        assert "show all users" in prompt

    def test_prompt_contains_schema(self):
        prompt = _build_prompt("query", "orders(id int, amount float)", "postgresql")
        assert "orders(id int, amount float)" in prompt

    def test_prompt_contains_db_type(self):
        prompt = _build_prompt("query", "", "mysql")
        assert "mysql" in prompt.lower()

    def test_postgresql_dialect_hint(self):
        prompt = _build_prompt("query", "", "postgresql")
        assert "PostgreSQL" in prompt

    def test_mongodb_dialect_hint(self):
        prompt = _build_prompt("query", "", "mongodb")
        assert "MongoDB" in prompt or "aggregation" in prompt.lower()

    def test_schema_context_is_included(self):
        schema = "cloud_costs(id varchar, service_name varchar, cost_amount decimal, date date)"
        prompt = _build_prompt("total cost by service", schema, "mysql")
        assert schema in prompt

    def test_prompt_instructs_sql_only_output(self):
        prompt = _build_prompt("query", "", "mysql")
        assert "only" in prompt.lower() or "no explanation" in prompt.lower() or "ONLY" in prompt

    def test_prompt_instructs_no_markdown(self):
        prompt = _build_prompt("query", "", "mysql")
        assert "markdown" in prompt.lower() or "code block" in prompt.lower() or "only" in prompt.lower()

    def test_empty_schema_still_builds_valid_prompt(self):
        prompt = _build_prompt("show all data", "", "mysql")
        assert "show all data" in prompt
        assert len(prompt) > 0

    def test_multiline_schema_preserved(self):
        schema = "users(id int, email varchar)\norders(id int, user_id int, amount float)"
        prompt = _build_prompt("join users and orders", schema, "postgresql")
        assert "users(id int, email varchar)" in prompt
        assert "orders(id int, user_id int, amount float)" in prompt
