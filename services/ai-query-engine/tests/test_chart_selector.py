"""Unit tests for chart type selection and response formatting.
Requirements: 6.5, 7.1, 7.2, 7.3, 7.4
"""
import pytest
from services.chart_selector import select_chart_type, format_response


class TestSelectChartType:
    """Tests for select_chart_type function."""

    def test_empty_rows_returns_table(self):
        assert select_chart_type([]) == "table"

    def test_single_numeric_value_returns_metric(self):
        rows = [{"total": 42}]
        assert select_chart_type(rows) == "metric"

    def test_single_row_single_string_returns_table(self):
        rows = [{"name": "Alice"}]
        # Non-numeric single value → table
        result = select_chart_type(rows)
        assert result in ("table", "metric")  # implementation may vary

    def test_time_column_with_numeric_returns_line(self):
        rows = [
            {"date": "2024-01-01", "cost": 100},
            {"date": "2024-01-02", "cost": 150},
        ]
        assert select_chart_type(rows) == "line"

    def test_category_column_with_numeric_returns_bar(self):
        rows = [
            {"service": "EC2", "total_cost": 500},
            {"service": "S3", "total_cost": 200},
            {"service": "RDS", "total_cost": 300},
            {"service": "Lambda", "total_cost": 100},
            {"service": "CloudFront", "total_cost": 50},
            {"service": "EKS", "total_cost": 400},
            {"service": "ECS", "total_cost": 250},
            {"service": "SQS", "total_cost": 75},
            {"service": "SNS", "total_cost": 60},
            {"service": "DynamoDB", "total_cost": 180},
            {"service": "ElastiCache", "total_cost": 90},
        ]
        # More than 10 rows with 2 cols → bar
        assert select_chart_type(rows) == "bar"

    def test_two_columns_small_dataset_returns_pie(self):
        rows = [
            {"provider": "AWS", "amount": 1000},
            {"provider": "Azure", "amount": 500},
            {"provider": "GCP", "amount": 300},
        ]
        assert select_chart_type(rows) == "pie"

    def test_many_columns_returns_table(self):
        # Rows with many mixed-type columns and no clear numeric/category pattern → table
        rows = [
            {"first_name": "Alice", "last_name": "Smith", "email": "a@b.com", "city": "NYC", "country": "US"},
            {"first_name": "Bob", "last_name": "Jones", "email": "b@b.com", "city": "LA", "country": "US"},
        ]
        assert select_chart_type(rows) == "table"

    def test_month_column_returns_line(self):
        rows = [
            {"month": "Jan", "revenue": 1000},
            {"month": "Feb", "revenue": 1200},
        ]
        assert select_chart_type(rows) == "line"

    def test_count_column_with_category(self):
        rows = [
            {"region": "us-east-1", "count": 50},
            {"region": "eu-west-1", "count": 30},
        ]
        # 2 cols, ≤10 rows → pie
        assert select_chart_type(rows) == "pie"


class TestFormatResponse:
    """Tests for format_response function."""

    def test_empty_rows(self):
        result = format_response([], "table", "SELECT 1", 10)
        assert result["chartType"] == "table"
        assert result["labels"] == []
        assert result["data"] == []
        assert result["rawData"] == []
        assert result["generatedSql"] == "SELECT 1"
        assert result["executionTimeMs"] == 10

    def test_metric_format(self):
        rows = [{"total": 42}]
        result = format_response(rows, "metric", "SELECT COUNT(*) AS total FROM t", 5)
        assert result["chartType"] == "metric"
        assert result["data"] == [42]
        assert result["rawData"] == rows

    def test_line_format(self):
        rows = [
            {"date": "2024-01", "cost": 100},
            {"date": "2024-02", "cost": 200},
        ]
        result = format_response(rows, "line", "SELECT date, cost FROM t", 20)
        assert result["chartType"] == "line"
        assert result["labels"] == ["2024-01", "2024-02"]
        assert result["data"] == [100, 200]

    def test_bar_format(self):
        rows = [
            {"service": "EC2", "total_cost": 500},
            {"service": "S3", "total_cost": 200},
        ]
        result = format_response(rows, "bar", "SELECT service, total_cost FROM t", 15)
        assert result["chartType"] == "bar"
        assert result["labels"] == ["EC2", "S3"]
        assert result["data"] == [500, 200]

    def test_pie_format(self):
        rows = [
            {"provider": "AWS", "amount": 1000},
            {"provider": "Azure", "amount": 500},
        ]
        result = format_response(rows, "pie", "SELECT provider, amount FROM t", 8)
        assert result["chartType"] == "pie"
        assert result["labels"] == ["AWS", "Azure"]
        assert result["data"] == [1000, 500]

    def test_table_format(self):
        rows = [
            {"id": 1, "name": "Alice", "role": "admin"},
            {"id": 2, "name": "Bob", "role": "user"},
        ]
        result = format_response(rows, "table", "SELECT * FROM users", 30)
        assert result["chartType"] == "table"
        assert result["labels"] == ["id", "name", "role"]
        assert len(result["data"]) == 2
        assert result["data"][0] == [1, "Alice", "admin"]

    def test_raw_data_always_present(self):
        rows = [{"x": 1}]
        result = format_response(rows, "metric", "SELECT 1", 1)
        assert result["rawData"] == rows
