#!/usr/bin/env bash
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "============================================================"
echo "  SaaS FinOps Analytics Platform - Stopping Services"
echo "============================================================"
echo

# Kill by saved PIDs
if [ -f "$ROOT/.service_pids" ]; then
    while IFS= read -r pid; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null && echo "  [STOP] PID $pid"
        fi
    done < "$ROOT/.service_pids"
    rm -f "$ROOT/.service_pids"
fi

# Kill by port as fallback
for port in 8080 8081 8082 8083 8084 5173; do
    pid=$(lsof -ti ":$port" 2>/dev/null || true)
    if [ -n "$pid" ]; then
        kill "$pid" 2>/dev/null && echo "  [FREE] port $port (PID $pid)"
    fi
done

echo
echo "  All services stopped."
echo "============================================================"
