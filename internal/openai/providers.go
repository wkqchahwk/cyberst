package openai

import (
	"net/http"
	"strings"

	"cyberstrike-ai/internal/config"
)

const (
	ProviderOpenAI     = "openai"
	ProviderAnthropic  = "anthropic"
	ProviderOpenRouter = "openrouter"
	ProviderOllama     = "ollama"
	ProviderOllamaCloud = "ollama_cloud"
	ProviderCustom     = "custom"
)

// NormalizeProvider maps legacy and alias names to stable provider IDs.
func NormalizeProvider(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "", ProviderOpenAI:
		return ProviderOpenAI
	case "claude", ProviderAnthropic:
		return ProviderAnthropic
	case ProviderOpenRouter:
		return ProviderOpenRouter
	case ProviderOllama:
		return ProviderOllama
	case ProviderOllamaCloud, "ollama-cloud", "ollamacloud":
		return ProviderOllamaCloud
	case ProviderCustom, "openai_compatible", "openai-compatible", "compatible":
		return ProviderCustom
	default:
		return strings.TrimSpace(strings.ToLower(provider))
	}
}

// DefaultBaseURLForProvider returns the official or recommended default endpoint.
func DefaultBaseURLForProvider(provider string) string {
	switch NormalizeProvider(provider) {
	case ProviderAnthropic:
		return "https://api.anthropic.com"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api/v1"
	case ProviderOllama:
		return "http://localhost:11434/v1"
	case ProviderOllamaCloud:
		return "https://ollama.com/v1"
	case ProviderCustom:
		return ""
	default:
		return "https://api.openai.com/v1"
	}
}

// ProviderRequiresAPIKey returns whether the provider should reject empty API keys.
func ProviderRequiresAPIKey(provider string) bool {
	switch NormalizeProvider(provider) {
	case ProviderOllama:
		return false
	default:
		return true
	}
}

// ResolveBaseURL applies provider defaults when the config leaves BaseURL empty.
func ResolveBaseURL(cfg *config.OpenAIConfig) string {
	if cfg == nil {
		return DefaultBaseURLForProvider(ProviderOpenAI)
	}

	baseURL := strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL != "" {
		return baseURL
	}
	return strings.TrimSuffix(DefaultBaseURLForProvider(cfg.Provider), "/")
}

// ApplyOpenAICompatibleHeaders sets the headers shared by OpenAI-compatible providers.
func ApplyOpenAICompatibleHeaders(req *http.Request, cfg *config.OpenAIConfig) {
	req.Header.Set("Content-Type", "application/json")
	if cfg == nil {
		return
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
}
