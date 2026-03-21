"""Unit tests for query executor module.
Requirements: 6.4, 28.1, 28.2
"""
import pytest
from unittest.mock import patch, MagicMock
from services.query_executor import QueryTimeoutError, execute_query, _execute_with_thread_timeout


class TestQueryTimeoutError:
    """Tests for QueryTimeoutError exception."""

    def test_is_exception(self):
        err = QueryTimeoutError("timed out")
        assert isinstance(err, Exception)

    def test_message(self):
        err = QueryTimeoutError("Query exceeded 30s timeout")
        assert "30s" in str(err)


class TestExecuteWithThreadTimeout:
    """Tests for thread-based timeout mechanism."""

    def test_returns_result_on_success(self):
        def fast_fn():
            return [{"id": 1}]

        result, elapsed = _execute_with_thread_timeout(fast_fn, timeout_seconds=5)
        assert result == [{"id": 1}]
        assert elapsed >= 0

    def test_raises_timeout_error_when_slow(self):
        import time

        def slow_fn():
            time.sleep(10)
            return []

        with pytest.raises(QueryTimeoutError):
            _execute_with_thread_timeout(slow_fn, timeout_seconds=1)

    def test_propagates_function_exception(self):
        def failing_fn():
            raise ValueError("DB error")

        with pytest.raises(ValueError, match="DB error"):
            _execute_with_thread_timeout(failing_fn, timeout_seconds=5)


class TestExecuteQuery:
    """Tests for execute_query dispatcher."""

    def test_unsupported_db_type_raises(self):
        with pytest.raises(ValueError, match="Unsupported database type"):
            execute_query("oracle", "SELECT 1", "host", 1521, "db", "user", "pass", timeout=5)

    def test_mysql_executor_called(self):
        mock_rows = [{"id": 1, "name": "test"}]
        with patch("services.query_executor.execute_mysql", return_value=(mock_rows, 10)) as mock_exec:
            rows, elapsed = execute_query(
                "mysql", "SELECT * FROM t", "localhost", 3306, "testdb", "user", "pass", timeout=5
            )
            mock_exec.assert_called_once()
            assert rows == mock_rows
            assert elapsed == 10

    def test_postgresql_executor_called(self):
        mock_rows = [{"count": 5}]
        with patch("services.query_executor.execute_postgresql", return_value=(mock_rows, 20)) as mock_exec:
            rows, elapsed = execute_query(
                "postgresql", "SELECT COUNT(*) FROM t", "localhost", 5432, "testdb", "user", "pass", timeout=5
            )
            mock_exec.assert_called_once()
            assert rows == mock_rows

    def test_mongodb_executor_called(self):
        mock_rows = [{"_id": "abc", "count": 3}]
        with patch("services.query_executor.execute_mongodb", return_value=(mock_rows, 15)) as mock_exec:
            rows, elapsed = execute_query(
                "mongodb", '{"collection":"t","pipeline":[]}', "localhost", 27017, "testdb", "user", "pass", timeout=5
            )
            mock_exec.assert_called_once()
            assert rows == mock_rows

    def test_sqlserver_executor_called(self):
        mock_rows = [{"total": 100}]
        with patch("services.query_executor.execute_sqlserver", return_value=(mock_rows, 25)) as mock_exec:
            rows, elapsed = execute_query(
                "sqlserver", "SELECT SUM(amount) AS total FROM t", "localhost", 1433, "testdb", "user", "pass", timeout=5
            )
            mock_exec.assert_called_once()
            assert rows == mock_rows

    def test_case_insensitive_db_type(self):
        mock_rows = []
        with patch("services.query_executor.execute_mysql", return_value=(mock_rows, 5)):
            rows, _ = execute_query(
                "MySQL", "SELECT 1", "localhost", 3306, "db", "user", "pass", timeout=5
            )
            assert rows == mock_rows
