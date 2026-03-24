package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowTracer_RecordAndSummary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	replayPath := filepath.Join(dir, "workflow_replay.jsonl")
	tracer := NewWorkflowTracer(replayPath)

	tracer.Record(context.Background(), WorkflowTrace{
		SessionID:        "s1",
		StepID:           "step-1",
		StepType:         "planning",
		Tool:             "web_search",
		Endpoint:         "/v1/chat/completions",
		RequestedModel:   "gpt-4",
		UpstreamModel:    "gpt-3.5-turbo",
		UpstreamProvider: "openai",
		StatusCode:       200,
		LatencyMs:        120,
		CacheHit:         false,
		PromptTokens:     30,
		CompletionTokens: 20,
		TotalTokens:      50,
	})

	tracer.Record(context.Background(), WorkflowTrace{
		SessionID:        "s1",
		StepID:           "step-2",
		StepType:         "execution",
		Tool:             "db_query",
		Endpoint:         "/v1/chat/completions",
		RequestedModel:   "gpt-4",
		UpstreamModel:    "gpt-4",
		UpstreamProvider: "openai",
		StatusCode:       503,
		LatencyMs:        80,
		CacheHit:         true,
		PromptTokens:     10,
		CompletionTokens: 0,
		TotalTokens:      10,
		ErrorCode:        "model_overloaded",
	})

	summary, ok := tracer.GetSummary("s1")
	if !ok {
		t.Fatalf("expected summary for session s1")
	}
	if summary.Requests != 2 {
		t.Fatalf("expected requests=2, got %d", summary.Requests)
	}
	if summary.Errors != 1 {
		t.Fatalf("expected errors=1, got %d", summary.Errors)
	}
	if summary.TotalTokens != 60 {
		t.Fatalf("expected total tokens=60, got %d", summary.TotalTokens)
	}
	if summary.CacheHitRate != 0.5 {
		t.Fatalf("expected cache hit rate=0.5, got %f", summary.CacheHitRate)
	}
	if _, exists := summary.ByStepType["planning"]; !exists {
		t.Fatalf("expected planning phase summary")
	}
	if _, exists := summary.ByTool["web_search"]; !exists {
		t.Fatalf("expected tool summary for web_search")
	}

	content, err := os.ReadFile(replayPath)
	if err != nil {
		t.Fatalf("failed to read replay file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 replay lines, got %d", len(lines))
	}
}

func TestWorkflowTracer_ListSummaries(t *testing.T) {
	t.Parallel()

	tracer := NewWorkflowTracer(filepath.Join(t.TempDir(), "replay.jsonl"))
	tracer.Record(context.Background(), WorkflowTrace{SessionID: "s1", Endpoint: "/v1/chat/completions", StatusCode: 200})
	tracer.Record(context.Background(), WorkflowTrace{SessionID: "s2", Endpoint: "/v1/chat/completions", StatusCode: 200})

	items := tracer.ListSummaries(1)
	if len(items) != 1 {
		t.Fatalf("expected 1 item with limit=1, got %d", len(items))
	}
}
