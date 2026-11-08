#!/usr/bin/env bash
# Run one Claude Code prompt with metrics, logs (events) and traces exported to the local LGTM stack.
set -euo pipefail

cd "$(dirname "$0")/../workspace"

export CLAUDE_CODE_ENABLE_TELEMETRY=1
export CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1 # traces are beta and off without this
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_TRACES_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
# Claude Code defaults to delta; with delta no claude_code_* series showed up in the otel-lgtm Prometheus.
export OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE=cumulative
export OTEL_METRIC_EXPORT_INTERVAL=5000
# Uncomment to record prompt text / tool input (off by default, may contain sensitive data).
# export OTEL_LOG_USER_PROMPTS=1
# export OTEL_LOG_TOOL_DETAILS=1

claude -p --model "${MODEL:-haiku}" --allowedTools=Read \
  "${1:-Read weather.txt and answer in one sentence: what is the weather in Tokyo?}"
