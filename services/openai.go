package services

import (
	"os"
	"sync"

	"github.com/sashabaranov/go-openai"
)

var DefaultOpenAIClient = sync.OnceValue(func() *openai.Client {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		panic("OPENAI_API_KEY is not set, please set it in MCP Config")
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	config := openai.DefaultConfig(apiKey)

	if baseURL != "" {
		config.BaseURL = baseURL
	}

	return openai.NewClientWithConfig(config)
})
