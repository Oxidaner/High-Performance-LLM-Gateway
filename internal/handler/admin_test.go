package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetStats_FallbackToInMemoryWhenDBUnavailable(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/api/v1/stats", GetStats(nil))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode stats response: %v", err)
	}
	if _, ok := payload["total_requests"]; !ok {
		t.Fatalf("expected total_requests field in stats response")
	}
	if _, ok := payload["models"]; !ok {
		t.Fatalf("expected models field in stats response")
	}
}
