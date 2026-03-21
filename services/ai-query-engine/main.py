"""AI Query Engine - FastAPI application entry point."""
import logging
import os
from pathlib import Path

import uvicorn
import yaml
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import Response

from routers.connections import router as connections_router
from routers.query import router as query_router
from routers.trained_queries import router as trained_queries_router

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="SaaS FinOps Analytics Platform API",
    version="1.0.0",
    description=(
        "Natural language to SQL query engine and FinOps analytics platform.\n\n"
        "## Authentication\n\n"
        "This API supports two authentication methods:\n\n"
        "- **JWT Bearer Token** – Obtain via `POST /auth/login`. "
        "Include as `Authorization: Bearer <token>`. Expires in 15 minutes.\n"
        "- **API Key** – Generate via `POST /auth/api-keys`. "
        "Include as `X-API-Key: <key>` header.\n\n"
        "## Interactive Documentation\n\n"
        "- Swagger UI: [/docs](/docs)\n"
        "- ReDoc: [/redoc](/redoc)\n"
        "- OpenAPI spec: [/openapi.yaml](/openapi.yaml)"
    ),
    docs_url="/docs",
    redoc_url="/redoc",
)

# CORS middleware - allow all origins for development; restrict in production
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Register routers
app.include_router(connections_router)
app.include_router(query_router)
app.include_router(trained_queries_router)


@app.get("/health", tags=["health"])
async def health_check():
    """Health check endpoint for container orchestration. Requirements: 18.5"""
    return {
        "status": "healthy",
        "service": "ai-query-engine",
    }


@app.get("/openapi.yaml", include_in_schema=False)
async def serve_openapi_yaml():
    """Serve the OpenAPI 3.0 specification as YAML. Requirements: 16.1, 16.7"""
    # Look for openapi.yaml relative to this file or in the docs/ directory
    candidates = [
        Path(__file__).parent.parent.parent / "docs" / "openapi.yaml",
        Path(__file__).parent / "docs" / "openapi.yaml",
        Path("docs") / "openapi.yaml",
    ]
    for candidate in candidates:
        if candidate.exists():
            content = candidate.read_text(encoding="utf-8")
            return Response(content=content, media_type="application/yaml")

    # Fallback: generate from FastAPI's built-in schema and return as YAML
    from fastapi.openapi.utils import get_openapi

    schema = get_openapi(
        title=app.title,
        version=app.version,
        description=app.description,
        routes=app.routes,
    )
    return Response(content=yaml.dump(schema, allow_unicode=True), media_type="application/yaml")


if __name__ == "__main__":
    port = int(os.getenv("PORT", "8084"))
    uvicorn.run(app, host="0.0.0.0", port=port)
