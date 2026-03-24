package provider

import (
	"net/http"

	"llm-gateway/internal/config"
)

func newOpenAIProvider(cfg config.ProviderConfig, client *http.Client) Client {
	return newOpenAICompatibleProvider("openai", cfg.BaseURL, cfg.APIKey, client)
}
