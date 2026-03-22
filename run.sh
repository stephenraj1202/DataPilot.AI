#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKIP_FRONTEND=0
PIDS=()

# ── Colours ──────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; NC='\033[0m'
info()  { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

echo "============================================================"
echo "  SaaS FinOps Analytics Platform - Linux/Mac One-Click Run"
echo "============================================================"
echo

# ── 1. Prerequisites ─────────────────────────────────────────
echo "[1/6] Checking prerequisites..."

command -v go  >/dev/null 2>&1 || error "Go not found. Install from https://go.dev/dl/"
echo "       $(go version)"

PYTHON=""
for p in python3 python; do
    if command -v "$p" >/dev/null 2>&1; then PYTHON="$p"; break; fi
done
[ -z "$PYTHON" ] && error "Python not found. Install Python 3.11+ from https://www.python.org/"
echo "       $($PYTHON --version)"

command -v mysql >/dev/null 2>&1 \
    && echo "       MySQL CLI found." \
    || warn "mysql CLI not in PATH. Ensure MySQL 8.0 is running on port 3306."

if command -v node >/dev/null 2>&1; then
    echo "       Node $(node --version)"
else
    warn "Node.js not found - frontend will be skipped."
    SKIP_FRONTEND=1
fi
info "Prerequisites checked."
echo

# ── 2. Database setup + migrations ───────────────────────────
echo "[2/6] Setting up database and running migrations..."
"$PYTHON" "$ROOT/_dbsetup.py" || error "Database setup failed. Check config.ini credentials."
info "Database ready."
echo

# ── 3. Go workspace + deps ───────────────────────────────────
echo "[3/6] Syncing Go workspace and downloading dependencies..."
cd "$ROOT"
go work sync || warn "go work sync had issues - continuing."

for svc in api-gateway auth-service billing-service finops-service; do
    [ -f "$ROOT/services/$svc/go.mod" ] || continue
    echo "       [$svc] go mod download..."
    (cd "$ROOT/services/$svc" && go mod download) &
done
for mod in config database redis; do
    [ -f "$ROOT/shared/$mod/go.mod" ] || continue
    (cd "$ROOT/shared/$mod" && go mod download) &
done
wait
info "Go dependencies ready."
echo

# ── 4. Python venv ───────────────────────────────────────────
echo "[4/6] Setting up Python virtual environment..."
cd "$ROOT/services/ai-query-engine"

if [ ! -f "venv/bin/activate" ] && [ ! -f "venv/Scripts/activate" ]; then
    echo "       Creating venv..."
    "$PYTHON" -m venv venv
fi

# Activate (works on Linux/Mac and Git Bash/WSL)
if [ -f "venv/bin/activate" ]; then
    # shellcheck disable=SC1091
    source venv/bin/activate
else
    # shellcheck disable=SC1091
    source venv/Scripts/activate
fi

pip install --upgrade pip -q
pip install -r requirements.txt -q
deactivate

if [ ! -f ".env" ]; then
    [ -f ".env.example" ] && cp .env.example .env
    warn "Edit services/ai-query-engine/.env and set GEMINI_API_KEY."
fi
cd "$ROOT"
info "Python environment ready."
echo

# ── 5. Frontend npm install ───────────────────────────────────
echo "[5/6] Installing frontend dependencies..."
if [ "$SKIP_FRONTEND" -eq 0 ]; then
    cd "$ROOT/services/frontend"
    if [ ! -d "node_modules" ]; then
        npm install --silent
    else
        echo "       node_modules already present, skipping."
    fi
    cd "$ROOT"
    info "Frontend ready."
else
    echo "[SKIP] Node.js not found - skipping frontend."
fi
echo

# ── 6. Launch services ────────────────────────────────────────
echo "[6/6] Launching services..."
echo

LOG_DIR="$ROOT/logs"
mkdir -p "$LOG_DIR"

launch() {
    local name="$1" dir="$2" cmd="$3"
    echo "       [START] $name"
    (cd "$dir" && eval "$cmd" >> "$LOG_DIR/${name// /_}.log" 2>&1) &
    PIDS+=($!)
}

launch "api-gateway"     "$ROOT/services/api-gateway"     "go run main.go"
sleep 1
launch "auth-service"    "$ROOT/services/auth-service"    "go run main.go"
sleep 1
launch "billing-service" "$ROOT/services/billing-service" "go run main.go"
sleep 1
launch "finops-service"  "$ROOT/services/finops-service"  "go run main.go"
sleep 1

# Activate venv for AI engine
if [ -f "$ROOT/services/ai-query-engine/venv/bin/activate" ]; then
    launch "ai-query-engine" "$ROOT/services/ai-query-engine" \
        "source venv/bin/activate && python main.py"
else
    launch "ai-query-engine" "$ROOT/services/ai-query-engine" \
        "source venv/Scripts/activate && python main.py"
fi

if [ "$SKIP_FRONTEND" -eq 0 ]; then
    sleep 1
    launch "frontend" "$ROOT/services/frontend" "npm run dev"
fi

# Save PIDs for stop.sh
printf '%s\n' "${PIDS[@]}" > "$ROOT/.service_pids"

echo
echo "============================================================"
echo "  All services launched. Logs in: logs/"
echo
echo "  API Gateway  : http://localhost:8080/health"
echo "  Auth Service : http://localhost:8081/health"
echo "  Billing      : http://localhost:8082/health"
echo "  FinOps       : http://localhost:8083/health"
echo "  AI Engine    : http://localhost:8084/health"
[ "$SKIP_FRONTEND" -eq 0 ] && echo "  Frontend     : http://localhost:5173"
echo
echo "  Run ./stop.sh to shut everything down."
echo "  Tail logs:  tail -f logs/<service>.log"
echo "============================================================"
echo

# Wait and handle Ctrl+C
trap 'echo; echo "Stopping all services..."; kill "${PIDS[@]}" 2>/dev/null; exit 0' INT TERM
wait
