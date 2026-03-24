#!/usr/bin/env bash
set -euo pipefail

URL="${URL:-http://localhost:8080/v1/chat/completions}"
API_KEY="${API_KEY:-}"
MODEL="${MODEL:-gpt-4}"
PROMPT="${PROMPT:-hello from loadtest}"
REQUESTS="${REQUESTS:-200}"
CONCURRENCY="${CONCURRENCY:-20}"
TIMEOUT="${TIMEOUT:-10s}"

go run ./scripts/loadtest \
  -url "$URL" \
  -api-key "$API_KEY" \
  -model "$MODEL" \
  -prompt "$PROMPT" \
  -requests "$REQUESTS" \
  -concurrency "$CONCURRENCY" \
  -timeout "$TIMEOUT"
