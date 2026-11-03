#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
PORT="${PORT:-8080}"
export PORT

if [ "${1:-}" = "--demo" ]; then
  export SWARM_DEMO=1
  echo "Demo mode: the server will add 10 sample torrents to an empty database."
fi

command -v go >/dev/null || { echo "Go is not installed. Install it from https://go.dev/dl/" >&2; exit 1; }
command -v npm >/dev/null || { echo "npm is not installed. Install Node.js from https://nodejs.org/" >&2; exit 1; }

if [ ! -d "$ROOT/dashboard/node_modules" ]; then
  echo "Installing dashboard dependencies..."
  (cd "$ROOT/dashboard" && npm install)
fi

pids=()
cleanup() {
  trap - INT TERM EXIT
  for pid in "${pids[@]:-}"; do
    [ -n "$pid" ] && kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup INT TERM EXIT

echo "Building server..."
(cd "$ROOT/server" && go build -o bin/server ./cmd/server)

echo "Starting server on :$PORT"
(cd "$ROOT/server" && exec ./bin/server) &
pids+=($!)

echo "Starting dashboard on http://localhost:5173"
(cd "$ROOT/dashboard" && exec npm run dev -- --host 127.0.0.1) &
pids+=($!)

while true; do
  for pid in "${pids[@]}"; do
    kill -0 "$pid" 2>/dev/null || { echo "A process exited (pid $pid), shutting down." >&2; exit 1; }
  done
  sleep 1
done
