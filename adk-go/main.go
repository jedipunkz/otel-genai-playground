// Command adk-go runs an ADK agent backed by a fake LLM and exports
// OpenTelemetry GenAI traces and logs over OTLP. No model API key is needed.
package main

import (
	"context"
	"fmt"
	"iter"
	"log"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/telemetry"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// fakeLLM asks for the get_weather tool on the first turn and answers with
// text once the tool result is in the request. Token counts are fixed values.
type fakeLLM struct{}

func (fakeLLM) Name() string { return "fake-llm" }

func (fakeLLM) GenerateContent(_ context.Context, req *model.LLMRequest, _ bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		time.Sleep(200 * time.Millisecond) // make the span visible on a timeline

		var weather string
		if n := len(req.Contents); n > 0 {
			for _, p := range req.Contents[n-1].Parts {
				if p.FunctionResponse != nil {
					weather = fmt.Sprint(p.FunctionResponse.Response["forecast"])
				}
			}
		}

		var part *genai.Part
		if weather == "" {
			part = &genai.Part{FunctionCall: &genai.FunctionCall{
				ID:   "call-1",
				Name: "get_weather",
				Args: map[string]any{"city": "Tokyo"},
			}}
		} else {
			part = genai.NewPartFromText("Tokyo: " + weather)
		}

		yield(&model.LLMResponse{
			Content:      &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{part}},
			FinishReason: genai.FinishReasonStop,
			UsageMetadata: &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     120,
				CandidatesTokenCount: 30,
				TotalTokenCount:      150,
			},
		}, nil)
	}
}

type weatherIn struct {
	City string `json:"city"`
}

type weatherOut struct {
	Forecast string `json:"forecast"`
}

func getWeather(_ agent.Context, in weatherIn) (weatherOut, error) {
	time.Sleep(100 * time.Millisecond)
	return weatherOut{Forecast: "sunny in " + in.City}, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String("adk-go-fake-agent")))
	if err != nil {
		return err
	}
	// Exports via OTLP/HTTP when OTEL_EXPORTER_OTLP_ENDPOINT is set.
	providers, err := telemetry.New(ctx, telemetry.WithResource(res))
	if err != nil {
		return err
	}
	providers.SetGlobalOtelProviders()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := providers.Shutdown(shutdownCtx); err != nil {
			log.Printf("telemetry shutdown: %v", err)
		}
	}()

	weatherTool, err := functiontool.New(functiontool.Config{
		Name:        "get_weather",
		Description: "Returns the weather forecast for a city.",
	}, getWeather)
	if err != nil {
		return err
	}
	a, err := llmagent.New(llmagent.Config{
		Name:        "weather_agent",
		Description: "Answers weather questions.",
		Instruction: "Use get_weather to answer.",
		Model:       fakeLLM{},
		Tools:       []tool.Tool{weatherTool},
	})
	if err != nil {
		return err
	}

	r, err := runner.NewInMemory("otel-genai-playground", a)
	if err != nil {
		return err
	}
	msg := genai.NewContentFromText("What is the weather in Tokyo?", genai.RoleUser)
	for ev, err := range r.Run(ctx, "user-1", "session-1", msg, agent.RunConfig{}) {
		if err != nil {
			return err
		}
		if ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.Text != "" {
				fmt.Printf("%s: %s\n", ev.Author, p.Text)
			}
		}
	}
	return nil
}
