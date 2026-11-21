#!/bin/sh
set -e

DAEMON_BIN="/usr/local/bin/typst-renderd"
BACKEND="${RENDER_BACKEND:-auto}"
SOCKET="${RENDER_DAEMON_SOCKET:-/tmp/typst-renderd.sock}"
DAEMON_PID=""
SERVER_PID=""

# Ensure explicit temporary render directory exists when configured and writable.
if [ -n "${RENDER_TMP_DIR:-}" ]; then
    mkdir -p "$RENDER_TMP_DIR" 2>/dev/null || true
fi

is_truthy() {
    case "${1:-}" in
        1|true|TRUE|True|yes|YES|Yes|on|ON) return 0 ;;
        *) return 1 ;;
    esac
}

should_start_daemon() {
    case "$BACKEND" in
        cli|CLI) return 1 ;;
    esac
    [ -x "$DAEMON_BIN" ] && [ -s "$DAEMON_BIN" ]
}

start_daemon() {
    mkdir -p "$(dirname "$SOCKET")"
    rm -f "$SOCKET"

    set -- --socket "$SOCKET"

    FONT_PATHS="${RENDER_FONT_PATHS:-${TYPST_FONT_PATHS:-}}"
    if [ -n "$FONT_PATHS" ]; then
        set -- "$@" --font-path "$FONT_PATHS"
    fi

    if is_truthy "${TYPST_IGNORE_SYSTEM_FONTS:-false}"; then
        set -- "$@" --ignore-system-fonts
    fi

    if [ -n "${RENDER_WORKERS:-}" ] && [ "$RENDER_WORKERS" -gt 0 ] 2>/dev/null; then
        set -- "$@" --workers "$RENDER_WORKERS"
    fi

    echo "[entrypoint] Starting typst-renderd (socket=$SOCKET)..."
    "$DAEMON_BIN" "$@" &
    DAEMON_PID=$!
}

shutdown() {
    trap - TERM INT
    echo "[entrypoint] Termination signal received, shutting down fx-server and typst-renderd..."
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -TERM "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
    fi
    if [ -n "$DAEMON_PID" ] && kill -0 "$DAEMON_PID" 2>/dev/null; then
        kill -TERM "$DAEMON_PID" 2>/dev/null || true
        wait "$DAEMON_PID" 2>/dev/null || true
    fi
    exit 0
}

if should_start_daemon; then
    export RENDER_DAEMON_SOCKET="$SOCKET"
    trap shutdown TERM INT

    start_daemon

    # Wait up to 5 seconds (50 x 0.1s) for the Unix domain socket to appear.
    WAIT_TICKS=0
    SOCKET_READY=0
    while [ "$WAIT_TICKS" -lt 50 ]; do
        if [ -S "$SOCKET" ]; then
            SOCKET_READY=1
            break
        fi
        if ! kill -0 "$DAEMON_PID" 2>/dev/null; then
            echo "[entrypoint] Warning: typst-renderd exited prematurely during startup" >&2
            break
        fi
        sleep 0.1
        WAIT_TICKS=$((WAIT_TICKS + 1))
    done

    if [ "$SOCKET_READY" -eq 1 ]; then
        echo "[entrypoint] typst-renderd ready on $SOCKET (pid=$DAEMON_PID)"
    else
        echo "[entrypoint] Warning: typst-renderd socket $SOCKET not ready after 5s; continuing with fx-server (backend=$BACKEND)" >&2
    fi

    fx-server "$@" &
    SERVER_PID=$!

    set +e
    wait "$SERVER_PID"
    EXIT_CODE=$?
    set -e

    if [ -n "$DAEMON_PID" ] && kill -0 "$DAEMON_PID" 2>/dev/null; then
        kill -TERM "$DAEMON_PID" 2>/dev/null || true
        wait "$DAEMON_PID" 2>/dev/null || true
    fi
    exit "$EXIT_CODE"
fi

exec fx-server "$@"
