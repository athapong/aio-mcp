package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sashabaranov/go-openai"
)

func RegisterToolManagerTool(s *server.MCPServer, envFile string) {
	tool := mcp.NewTool("tool_manager",
		mcp.WithDescription("Manage MCP tools - enable or disable tools"),
		mcp.WithString("action", mcp.Required(), mcp.Description("Action to perform: list, enable, disable")),
		mcp.WithString("tool_name", mcp.Description("Tool name to enable/disable")),
	)

	s.AddTool(tool, util.ErrorGuard(util.AdaptLegacyHandler(toolManagerHandler)))

	planTool := mcp.NewTool("tool_use_plan",
		mcp.WithDescription("Create a plan using available tools to solve the request"),
		mcp.WithString("request", mcp.Required(), mcp.Description("Request to plan for")),
		mcp.WithString("context", mcp.Required(), mcp.Description("Context related to the request")),
	)
	s.AddTool(planTool, util.ErrorGuard(util.AdaptLegacyHandler(toolUsePlanHandler)))
}

func toolManagerHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	action, ok := arguments["action"].(string)
	if !ok {
		return mcp.NewToolResultError("action must be a string"), nil
	}

	enableTools := os.Getenv("ENABLE_TOOLS")
	toolList := strings.Split(enableTools, ",")

	switch action {
	case "list":
		response := "Available tools:\n"
		allEnabled := enableTools == ""

		// List all available tools with status
		tools := []struct {
			name string
			desc string
		}{
			{"tool_manager", "Tool management"},
			{"gemini", "AI tools: web search"},
			{"fetch", "Web content fetching"},
			{"confluence", "Confluence integration"},
			{"youtube", "YouTube transcript"},
			{"jira", "Jira issue management"},
			{"gitlab", "GitLab integration"},
			{"script", "Script execution"},
			{"rag", "RAG memory tools"},
			{"gmail", "Gmail tools"},
			{"calendar", "Google Calendar tools"},
			{"youtube_channel", "YouTube channel tools"},
			{"sequential_thinking", "Sequential thinking tool"},
			{"deepseek", "Deepseek reasoning tool"},
			{"maps_location_search", "Google Maps location search"},
			{"maps_geocoding", "Google Maps geocoding and reverse geocoding"},
			{"maps_place_details", "Google Maps detailed place information"},
		}

		for _, t := range tools {
			status := "disabled"
			if allEnabled || contains(toolList, t.name) {
				status = "enabled"
			}
			response += fmt.Sprintf("- %s (%s) [%s]\n", t.name, t.desc, status)
		}
		response += "\n"

		// List enabled tools
		response += "Currently enabled tools:\n"
		if allEnabled {
			response += "All tools are enabled (ENABLE_TOOLS is empty)\n"
		} else {
			for _, tool := range toolList {
				if tool != "" {
					response += fmt.Sprintf("- %s\n", tool)
				}
			}
		}
		return mcp.NewToolResultText(response), nil

	case "enable", "disable":
		toolName, ok := arguments["tool_name"].(string)
		if !ok || toolName == "" {
			return mcp.NewToolResultError("tool_name is required for enable/disable actions"), nil
		}

		if enableTools == "" {
			toolList = []string{}
		}

		if action == "enable" {
			if !contains(toolList, toolName) {
				toolList = append(toolList, toolName)
			}
		} else {
			toolList = removeString(toolList, toolName)
		}

		newEnableTools := strings.Join(toolList, ",")
		os.Setenv("ENABLE_TOOLS", newEnableTools)

		return mcp.NewToolResultText(fmt.Sprintf("Successfully %sd tool: %s", action, toolName)), nil

	default:
		return mcp.NewToolResultError("Invalid action. Use 'list', 'enable', or 'disable'"), nil
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	result := []string{}
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}

func toolUsePlanHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	request, _ := arguments["request"].(string)
	contextString, _ := arguments["context"].(string)

	enabledTools := strings.Split(os.Getenv("ENABLE_TOOLS"), ",")
	if !contains(enabledTools, "deepseek") {
		return mcp.NewToolResultError("Deepseek tool must be enabled to generate plans"), nil
	}

	// Check for configuration
	useOllama := os.Getenv("USE_OLLAMA_DEEPSEEK") == "true"
	useOpenRouter := os.Getenv("USE_OPENROUTER") == "true"

	if !useOllama && !useOpenRouter && os.Getenv("DEEPSEEK_API_KEY") == "" {
		return mcp.NewToolResultError("Either USE_OLLAMA_DEEPSEEK, USE_OPENROUTER must be true, or DEEPSEEK_API_KEY must be set"), nil
	}

	systemPrompt := fmt.Sprintf(`You are a tool usage planning assistant. Create a detailed execution plan using the currently enabled tools: %s

Context: %s

Output format:
1. [Tool Name] - Purpose: ... (Expected result: ...)
2. [Tool Name] - Purpose: ... (Expected result: ...)
...`, strings.Join(enabledTools, ", "), contextString)

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: request,
		},
	}

	client := services.DefaultDeepseekClient()
	if client == nil {
		return mcp.NewToolResultError("Failed to initialize client"), nil
	}

	modelName := "deepseek-reasoner"
	if useOllama {
		modelName = "deepseek-r1:8b"
	} else if useOpenRouter {
		modelName = "deepseek/deepseek-r1-distill-qwen-32b" // or any other model available on OpenRouter
	}

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       modelName,
			Messages:    messages,
			Temperature: 0.3,
		},
	)

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API call failed: %v", err)), nil
	}

	if len(resp.Choices) == 0 {
		return mcp.NewToolResultError("No response from Deepseek"), nil
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	return mcp.NewToolResultText("📝 **Execution Plan:**\n" + content), nil
}
