.PHONY: help build run stop migrate test clean

help:
	@echo ""
	@echo "  SaaS FinOps Analytics Platform"
	@echo ""
	@echo "  One-click:"
	@echo "    make run          - Setup env + launch all services (Linux/Mac)"
	@echo "    make build        - Compile all Go services to dist/ (Linux/Mac)"
	@echo "    make stop         - Stop all running services"
	@echo ""
	@echo "  Individual services:"
	@echo "    make run-api-gateway"
	@echo "    make run-auth"
	@echo "    make run-billing"
	@echo "    make run-finops"
	@echo "    make run-ai"
	@echo "    make run-frontend"
	@echo ""
	@echo "  Other:"
	@echo "    make migrate      - Run database migrations only"
	@echo "    make test         - Run all tests"
	@echo "    make clean        - Remove build artifacts"
	@echo ""

# ── One-click targets ─────────────────────────────────────────
run:
	@chmod +x run.sh && ./run.sh

build:
	@chmod +x build.sh && ./build.sh

stop:
	@chmod +x stop.sh && ./stop.sh

# ── Database ──────────────────────────────────────────────────
migrate:
	cd migrations && go run migrate.go

# ── Individual services ───────────────────────────────────────
run-api-gateway:
	cd services/api-gateway && go run main.go

run-auth:
	cd services/auth-service && go run main.go

run-billing:
	cd services/billing-service && go run main.go

run-finops:
	cd services/finops-service && go run main.go

run-ai:
	cd services/ai-query-engine && \
	  ([ -f venv/bin/activate ] && . venv/bin/activate || . venv/Scripts/activate) && \
	  python main.py

run-frontend:
	cd services/frontend && npm run dev

# ── Tests ─────────────────────────────────────────────────────
test:
	go test ./...
	cd services/ai-query-engine && \
	  ([ -f venv/bin/activate ] && . venv/bin/activate || . venv/Scripts/activate) && \
	  python -m pytest --tb=short

# ── Clean ─────────────────────────────────────────────────────
clean:
	rm -rf dist/ logs/ .service_pids
	go clean -cache
	find . -name "*.pyc" -delete
	find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null || true
