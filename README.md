# otel-genai-playground

Send OpenTelemetry traces, metrics and logs from three AI agents to one local backend and compare
what each emits against the [OpenTelemetry GenAI semantic conventions](https://github.com/open-telemetry/semantic-conventions-genai)
(status: Development).

| Agent | Directory | Model | Signals |
|---|---|---|---|
| [Google ADK for Go](https://github.com/google/adk-go) | `adk-go/` | fake LLM in code, no API key | traces, logs |
| [OpenCode](https://github.com/anomalyco/opencode) | `opencode/` | any configured provider | traces, logs |
| [Claude Code](https://code.claude.com/docs/en/monitoring-usage) | `claude-code/` | your Claude account | traces (beta), metrics, logs |

The backend is [grafana/otel-lgtm](https://github.com/grafana/docker-otel-lgtm): an OpenTelemetry Collector,
Tempo, Loki, Prometheus and Grafana in a single container.

## Requirements

- Docker
- Go 1.26.10+ (for `adk-go/`)
- `opencode` and/or `claude` CLI, logged in

## Usage

```sh
docker compose up -d   # Grafana on http://127.0.0.1:3000, OTLP on 127.0.0.1:4317 (gRPC) / 4318 (HTTP)

./adk-go/run.sh
MODEL=anthropic/claude-haiku-5-5 ./opencode/run.sh
./claude-code/run.sh
```

Each script runs one prompt that makes the agent call one tool (read `workspace/weather.txt`, or a
`get_weather` function in ADK). Open Grafana > Explore > Tempo and search by `service.name`:
`adk-go-fake-agent`, `opencode`, `claude-code`. Tempo search can lag ingestion by 10-30 seconds.

Useful TraceQL:

```
{ span.gen_ai.operation.name = "execute_tool" }
{ resource.service.name = "claude-code" && name = "claude_code.llm_request" }
```

Ports are bound to `127.0.0.1` only: Grafana runs with anonymous admin and the OTLP receivers have no auth.

## What each agent emits

Observed with adk-go v2.7.1, OpenCode v2.2.5 and Claude Code v2.1.314.

### ADK for Go

Follows GenAI semconv v1.44 by default. Span tree:

```
invoke_workflow weather_agent        gen_ai.operation.name, gen_ai.workflow.name, gen_ai.conversation.id
└── invoke_agent weather_agent       gen_ai.agent.name, gen_ai.agent.description
    ├── generate_content fake-llm    gen_ai.request.model, gen_ai.usage.input_tokens/output_tokens,
    │                                gen_ai.response.finish_reasons, gen_ai.input/output.messages (opt-in)
    ├── execute_tool get_weather     gen_ai.tool.name, gen_ai.tool.call.id, gen_ai.tool.call.arguments/result
    └── generate_content fake-llm
```

- `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT` = `SPAN_ONLY` / `EVENT_ONLY` / `SPAN_AND_EVENT` records message content (off by default).
- `ADK_TELEMETRY_SCHEMA_VERSION_OPT_IN=otel_semconv_1_36` restores the pre-1.44 attribute format.
- `telemetry.New()` reads `OTEL_EXPORTER_OTLP_ENDPOINT` and exports over OTLP/HTTP.
- The fake model sets no `gen_ai.provider.name`; real model packages (`gemini`, `anthropicmodel`, `openaimodel`) do.

### OpenCode

Exports when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (V1's `experimental.openTelemetry` is ignored in V2).
Span names are the internal Effect function names: `Session.prompt`, `SessionRunner.runStep`,
`SessionStep.attempt`, `Tool.execute`, `ReadTool.read`, ...

- v2.2.5: session spans nest under one `ServerProcess.start` trace and carry no attributes.
- v2.2.6+: each session turn is its own trace; model and tool spans carry `gen_ai.provider.name`,
  `gen_ai.request.model`, token usage, `gen_ai.tool.name`, `gen_ai.tool.call.id` and `session.id`
  ([anomalyco/opencode#60974](https://github.com/anomalyco/opencode/pull/60974)).

### Claude Code

Custom `claude_code.*` names with GenAI attributes added on top:

```
claude_code.interaction              gen_ai.operation.name=invoke_agent, gen_ai.conversation.id
├── claude_code.llm_request          gen_ai.request.model, gen_ai.response.finish_reasons,
│                                    input_tokens, output_tokens, cache_read_tokens, ttft_ms (custom names)
├── claude_code.tool                 gen_ai.operation.name=execute_tool, gen_ai.tool.name, gen_ai.tool.call.id
│   ├── claude_code.tool.blocked_on_user   decision, source
│   └── claude_code.tool.execution         success, duration_ms
└── claude_code.llm_request
```

- Metrics: `claude_code.token.usage` (by `type`: input / output / cacheRead / cacheCreation),
  `claude_code.cost.usage`, `claude_code.session.count`, `claude_code.active_time.total`.
- Events in Loki: `claude_code.user_prompt`, `api_request`, `tool_decision`, `tool_result`, `assistant_response`, hook and plugin events.
- Prompt text and tool input are redacted unless `OTEL_LOG_USER_PROMPTS=1` / `OTEL_LOG_TOOL_DETAILS=1`.
- Metrics default to delta temporality. With delta, no `claude_code_*` series showed up in the otel-lgtm Prometheus; `run.sh` sets `cumulative`.
- Every signal carries account identity attributes (`user.email`, `organization.id`, ...). Mind this before sharing screenshots.

## Cleanup

```sh
docker compose down
```
