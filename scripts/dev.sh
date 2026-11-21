#!/bin/sh
# Lance toute la stack fX v2 en local (sans Docker) dans un seul terminal :
#   1. typst-renderd (Unix socket)
#   2. fx-server (Go API + WebSocket)
#   3. ng-web (Angular 21 dev server sur http://localhost:4200/studio)
# Un seul Ctrl+C arrête proprement les 3 processus.
set -e

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

# Charger .env s'il existe
if [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
fi

SOCKET="${RENDER_DAEMON_SOCKET:-/tmp/typst-renderd.sock}"
DAEMON_PID=""
SERVER_PID=""
WEB_PID=""

cleanup() {
    trap - INT TERM EXIT
    echo ""
    echo "[dev] Arrêt propre de toute la stack fX v2..."
    for pid in "$WEB_PID" "$SERVER_PID" "$DAEMON_PID"; do
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill -TERM "$pid" 2>/dev/null || true
        fi
    done
    wait 2>/dev/null || true
    rm -f "$SOCKET"
    echo "[dev] Arrêt terminé."
    exit 0
}
trap cleanup INT TERM EXIT

# 1. Démarrer typst-renderd si présent (sinon build automatique)
if [ ! -x ./bin/typst-renderd ]; then
    echo "[dev] bin/typst-renderd absent -> build Rust en cours..."
    make build-rust
fi

rm -f "$SOCKET"
echo "[dev] 1/3 Démarrage de typst-renderd ($SOCKET)..."
./bin/typst-renderd --socket "$SOCKET" &
DAEMON_PID=$!

WAIT_TICKS=0
while [ "$WAIT_TICKS" -lt 50 ]; do
    [ -S "$SOCKET" ] && break
    sleep 0.1
    WAIT_TICKS=$((WAIT_TICKS + 1))
done

# 2. Démarrer fx-server Go branché sur le daemon Rust
echo "[dev] 2/3 Démarrage de fx-server (http://localhost:${PORT:-8080})..."
RENDER_BACKEND=rust \
RENDER_DAEMON_SOCKET="$SOCKET" \
TEMPLATES_PATH="${TEMPLATES_PATH:-./templates-custom}" \
STUDIO_ENABLED=true \
ALLOW_CUSTOM_TEMPLATES=true \
go run ./cmd/server &
SERVER_PID=$!

# 3. Démarrer Angular 21 dev server
echo "[dev] 3/3 Démarrage de Studio Builder v2 (http://localhost:4200/studio)..."
(cd ng_web && npm start) &
WEB_PID=$!

echo ""
echo "================================================================"
echo "  fX v2 prêt !"
echo "  • Studio Builder v2 : http://localhost:4200/studio"
echo "  • API Backend       : http://localhost:${PORT:-8080}/api/v1"
echo "  • Rust Turbo Daemon : $SOCKET"
echo "  Appuie sur Ctrl+C pour tout arrêter proprement."
echo "================================================================"

wait
