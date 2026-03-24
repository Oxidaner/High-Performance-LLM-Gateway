package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/internal/config"
	apierrors "llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

func TestRateLimit_ModelLimitUsesJSONModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := config.RateLimitConfig{
		GlobalQPS: 100,
		Burst:     100,
		ModelLimits: map[string]int{
			"gpt-4": 1,
		},
	}

	router := gin.New()
	router.POST("/chat", RateLimit(cfg), func(c *gin.Context) {
		var req struct {
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"model": req.Model})
	})

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}]}`)

	req1 := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request status %d, got %d body=%s", http.StatusOK, rec1.Code, rec1.Body.String())
	}

	var okResp map[string]string
	if err := json.Unmarshal(rec1.Body.Bytes(), &okResp); err != nil {
		t.Fatalf("failed to parse first response: %v", err)
	}
	if okResp["model"] != "gpt-4" {
		t.Fatalf("expected handler to receive model %q, got %q", "gpt-4", okResp["model"])
	}

	req2 := httptest.NewRequest(http.MethodPost, "/chat", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status %d, got %d body=%s", http.StatusTooManyRequests, rec2.Code, rec2.Body.String())
	}

	var errResp apierrors.ErrorResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp.Error.Code != apierrors.ErrCodeRateLimitExceeded {
		t.Fatalf("expected error code %q, got %q", apierrors.ErrCodeRateLimitExceeded, errResp.Error.Code)
	}
}

func TestRateLimit_GlobalLimit(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	cfg := config.RateLimitConfig{
		GlobalQPS: 1,
		Burst:     1,
	}

	router := gin.New()
	router.GET("/healthz", RateLimit(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request status %d, got %d", http.StatusOK, rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request status %d, got %d", http.StatusTooManyRequests, rec2.Code)
	}
}
