package cache

import (
	"context"
	"testing"
)

func TestGenerateCacheKey_DeterministicAcrossMapInsertionOrder(t *testing.T) {
	t.Parallel()

	messages := []Message{
		{Role: "user", Content: "hello"},
	}

	paramsA := map[string]interface{}{
		"temperature": 0.7,
		"top_p":       0.9,
		"max_tokens":  128,
	}
	paramsB := map[string]interface{}{
		"max_tokens":  128,
		"top_p":       0.9,
		"temperature": 0.7,
	}

	keyA := GenerateCacheKey("gpt-4", messages, paramsA)
	keyB := GenerateCacheKey("gpt-4", messages, paramsB)

	if keyA != keyB {
		t.Fatalf("expected stable key generation, got %q != %q", keyA, keyB)
	}
}

func TestL1Cache_NilClientNoop(t *testing.T) {
	t.Parallel()

	cache := NewL1Cache(nil, L1CacheConfig{
		TTL:       3600,
		MaxSize:   10,
		KeyPrefix: "cache:l1",
	})
	ctx := context.Background()

	if err := cache.Set(ctx, "k1", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("expected nil error for Set with nil client, got %v", err)
	}

	val, err := cache.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("expected nil error for Get with nil client, got %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil value with nil client, got %q", string(val))
	}

	stats, err := cache.Stats(ctx)
	if err != nil {
		t.Fatalf("expected nil error for Stats with nil client, got %v", err)
	}
	enabled, _ := stats["enabled"].(bool)
	if enabled {
		t.Fatalf("expected cache enabled=false for nil client stats")
	}
}

func TestChatResponseMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	resp := &ChatResponse{
		ID:      "chatcmpl-1",
		Object:  "chat.completion",
		Created: 1,
		Model:   "gpt-4",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
	}

	data, err := resp.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	out, err := UnmarshalChatResponse(data)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if out.Model != resp.Model || len(out.Choices) != 1 || out.Choices[0].Message.Content != "hello" {
		t.Fatalf("unexpected round trip output: %+v", out)
	}
}
