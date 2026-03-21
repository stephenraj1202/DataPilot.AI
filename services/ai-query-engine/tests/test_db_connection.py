"""Unit tests for database connection testing (_test_connection).
Requirements: 8.4
"""
import sys
import types
import pytest
from unittest.mock import patch, MagicMock


# ---------------------------------------------------------------------------
# Stub out optional DB driver modules so tests run without them installed
# ---------------------------------------------------------------------------

def _stub_psycopg2():
    """Create a minimal psycopg2 stub in sys.modules."""
    mod = types.ModuleType("psycopg2")

    class OperationalError(Exception):
        pass

    mod.OperationalError = OperationalError
    mod.connect = MagicMock()
    sys.modules.setdefault("psycopg2", mod)
    return mod


def _stub_pymongo():
    """Create minimal pymongo stubs in sys.modules."""
    mod = types.ModuleType("pymongo")
    errors_mod = types.ModuleType("pymongo.errors")

    class ServerSelectionTimeoutError(Exception):
        pass

    class OperationFailure(Exception):
        pass

    errors_mod.ServerSelectionTimeoutError = ServerSelectionTimeoutError
    errors_mod.OperationFailure = OperationFailure
    mod.errors = errors_mod
    mod.MongoClient = MagicMock()
    sys.modules.setdefault("pymongo", mod)
    sys.modules.setdefault("pymongo.errors", errors_mod)
    return mod, errors_mod


def _stub_pyodbc():
    """Create a minimal pyodbc stub in sys.modules."""
    mod = types.ModuleType("pyodbc")

    class OperationalError(Exception):
        pass

    class InterfaceError(Exception):
        pass

    mod.OperationalError = OperationalError
    mod.InterfaceError = InterfaceError
    mod.connect = MagicMock()
    sys.modules.setdefault("pyodbc", mod)
    return mod


# Install stubs before importing the module under test
_psycopg2_stub = _stub_psycopg2()
_pymongo_stub, _pymongo_errors_stub = _stub_pymongo()
_pyodbc_stub = _stub_pyodbc()

from routers.connections import _test_connection  # noqa: E402


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_conn_mock():
    conn = MagicMock()
    conn.close = MagicMock()
    return conn


# ---------------------------------------------------------------------------
# PostgreSQL
# ---------------------------------------------------------------------------

class TestPostgresConnection:
    """Tests for PostgreSQL connection validation."""

    def test_successful_connection(self):
        conn = _make_conn_mock()
        with patch.object(_psycopg2_stub, "connect", return_value=conn) as mock_connect:
            _test_connection("postgresql", "localhost", 5432, "mydb", "user", "pass")
            mock_connect.assert_called_once()
            conn.close.assert_called_once()

    def test_connection_failure_raises_connection_error(self):
        with patch.object(_psycopg2_stub, "connect",
                          side_effect=_psycopg2_stub.OperationalError("Connection refused")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("postgresql", "badhost", 5432, "mydb", "user", "pass")
            assert "postgresql" in str(exc_info.value).lower()
            assert "badhost" in str(exc_info.value)

    def test_invalid_credentials_returns_descriptive_error(self):
        with patch.object(_psycopg2_stub, "connect",
                          side_effect=_psycopg2_stub.OperationalError("password authentication failed")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("postgresql", "localhost", 5432, "mydb", "user", "wrongpass")
            assert "password authentication failed" in str(exc_info.value)

    def test_invalid_host_returns_descriptive_error(self):
        with patch.object(_psycopg2_stub, "connect",
                          side_effect=_psycopg2_stub.OperationalError("could not translate host name")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("postgresql", "nonexistent.host", 5432, "mydb", "user", "pass")
            assert "could not translate host name" in str(exc_info.value)

    def test_connection_timeout_returns_descriptive_error(self):
        with patch.object(_psycopg2_stub, "connect",
                          side_effect=_psycopg2_stub.OperationalError("timeout expired")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("postgresql", "10.0.0.1", 5432, "mydb", "user", "pass")
            assert "timeout expired" in str(exc_info.value)

    def test_error_message_includes_host_and_port(self):
        with patch.object(_psycopg2_stub, "connect",
                          side_effect=_psycopg2_stub.OperationalError("refused")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("postgresql", "myhost", 9999, "mydb", "user", "pass")
            msg = str(exc_info.value)
            assert "myhost" in msg
            assert "9999" in msg


# ---------------------------------------------------------------------------
# MySQL
# ---------------------------------------------------------------------------

class TestMySQLConnection:
    """Tests for MySQL connection validation."""

    def test_successful_connection(self):
        import pymysql
        conn = _make_conn_mock()
        with patch("pymysql.connect", return_value=conn) as mock_connect:
            _test_connection("mysql", "localhost", 3306, "mydb", "user", "pass")
            mock_connect.assert_called_once()
            conn.close.assert_called_once()

    def test_connection_failure_raises_connection_error(self):
        import pymysql
        with patch("pymysql.connect",
                   side_effect=pymysql.OperationalError(2003, "Can't connect to MySQL server")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mysql", "badhost", 3306, "mydb", "user", "pass")
            assert "mysql" in str(exc_info.value).lower()
            assert "badhost" in str(exc_info.value)

    def test_invalid_credentials_returns_descriptive_error(self):
        import pymysql
        with patch("pymysql.connect",
                   side_effect=pymysql.OperationalError(1045, "Access denied for user")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mysql", "localhost", 3306, "mydb", "user", "wrongpass")
            assert "Access denied" in str(exc_info.value)

    def test_invalid_host_returns_descriptive_error(self):
        import pymysql
        with patch("pymysql.connect",
                   side_effect=pymysql.OperationalError(2005, "Unknown MySQL server host")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mysql", "nonexistent.host", 3306, "mydb", "user", "pass")
            assert "Unknown MySQL server host" in str(exc_info.value)

    def test_connection_timeout_returns_descriptive_error(self):
        import pymysql
        with patch("pymysql.connect",
                   side_effect=pymysql.OperationalError(2013, "Lost connection to MySQL server")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mysql", "10.0.0.1", 3306, "mydb", "user", "pass")
            assert "Lost connection" in str(exc_info.value)

    def test_error_message_includes_host_and_port(self):
        import pymysql
        with patch("pymysql.connect",
                   side_effect=pymysql.OperationalError(2003, "refused")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mysql", "myhost", 3307, "mydb", "user", "pass")
            msg = str(exc_info.value)
            assert "myhost" in msg
            assert "3307" in msg


# ---------------------------------------------------------------------------
# MongoDB
# ---------------------------------------------------------------------------

class TestMongoDBConnection:
    """Tests for MongoDB connection validation."""

    def test_successful_connection(self):
        mock_client = MagicMock()
        mock_client.server_info.return_value = {"version": "6.0"}
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            _test_connection("mongodb", "localhost", 27017, "mydb", "user", "pass")
            mock_client.server_info.assert_called_once()
            mock_client.close.assert_called_once()

    def test_successful_connection_without_credentials(self):
        mock_client = MagicMock()
        mock_client.server_info.return_value = {"version": "6.0"}
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            _test_connection("mongodb", "localhost", 27017, "mydb", "", "")
            mock_client.server_info.assert_called_once()

    def test_connection_failure_raises_connection_error(self):
        ServerSelectionTimeoutError = _pymongo_errors_stub.ServerSelectionTimeoutError
        mock_client = MagicMock()
        mock_client.server_info.side_effect = ServerSelectionTimeoutError("No servers found")
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mongodb", "badhost", 27017, "mydb", "user", "pass")
            assert "mongodb" in str(exc_info.value).lower()
            assert "badhost" in str(exc_info.value)

    def test_invalid_credentials_returns_descriptive_error(self):
        OperationFailure = _pymongo_errors_stub.OperationFailure
        mock_client = MagicMock()
        mock_client.server_info.side_effect = OperationFailure("Authentication failed")
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mongodb", "localhost", 27017, "mydb", "user", "wrongpass")
            assert "Authentication failed" in str(exc_info.value)

    def test_connection_timeout_returns_descriptive_error(self):
        ServerSelectionTimeoutError = _pymongo_errors_stub.ServerSelectionTimeoutError
        mock_client = MagicMock()
        mock_client.server_info.side_effect = ServerSelectionTimeoutError("timed out")
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mongodb", "10.0.0.1", 27017, "mydb", "user", "pass")
            assert "timed out" in str(exc_info.value)

    def test_error_message_includes_host_and_port(self):
        ServerSelectionTimeoutError = _pymongo_errors_stub.ServerSelectionTimeoutError
        mock_client = MagicMock()
        mock_client.server_info.side_effect = ServerSelectionTimeoutError("refused")
        with patch.object(_pymongo_stub, "MongoClient", return_value=mock_client):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("mongodb", "myhost", 27018, "mydb", "user", "pass")
            msg = str(exc_info.value)
            assert "myhost" in msg
            assert "27018" in msg


# ---------------------------------------------------------------------------
# SQL Server
# ---------------------------------------------------------------------------

class TestSQLServerConnection:
    """Tests for SQL Server connection validation."""

    def test_successful_connection(self):
        conn = _make_conn_mock()
        with patch.object(_pyodbc_stub, "connect", return_value=conn) as mock_connect:
            _test_connection("sqlserver", "localhost", 1433, "mydb", "user", "pass")
            mock_connect.assert_called_once()
            conn.close.assert_called_once()

    def test_connection_failure_raises_connection_error(self):
        with patch.object(_pyodbc_stub, "connect",
                          side_effect=_pyodbc_stub.OperationalError("Login timeout expired")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("sqlserver", "badhost", 1433, "mydb", "user", "pass")
            assert "sqlserver" in str(exc_info.value).lower()
            assert "badhost" in str(exc_info.value)

    def test_invalid_credentials_returns_descriptive_error(self):
        with patch.object(_pyodbc_stub, "connect",
                          side_effect=_pyodbc_stub.InterfaceError("Login failed for user")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("sqlserver", "localhost", 1433, "mydb", "user", "wrongpass")
            assert "Login failed" in str(exc_info.value)

    def test_invalid_host_returns_descriptive_error(self):
        with patch.object(_pyodbc_stub, "connect",
                          side_effect=_pyodbc_stub.OperationalError("Named Pipes Provider: Could not open")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("sqlserver", "nonexistent.host", 1433, "mydb", "user", "pass")
            assert "Named Pipes Provider" in str(exc_info.value)

    def test_connection_timeout_returns_descriptive_error(self):
        with patch.object(_pyodbc_stub, "connect",
                          side_effect=_pyodbc_stub.OperationalError("Login timeout expired")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("sqlserver", "10.0.0.1", 1433, "mydb", "user", "pass")
            assert "Login timeout expired" in str(exc_info.value)

    def test_error_message_includes_host_and_port(self):
        with patch.object(_pyodbc_stub, "connect",
                          side_effect=_pyodbc_stub.OperationalError("refused")):
            with pytest.raises(ConnectionError) as exc_info:
                _test_connection("sqlserver", "myhost", 1434, "mydb", "user", "pass")
            msg = str(exc_info.value)
            assert "myhost" in msg
            assert "1434" in msg


# ---------------------------------------------------------------------------
# Unsupported DB type
# ---------------------------------------------------------------------------

class TestUnsupportedDBType:
    """Tests for unsupported database type handling."""

    def test_unsupported_type_raises_value_error(self):
        with pytest.raises(ValueError, match="Unsupported database type"):
            _test_connection("oracle", "localhost", 1521, "mydb", "user", "pass")

    def test_unsupported_type_does_not_raise_connection_error(self):
        with pytest.raises(ValueError):
            _test_connection("redis", "localhost", 6379, "mydb", "user", "pass")
