#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST="$ROOT/dist"
ERRORS=0

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
ok()   { echo -e "${GREEN}[OK]${NC}   $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; ERRORS=$((ERRORS+1)); }

echo "============================================================"
echo "  SaaS FinOps Analytics Platform - Build All Binaries"
echo "============================================================"
echo

mkdir -p "$DIST"
echo "[INFO] Output directory: $DIST"
echo

# ── Check Go ─────────────────────────────────────────────────
command -v go >/dev/null 2>&1 || { echo "[ERROR] Go not found."; exit 1; }
echo "[INFO] Using: $(go version)"
echo

# ── Detect OS for binary extension ───────────────────────────
EXT=""
if [[ "${OSTYPE:-}" == "msys"* ]] || [[ "${OSTYPE:-}" == "cygwin"* ]]; then
    EXT=".exe"
fi

# ── Sync workspace ───────────────────────────────────────────
echo "[BUILD] Syncing Go workspace..."
cd "$ROOT"
go work sync || warn "go work sync had issues - continuing."
echo

# ── Build migrations tool ────────────────────────────────────
echo "[BUILD] migrations tool..."
(cd "$ROOT/migrations" && go build -o "$DIST/migrate${EXT}" .) \
    && ok "dist/migrate${EXT}" \
    || fail "migrations"

# ── Build Go services ────────────────────────────────────────
for svc in api-gateway auth-service billing-service finops-service; do
    echo "[BUILD] $svc..."
    (cd "$ROOT/services/$svc" && go build -ldflags="-s -w" -o "$DIST/$svc${EXT}" .) \
        && ok "dist/$svc${EXT}" \
        || fail "$svc"
done

# ── Build frontend ───────────────────────────────────────────
echo
if command -v node >/dev/null 2>&1; then
    echo "[BUILD] frontend..."
    cd "$ROOT/services/frontend"
    [ -d "node_modules" ] || npm install --silent
    npm run build
    mkdir -p "$DIST/frontend"
    cp -r dist/. "$DIST/frontend/"
    ok "dist/frontend/ (static assets)"
    cd "$ROOT"
else
    warn "Node.js not found - skipping frontend build."
fi

# ── Copy config + migrations SQL ─────────────────────────────
echo
echo "[COPY] Bundling config and migrations..."
cp "$ROOT/config.ini" "$DIST/config.ini"
mkdir -p "$DIST/migrations"
cp "$ROOT/migrations/"*.sql "$DIST/migrations/"
ok "config.ini and migrations SQL copied."

# ── Generate run-dist.sh ──────────────────────────────────────
echo
echo "[GEN]  Generating dist/run-dist.sh..."
cat > "$DIST/run-dist.sh" << 'RUNSCRIPT'
#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PIDS=()

echo "Running migrations..."
DB_HOST="${DB_HOST:-localhost}" \
DB_PORT="${DB_PORT:-3306}" \
DB_USERNAME="${DB_USERNAME:-root}" \
DB_PASSWORD="${DB_PASSWORD:-rootpassword}" \
DB_NAME="${DB_NAME:-finops_platform}" \
"$ROOT/migrate"

echo "Starting services..."
for svc in api-gateway auth-service billing-service finops-service; do
    [ -f "$ROOT/$svc" ] || continue
    CONFIG_PATH="$ROOT/config.ini" "$ROOT/$svc" >> "$ROOT/${svc}.log" 2>&1 &
    PIDS+=($!)
    echo "  [START] $svc (PID ${PIDS[-1]})"
    sleep 1
done

echo
echo "All services running. API Gateway: http://localhost:8080/health"
echo "Press Ctrl+C to stop."
trap 'kill "${PIDS[@]}" 2>/dev/null' INT TERM
wait
RUNSCRIPT
chmod +x "$DIST/run-dist.sh"
ok "dist/run-dist.sh"

# ── Summary ──────────────────────────────────────────────────
echo
echo "============================================================"
if [ "$ERRORS" -eq 0 ]; then
    echo "  Build SUCCESSFUL - all binaries in dist/"
else
    echo "  Build completed with $ERRORS error(s) - check output above."
fi
echo "============================================================"
echo
ls -lh "$DIST"
echo
