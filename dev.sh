#!/usr/bin/env bash
# Spins up the full local dev stack in one tmux session:
#   docker   - db/tap/appview/clustering (docker-compose.dev.yml), window just tails logs
#   inference - uvicorn, native (needs Apple Silicon MPS, can't run in Docker)
#   frontend - vite dev --host, native
#   tunnel   - cloudflared tunnel to expose localhost:8080 for mobile/remote testing
set -euo pipefail

SESSION="currents-dev"
TUNNEL_NAME="currents-dev"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

start_window() {
  local name="$1" dir="$2" run="$3"
  tmux new-window -t "$SESSION" -n "$name" -c "$dir" "$run; exec \$SHELL"
}

case "${1:-start}" in
  start)
    if tmux has-session -t "$SESSION" 2>/dev/null; then
      echo "Session '$SESSION' already running, attaching. (Use '$0 restart <window>' to bounce one process.)"
      exec tmux attach -t "$SESSION"
    fi

    echo "Building and starting docker compose stack (db, tap, appview, clustering)..."
    (cd "$ROOT" && docker compose -f docker-compose.dev.yml up -d --build --force-recreate)

    tmux new-session -d -s "$SESSION" -n docker -c "$ROOT" \
      "docker compose -f docker-compose.dev.yml logs -f; exec \$SHELL"
    start_window docker-shell "$ROOT" "exec \$SHELL"
    start_window inference "$ROOT/inference" "source venv/bin/activate && uvicorn main:app --reload"
    start_window frontend "$ROOT/frontend" "npm run dev -- --host"
    start_window tunnel "$ROOT" "cloudflared tunnel run --url http://localhost:8080 $TUNNEL_NAME"

    tmux select-window -t "$SESSION:docker"
    echo "Attaching to tmux session '$SESSION' (windows: docker, docker-shell, inference, frontend, tunnel)."
    echo "  docker-shell is a free prompt for rebuilds, e.g.: docker compose -f docker-compose.dev.yml up -d --build appview"
    echo "  switch windows:      Ctrl-b <number>   or   Ctrl-b w"
    echo "  detach (keep running): Ctrl-b d"
    exec tmux attach -t "$SESSION"
    ;;

  stop)
    if tmux has-session -t "$SESSION" 2>/dev/null; then
      tmux kill-session -t "$SESSION"
      echo "Stopped inference, frontend, and tunnel."
    else
      echo "No tmux session '$SESSION' running."
    fi
    if [[ "${2:-}" == "--down" ]]; then
      (cd "$ROOT" && docker compose -f docker-compose.dev.yml down)
    else
      echo "Docker compose stack left running (rebuilds are slow). Use '$0 stop --down' to also stop it."
    fi
    ;;

  attach)
    exec tmux attach -t "$SESSION"
    ;;

  restart)
    win="${2:-}"
    if [[ -z "$win" ]]; then
      echo "Usage: $0 restart <docker|inference|frontend|tunnel>" >&2
      exit 1
    fi
    tmux send-keys -t "$SESSION:$win" C-c
    sleep 0.3
    case "$win" in
      docker) tmux send-keys -t "$SESSION:$win" "docker compose -f docker-compose.dev.yml logs -f" Enter ;;
      inference) tmux send-keys -t "$SESSION:$win" "source venv/bin/activate && uvicorn main:app --reload" Enter ;;
      frontend) tmux send-keys -t "$SESSION:$win" "npm run dev -- --host" Enter ;;
      tunnel) tmux send-keys -t "$SESSION:$win" "cloudflared tunnel run --url http://localhost:8080 $TUNNEL_NAME" Enter ;;
      *) echo "Unknown window '$win' (expected docker|inference|frontend|tunnel)" >&2; exit 1 ;;
    esac
    ;;

  status)
    tmux list-windows -t "$SESSION" 2>/dev/null || echo "No tmux session '$SESSION' running."
    ;;

  *)
    echo "Usage: $0 [start|stop [--down]|attach|restart <window>|status]" >&2
    exit 1
    ;;
esac
