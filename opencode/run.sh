#!/usr/bin/env bash
# Run one OpenCode prompt with logs and traces exported to the local LGTM stack.
set -euo pipefail

: "${MODEL:?set MODEL to provider/model, e.g. MODEL=anthropic/claude-haiku-5-5 (see: opencode models)}"

cd "$(dirname "$0")/../workspace"

export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318

# --standalone starts a private server as a child of this shell so it inherits OTEL_*
# (the shared background service runs as a separate process).
opencode run --standalone -m "$MODEL" \
  "${1:-Read weather.txt and answer in one sentence: what is the weather in Tokyo?}"
