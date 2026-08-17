package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Provider struct {
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
	BaseURL  string `json:"base_url"`
}

type Config struct {
	CurrentProvider string              `json:"current_provider"`
	Providers       map[string]Provider `json:"providers"`
	Language        string              `json:"language"`
	Emoji           bool                `json:"emoji"`
	Timeout         int                 `json:"timeout"`
}

func NewConfig() *Config {
	return &Config{
		Providers: make(map[string]Provider),
		Emoji:     true,
		Timeout:   60,
	}
}

func (c *Config) Load() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configFile := filepath.Join(homeDir, ".config", "aigit", "config.json")
	configData, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Return empty config if file doesn't exist
		}
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := json.Unmarshal(configData, c); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	if c.Language == "" {
		c.Language = detectLanguage()
	}

	return nil
}

func (c *Config) Save() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "aigit")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "config.json")
	jsonData, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(configFile, jsonData, 0o600); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

func (c *Config) AddProvider(provider, apiKey string, endpoint ...string) error {
	p := Provider{APIKey: apiKey}
	if len(endpoint) > 0 {
		p.Endpoint = endpoint[0]
	}
	c.Providers[provider] = p
	if c.CurrentProvider == "" {
		c.CurrentProvider = provider
	}
	return c.Save()
}

func (c *Config) UseProvider(provider string) error {
	if _, exists := c.Providers[provider]; !exists {
		return fmt.Errorf("provider %s not configured", provider)
	}
	c.CurrentProvider = provider
	return c.Save()
}

func (c *Config) GetAPIKey(provider string) (string, error) {
	if p, exists := c.Providers[provider]; exists {
		return p.APIKey, nil
	}
	return "", fmt.Errorf("no API key found for provider %s", provider)
}

func (c *Config) ListProviders() []string {
	providers := make([]string, 0, len(c.Providers))
	for k, p := range c.Providers {
		entry := fmt.Sprintf("%s(%s)", k, providerModel(k, p))
		if k == c.CurrentProvider {
			entry += " *default"
		}
		providers = append(providers, entry)
	}
	return providers
}

// providerModel returns the model a provider will use: the configured
// endpoint/model if set, otherwise the provider's default model.
func providerModel(provider string, p Provider) string {
	if p.Endpoint != "" {
		return p.Endpoint
	}
	switch provider {
	case ProviderGemini:
		return geminiModel
	case ProviderOpenAI:
		return openaiModel
	case ProviderDeepseek:
		return deepseekModel
	case ProviderQwen:
		return qwenModel
	case ProviderDoubao:
		return doubaoModel
	case ProviderOpenAICompatible:
		return "custom"
	}
	return "unknown"
}

// GetMessageGenerator returns a MessageGenerator instance based on the current provider.
func (c *Config) GetMessageGenerator() (MessageGenerator, error) {
	provider := c.CurrentProvider
	if provider == "" {
		provider = ProviderDoubao
	}

	p, exists := c.Providers[provider]
	if !exists {
		return NewDefauleGenerator(c.Language)
	}

	lang := c.Language
	emoji := c.Emoji
	timeout := c.Timeout

	switch provider {
	case ProviderGemini:
		return NewGeminiGenerator(p.APIKey, lang, emoji, timeout), nil
	case ProviderOpenAI:
		return NewOpenAIGenerator(p.APIKey, lang, emoji, timeout), nil
	case ProviderDoubao:
		return NewDoubaoGenerator(p.APIKey, p.Endpoint, lang, emoji, timeout), nil
	case ProviderDeepseek:
		return NewDeepseekGenerator(p.APIKey, lang, emoji, timeout), nil
	case ProviderQwen:
		return NewQwenGenerator(p.APIKey, lang, emoji, timeout), nil
	case ProviderOpenAICompatible:
		if p.APIKey == "" {
			return nil, fmt.Errorf("API key is required for %s provider", provider)
		}
		if p.BaseURL == "" {
			return nil, fmt.Errorf("base URL is required for %s provider", provider)
		}
		model := p.Endpoint
		if model == "" {
			return nil, fmt.Errorf("model is required for %s provider", provider)
		}
		return NewOpenAICompatibleGenerator(p.APIKey, model, p.BaseURL, lang, emoji, timeout), nil
	default:
		return NewDefauleGenerator(c.Language)
	}
}

func detectLanguage() string {
	lang := os.Getenv("LANG")
	if len(lang) >= 2 && lang[:2] == "zh" {
		return "zh"
	}
	return "en"
}
