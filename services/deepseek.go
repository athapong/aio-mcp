package services

import (
	"os"
	"sync"

	"github.com/sashabaranov/go-openai"
)

var (
	deepseekClient *openai.Client
	deepseekOnce   sync.Once
)

// DefaultDeepseekClient returns a singleton instance of the Deepseek OpenAI client
func DefaultDeepseekClient() *openai.Client {
	deepseekOnce.Do(func() {
		useOllama := os.Getenv("USE_OLLAMA_DEEPSEEK") == "true"
		useOpenRouter := os.Getenv("USE_OPENROUTER") == "true"

		if useOllama {
			config := openai.DefaultConfig("not-needed")
			config.BaseURL = "http://localhost:11434/v1"
			deepseekClient = openai.NewClientWithConfig(config)
			return
		}

		if useOpenRouter {
			apiKey := os.Getenv("OPENROUTER_API_KEY")
			if apiKey == "" {
				panic("OPENROUTER_API_KEY environment variable is not set")
			}

			config := openai.DefaultConfig(apiKey)
			config.BaseURL = "https://openrouter.ai/api/v1"
			config.OrgID = "openrouter"
			deepseekClient = openai.NewClientWithConfig(config)
			return
		}

		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			panic("DEEPSEEK_API_KEY environment variable is not set")
		}

		baseURL := os.Getenv("DEEPSEEK_API_BASE")
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}

		config := openai.DefaultConfig(apiKey)
		config.BaseURL = baseURL

		deepseekClient = openai.NewClientWithConfig(config)
	})
	return deepseekClient
}
