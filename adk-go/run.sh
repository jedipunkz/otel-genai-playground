#!/usr/bin/env bash
# Run the ADK agent (fake LLM, no API key) with traces and logs exported to the local LGTM stack.
set -euo pipefail

cd "$(dirname "$0")"

export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
# NO_CONTENT (default) | SPAN_ONLY | EVENT_ONLY | SPAN_AND_EVENT
export OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT="${CAPTURE:-SPAN_AND_EVENT}"

go run .
