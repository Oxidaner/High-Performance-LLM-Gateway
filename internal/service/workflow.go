package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultWorkflowReplayPath = "logs/workflow_replay.jsonl"

// WorkflowTrace is one replay-friendly workflow trace event.
type WorkflowTrace struct {
	TraceID          string  `json:"trace_id"`
	Timestamp        string  `json:"timestamp"`
	Endpoint         string  `json:"endpoint"`
	SessionID        string  `json:"session_id,omitempty"`
	StepID           string  `json:"step_id,omitempty"`
	StepType         string  `json:"step_type,omitempty"` // planning/execution/summarization
	Tool             string  `json:"tool,omitempty"`
	RequestedModel   string  `json:"requested_model,omitempty"`
	UpstreamModel    string  `json:"upstream_model,omitempty"`
	UpstreamProvider string  `json:"upstream_provider,omitempty"`
	StatusCode       int     `json:"status_code"`
	LatencyMs        int64   `json:"latency_ms"`
	CacheHit         bool    `json:"cache_hit"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	EstimatedCost    float64 `json:"estimated_cost"`
	ErrorCode        string  `json:"error_code,omitempty"`
}

// WorkflowSummary aggregates one workflow session.
type WorkflowSummary struct {
	SessionID     string                  `json:"session_id"`
	Requests      int64                   `json:"requests"`
	Errors        int64                   `json:"errors"`
	TotalTokens   int64                   `json:"total_tokens"`
	EstimatedCost float64                 `json:"estimated_cost"`
	AvgLatencyMs  float64                 `json:"avg_latency_ms"`
	CacheHitRate  float64                 `json:"cache_hit_rate"`
	ByStepType    map[string]PhaseSummary `json:"by_step_type"`
	ByTool        map[string]ToolSummary  `json:"by_tool"`
	LastUpdatedAt string                  `json:"last_updated_at"`
}

// PhaseSummary is an aggregated step-type view.
type PhaseSummary struct {
	Requests      int64   `json:"requests"`
	Errors        int64   `json:"errors"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	TotalTokens   int64   `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
}

// ToolSummary is an aggregated tool view.
type ToolSummary struct {
	Requests      int64   `json:"requests"`
	Errors        int64   `json:"errors"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	TotalTokens   int64   `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
}

// WorkflowTracer records workflow traces and keeps in-memory summaries.
type WorkflowTracer struct {
	mu         sync.RWMutex
	writeMu    sync.Mutex
	replayPath string
	sessions   map[string]*workflowSummaryState
}

type workflowSummaryState struct {
	summary       WorkflowSummary
	totalLatency  int64
	cacheHits     int64
	phaseLatency  map[string]int64
	phaseRequests map[string]int64
	toolLatency   map[string]int64
	toolRequests  map[string]int64
}

var (
	defaultWorkflowTracerMu sync.RWMutex
	defaultWorkflowTracer   = NewWorkflowTracer(defaultWorkflowReplayPath)
)

// NewWorkflowTracer creates a workflow tracer.
func NewWorkflowTracer(replayPath string) *WorkflowTracer {
	replayPath = strings.TrimSpace(replayPath)
	if replayPath == "" {
		replayPath = defaultWorkflowReplayPath
	}

	return &WorkflowTracer{
		replayPath: replayPath,
		sessions:   make(map[string]*workflowSummaryState),
	}
}

// ConfigureDefaultWorkflowTracer updates the singleton tracer configuration.
func ConfigureDefaultWorkflowTracer(replayPath string) {
	defaultWorkflowTracerMu.Lock()
	defer defaultWorkflowTracerMu.Unlock()
	defaultWorkflowTracer = NewWorkflowTracer(replayPath)
}

// DefaultWorkflowTracer returns the singleton workflow tracer.
func DefaultWorkflowTracer() *WorkflowTracer {
	defaultWorkflowTracerMu.RLock()
	defer defaultWorkflowTracerMu.RUnlock()
	return defaultWorkflowTracer
}

// Record appends a replay event and updates in-memory summary.
func (t *WorkflowTracer) Record(ctx context.Context, trace WorkflowTrace) {
	_ = ctx
	trace = normalizeTrace(trace)

	_ = t.appendJSONL(trace)
	t.updateSummary(trace)
}

// GetSummary returns one session summary.
func (t *WorkflowTracer) GetSummary(sessionID string) (WorkflowSummary, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return WorkflowSummary{}, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	state, ok := t.sessions[sessionID]
	if !ok || state == nil {
		return WorkflowSummary{}, false
	}

	return cloneWorkflowSummary(state.summary), true
}

// ListSummaries returns latest workflow summaries ordered by update time desc.
func (t *WorkflowTracer) ListSummaries(limit int) []WorkflowSummary {
	if limit <= 0 {
		limit = 20
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	items := make([]WorkflowSummary, 0, len(t.sessions))
	for _, state := range t.sessions {
		if state == nil {
			continue
		}
		items = append(items, cloneWorkflowSummary(state.summary))
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].LastUpdatedAt > items[j].LastUpdatedAt
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func (t *WorkflowTracer) appendJSONL(trace WorkflowTrace) error {
	if strings.TrimSpace(t.replayPath) == "" {
		return nil
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	dir := filepath.Dir(t.replayPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(t.replayPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(trace)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (t *WorkflowTracer) updateSummary(trace WorkflowTrace) {
	if strings.TrimSpace(trace.SessionID) == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.sessions[trace.SessionID]
	if !ok {
		state = &workflowSummaryState{
			summary: WorkflowSummary{
				SessionID:  trace.SessionID,
				ByStepType: make(map[string]PhaseSummary),
				ByTool:     make(map[string]ToolSummary),
			},
			phaseLatency:  make(map[string]int64),
			phaseRequests: make(map[string]int64),
			toolLatency:   make(map[string]int64),
			toolRequests:  make(map[string]int64),
		}
		t.sessions[trace.SessionID] = state
	}

	s := &state.summary
	s.Requests++
	if trace.StatusCode >= 400 {
		s.Errors++
	}
	if trace.TotalTokens > 0 {
		s.TotalTokens += int64(trace.TotalTokens)
	}
	s.EstimatedCost += trace.EstimatedCost
	state.totalLatency += trace.LatencyMs
	s.AvgLatencyMs = float64(state.totalLatency) / float64(s.Requests)
	if trace.CacheHit {
		state.cacheHits++
	}
	s.CacheHitRate = float64(state.cacheHits) / float64(s.Requests)
	s.LastUpdatedAt = trace.Timestamp

	phase := strings.TrimSpace(trace.StepType)
	if phase == "" {
		phase = "default"
	}
	phaseSummary := s.ByStepType[phase]
	phaseSummary.Requests++
	if trace.StatusCode >= 400 {
		phaseSummary.Errors++
	}
	if trace.TotalTokens > 0 {
		phaseSummary.TotalTokens += int64(trace.TotalTokens)
	}
	phaseSummary.EstimatedCost += trace.EstimatedCost
	state.phaseLatency[phase] += trace.LatencyMs
	state.phaseRequests[phase]++
	phaseSummary.AvgLatencyMs = float64(state.phaseLatency[phase]) / float64(state.phaseRequests[phase])
	s.ByStepType[phase] = phaseSummary

	tool := strings.TrimSpace(trace.Tool)
	if tool != "" {
		toolSummary := s.ByTool[tool]
		toolSummary.Requests++
		if trace.StatusCode >= 400 {
			toolSummary.Errors++
		}
		if trace.TotalTokens > 0 {
			toolSummary.TotalTokens += int64(trace.TotalTokens)
		}
		toolSummary.EstimatedCost += trace.EstimatedCost
		state.toolLatency[tool] += trace.LatencyMs
		state.toolRequests[tool]++
		toolSummary.AvgLatencyMs = float64(state.toolLatency[tool]) / float64(state.toolRequests[tool])
		s.ByTool[tool] = toolSummary
	}
}

func normalizeTrace(trace WorkflowTrace) WorkflowTrace {
	if strings.TrimSpace(trace.TraceID) == "" {
		trace.TraceID = newTraceID()
	}
	if strings.TrimSpace(trace.Timestamp) == "" {
		trace.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	trace.Endpoint = strings.TrimSpace(trace.Endpoint)
	if trace.Endpoint == "" {
		trace.Endpoint = "unknown"
	}
	trace.SessionID = strings.TrimSpace(trace.SessionID)
	trace.StepID = strings.TrimSpace(trace.StepID)
	trace.StepType = strings.ToLower(strings.TrimSpace(trace.StepType))
	trace.Tool = strings.TrimSpace(trace.Tool)
	trace.RequestedModel = strings.TrimSpace(trace.RequestedModel)
	trace.UpstreamModel = strings.TrimSpace(trace.UpstreamModel)
	trace.UpstreamProvider = strings.TrimSpace(trace.UpstreamProvider)
	if trace.StatusCode == 0 {
		trace.StatusCode = 200
	}
	if trace.LatencyMs < 0 {
		trace.LatencyMs = 0
	}
	if trace.PromptTokens < 0 {
		trace.PromptTokens = 0
	}
	if trace.CompletionTokens < 0 {
		trace.CompletionTokens = 0
	}
	if trace.TotalTokens < 0 {
		trace.TotalTokens = 0
	}
	if trace.TotalTokens == 0 {
		trace.TotalTokens = trace.PromptTokens + trace.CompletionTokens
	}
	if trace.EstimatedCost <= 0 {
		trace.EstimatedCost = estimateCost(trace.UpstreamModel, trace.PromptTokens, trace.CompletionTokens, trace.TotalTokens)
	}
	trace.ErrorCode = strings.TrimSpace(trace.ErrorCode)
	return trace
}

func estimateCost(model string, promptTokens, completionTokens, totalTokens int) float64 {
	model = strings.ToLower(strings.TrimSpace(model))

	var promptPer1k, completionPer1k float64
	switch {
	case strings.HasPrefix(model, "gpt-4"):
		promptPer1k = 0.03
		completionPer1k = 0.06
	case strings.HasPrefix(model, "gpt-3.5"):
		promptPer1k = 0.0015
		completionPer1k = 0.002
	case strings.HasPrefix(model, "claude"):
		promptPer1k = 0.008
		completionPer1k = 0.024
	default:
		// Fallback estimate: $2 per 1M tokens.
		if totalTokens <= 0 {
			return 0
		}
		return float64(totalTokens) * 0.000002
	}

	prompt := float64(promptTokens) / 1000.0 * promptPer1k
	completion := float64(completionTokens) / 1000.0 * completionPer1k
	if prompt == 0 && completion == 0 && totalTokens > 0 {
		return float64(totalTokens) * 0.000002
	}
	return prompt + completion
}

func newTraceID() string {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("wf_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("wf_%d_%s", time.Now().UnixNano(), hex.EncodeToString(random))
}

func cloneWorkflowSummary(in WorkflowSummary) WorkflowSummary {
	out := in
	out.ByStepType = make(map[string]PhaseSummary, len(in.ByStepType))
	for k, v := range in.ByStepType {
		out.ByStepType[k] = v
	}
	out.ByTool = make(map[string]ToolSummary, len(in.ByTool))
	for k, v := range in.ByTool {
		out.ByTool[k] = v
	}
	return out
}
