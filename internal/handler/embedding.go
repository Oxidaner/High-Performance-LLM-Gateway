package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"llm-gateway/internal/config"
)

// EmbeddingHandler handles embedding requests
func EmbeddingHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req EmbeddingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "invalid_request_error",
				},
			})
			return
		}

		if req.Model == "" {
			req.Model = "text-embedding-ada-002"
		}

		// Forward to Python Worker for embedding
		resp, err := generateEmbedding(c, cfg, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":   "server_error",
				},
			})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Input interface{} `json:"input"` // string or []string
	Model string    `json:"model"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Object string       `json:"object"`
	Data   []EmbedEntry `json:"data"`
	Model  string       `json:"model"`
	Usage  Usage        `json:"usage"`
}

// EmbedEntry represents a single embedding
type EmbedEntry struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

func generateEmbedding(c *gin.Context, cfg *config.Config, req EmbeddingRequest) (*EmbeddingResponse, error) {
	// Call Python Worker
	workerURL := cfg.PythonWorker.Address + "/embeddings"

	jsonData, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest("POST", workerURL, bytes.NewBuffer(jsonData))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: cfg.PythonWorker.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{Message: string(body)}
	}

	var embeddingResp EmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, err
	}

	return &embeddingResp, nil
}

// APIError represents an API error
type APIError struct {
	Message string
}

func (e *APIError) Error() string {
	return e.Message
}
