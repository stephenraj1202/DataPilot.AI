"""Pytest configuration - ensure the ai-query-engine root is on sys.path."""
import sys
import os

# Add the service root to sys.path so imports work correctly
sys.path.insert(0, os.path.dirname(__file__))
