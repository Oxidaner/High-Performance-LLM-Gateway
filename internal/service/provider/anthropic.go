package provider

import (
	"net/http"

	"llm-gateway/internal/config"
)

func newAnthropicProvider(cfg config.ProviderConfig, client *http.Client) Client {
	return newOpenAICompatibleProvider("anthropic", cfg.BaseURL, cfg.APIKey, client)
}
