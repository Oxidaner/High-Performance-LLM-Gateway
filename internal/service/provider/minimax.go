package provider

import (
	"net/http"

	"llm-gateway/internal/config"
)

func newMiniMaxProvider(cfg config.ProviderConfig, client *http.Client) Client {
	return newOpenAICompatibleProvider("minimax", cfg.BaseURL, cfg.APIKey, client)
}
