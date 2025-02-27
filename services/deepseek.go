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

func DefaultDeepseekClient() *openai.Client {
	// If using Ollama, return nil as we don't need the OpenAI client
	if os.Getenv("USE_OLLAMA_DEEPSEEK") == "true" {
		return nil
	}

	deepseekOnce.Do(func() {
		apiKey := os.Getenv("DEEPSEEK_API_KEY")
		if apiKey == "" {
			return
		}

		openai.DefaultConfig(apiKey)
		deepseekClient = openai.NewClient(apiKey)
	})

	return deepseekClient
}
