package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"llm-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

func TestWorkflowSummaryEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	replayPath := filepath.Join(t.TempDir(), "workflow_replay.jsonl")
	service.ConfigureDefaultWorkflowTracer(replayPath)
	tracer := service.DefaultWorkflowTracer()
	tracer.Record(context.Background(), service.WorkflowTrace{
		SessionID:        "session-1",
		StepID:           "step-1",
		StepType:         "planning",
		Endpoint:         "/v1/chat/completions",
		RequestedModel:   "gpt-4",
		UpstreamModel:    "gpt-3.5-turbo",
		UpstreamProvider: "openai",
		StatusCode:       200,
		LatencyMs:        50,
		TotalTokens:      12,
	})

	router := gin.New()
	router.GET("/api/v1/workflows/:session_id/summary", GetWorkflowSummary())
	router.GET("/api/v1/workflows/summaries", ListWorkflowSummaries())

	reqSummary := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/session-1/summary", nil)
	recSummary := httptest.NewRecorder()
	router.ServeHTTP(recSummary, reqSummary)
	if recSummary.Code != http.StatusOK {
		t.Fatalf("expected summary status %d, got %d body=%s", http.StatusOK, recSummary.Code, recSummary.Body.String())
	}

	var summaryResp map[string]interface{}
	if err := json.Unmarshal(recSummary.Body.Bytes(), &summaryResp); err != nil {
		t.Fatalf("failed to decode summary response: %v", err)
	}
	if summaryResp["session_id"] != "session-1" {
		t.Fatalf("expected session_id=session-1, got %v", summaryResp["session_id"])
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/workflows/summaries?limit=1", nil)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d body=%s", http.StatusOK, recList.Code, recList.Body.String())
	}

	var listResp map[string]interface{}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if listResp["object"] != "list" {
		t.Fatalf("expected object=list, got %v", listResp["object"])
	}
}
