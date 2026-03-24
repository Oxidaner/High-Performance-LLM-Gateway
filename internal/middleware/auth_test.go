package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llm-gateway/internal/storage"
	apierrors "llm-gateway/pkg/errors"

	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuth_MissingAuthorizationHeader(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", APIKeyAuth(new(storage.RedisClient)), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp apierrors.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != apierrors.ErrCodeMissingAPIKey {
		t.Fatalf("expected error code %q, got %q", apierrors.ErrCodeMissingAPIKey, resp.Error.Code)
	}
}

func TestAPIKeyAuth_InvalidAuthorizationHeaderFormat(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/secure", APIKeyAuth(new(storage.RedisClient)), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Token abc")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var resp apierrors.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error.Code != apierrors.ErrCodeInvalidAPIKey {
		t.Fatalf("expected error code %q, got %q", apierrors.ErrCodeInvalidAPIKey, resp.Error.Code)
	}
}
