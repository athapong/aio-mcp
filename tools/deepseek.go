package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sashabaranov/go-openai"
)

type OllamaRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaResponse struct {
	Message Message `json:"message"`
}

func RegisterDeepseekTool(s *server.MCPServer) {
	reasoningTool := mcp.NewTool("deepseek_reasoning",
		mcp.WithDescription("advanced reasoning engine using Deepseek's AI capabilities for multi-step problem solving, critical analysis, and strategic decision support"),
		mcp.WithString("question", mcp.Required(), mcp.Description("The structured query or problem statement requiring deep analysis and reasoning")),
		mcp.WithString("context", mcp.Required(), mcp.Description("Defines the operational context and purpose of the query within the MCP ecosystem")),
		mcp.WithString("knowledge", mcp.Description("Provides relevant chat history, knowledge base entries, and structured data context for MCP-aware reasoning")),
	)

	s.AddTool(reasoningTool, util.ErrorGuard(deepseekReasoningHandler))
}

func deepseekReasoningHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	systemPrompt, question, _ := buildMessages(arguments)

	// Check if we should use Ollama
	if useOllama := os.Getenv("USE_OLLAMA_DEEPSEEK"); useOllama == "true" {
		ollamaMessages := []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: question,
			},
		}

		ollamaReq := OllamaRequest{
			Model:    "deepseek-r1:1.5b",
			Messages: ollamaMessages,
		}

		return callOllamaDeepseek(ollamaReq)
	}

	// Using Deepseek API
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: question,
		},
	}

	return callDeepseekAPI(messages)
}

func buildMessages(arguments map[string]interface{}) (string, string, string) {
	question, _ := arguments["question"].(string)
	contextArgument, _ := arguments["context"].(string)
	chatContext, _ := arguments["chat_context"].(string)

	systemPrompt := "Context:\n" + contextArgument
	if chatContext != "" {
		systemPrompt += "\n\nAdditional Context:\n" + chatContext
	}

	return systemPrompt, question, chatContext
}

func callDeepseekAPI(messages []openai.ChatCompletionMessage) (*mcp.CallToolResult, error) {
	client := services.DefaultDeepseekClient()
	if client == nil {
		return mcp.NewToolResultError("Deepseek client not properly initialized"), nil
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       "deepseek-reasoner",
			Messages:    messages,
			Temperature: 0.7,
		},
	)

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate content: %s", err)), nil
	}

	if len(resp.Choices) == 0 {
		return mcp.NewToolResultError("no response from Deepseek"), nil
	}

	return mcp.NewToolResultText(resp.Choices[0].Message.Content), nil
}

func callOllamaDeepseek(req OllamaRequest) (*mcp.CallToolResult, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal Ollama request: %s", err)), nil
	}

	ollamaURL := os.Getenv("OLLAMA_URL")
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	resp, err := http.Post(ollamaURL+"/api/chat", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to call Ollama: %s", err)), nil
	}
	defer resp.Body.Close()

	var ollamaResp OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to decode Ollama response: %s", err)), nil
	}

	return mcp.NewToolResultText(ollamaResp.Message.Content), nil
}
