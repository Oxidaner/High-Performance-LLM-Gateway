package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const defaultProviderTimeout = 30 * time.Second

type openAICompatibleProvider struct {
	name    string
	baseURL string
	apiKey  string
	client  *http.Client
}

func newOpenAICompatibleProvider(name, baseURL, apiKey string, client *http.Client) *openAICompatibleProvider {
	if client == nil {
		client = &http.Client{Timeout: defaultProviderTimeout}
	}
	return &openAICompatibleProvider{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  client,
	}
}

func (p *openAICompatibleProvider) Name() string {
	return p.name
}

func (p *openAICompatibleProvider) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
	tracer := otel.Tracer("llm-gateway/provider/" + p.name)
	ctx, span := tracer.Start(ctx, "chat.completions")
	span.SetAttributes(
		attribute.String("provider.name", p.name),
		attribute.String("request.model", req.Model),
	)
	defer span.End()

	if p.baseURL == "" || p.apiKey == "" {
		span.SetStatus(codes.Error, "provider not configured")
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusServiceUnavailable,
			Message:    fmt.Sprintf("%s provider is not configured", p.name),
		}
	}

	payload, err := json.Marshal(req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusBadRequest,
			Message:    "failed to marshal chat request",
			Err:        err,
		}
	}

	reqURL := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewBuffer(payload))
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusBadGateway,
			Message:    "failed to build upstream request",
			Err:        err,
		}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusServiceUnavailable,
			Message:    "failed to call upstream provider",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusBadGateway,
			Message:    "failed to read upstream response",
			Err:        err,
		}
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
		span.SetStatus(codes.Error, fmt.Sprintf("http %d", resp.StatusCode))
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: resp.StatusCode,
			Message:    extractUpstreamMessage(resp.StatusCode, body),
			Body:       string(body),
		}
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, &UpstreamError{
			Provider:   p.name,
			StatusCode: http.StatusBadGateway,
			Message:    "failed to parse upstream response",
			Body:       string(body),
			Err:        err,
		}
	}

	if strings.TrimSpace(chatResp.Model) == "" {
		chatResp.Model = req.Model
	}
	chatResp.Provider = p.name
	span.SetAttributes(
		attribute.Int("http.status_code", http.StatusOK),
		attribute.String("response.model", chatResp.Model),
		attribute.Int("usage.total_tokens", chatResp.Usage.TotalTokens),
	)
	span.SetStatus(codes.Ok, "")

	return &chatResp, nil
}

func extractUpstreamMessage(statusCode int, body []byte) string {
	if len(body) == 0 {
		return fmt.Sprintf("upstream provider returned status %d", statusCode)
	}

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Error.Message != "" {
			return envelope.Error.Message
		}
		if envelope.Message != "" {
			return envelope.Message
		}
	}

	msg := strings.TrimSpace(string(body))
	if len(msg) > 512 {
		msg = msg[:512]
	}
	return msg
}
