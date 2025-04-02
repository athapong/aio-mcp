package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/athapong/aio-mcp/prompts"
	"github.com/athapong/aio-mcp/resources"
	"github.com/athapong/aio-mcp/tools"
	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	envFile := flag.String("env", ".env", "Path to environment file")
	flag.Parse()

	if err := godotenv.Load(*envFile); err != nil {
		// do nothing
	}

	mcpServer := server.NewMCPServer(
		"All-In-One MCP",
		"1.0.0",
		server.WithLogging(),
		server.WithPromptCapabilities(true),
		server.WithResourceCapabilities(true, true),
	)

	tools.RegisterToolManagerTool(mcpServer)

	enableTools := strings.Split(os.Getenv("ENABLE_TOOLS"), ",")
	allToolsEnabled := len(enableTools) == 1 && enableTools[0] == ""

	isEnabled := func(toolName string) bool {
		return allToolsEnabled || slices.Contains(enableTools, toolName)
	}

	if isEnabled("gemini") {
		tools.RegisterGeminiTool(mcpServer)
	}

	if isEnabled("deepseek") {
		tools.RegisterDeepseekTool(mcpServer)
	}

	if isEnabled("fetch") {
		tools.RegisterFetchTool(mcpServer)
	}

	if isEnabled("brave_search") {
		tools.RegisterWebSearchTool(mcpServer)
	}

	if isEnabled("confluence") {
		tools.RegisterConfluenceTool(mcpServer)
	}

	if isEnabled("youtube") {
		tools.RegisterYouTubeTool(mcpServer)
	}

	if isEnabled("jira") {
		tools.RegisterJiraTool(mcpServer)
		resources.RegisterJiraResource(mcpServer)
	}

	if isEnabled("gitlab") {
		tools.RegisterGitLabTool(mcpServer)
	}

	if isEnabled("script") {
		tools.RegisterScriptTool(mcpServer)
	}

	if isEnabled("rag") {
		tools.RegisterRagTools(mcpServer)
	}

	if isEnabled("gmail") {
		tools.RegisterGmailTools(mcpServer)
	}

	if isEnabled("calendar") {
		tools.RegisterCalendarTools(mcpServer)
	}

	if isEnabled("youtube_channel") {
		tools.RegisterYouTubeChannelTools(mcpServer)
	}

	if isEnabled("sequential_thinking") {
		tools.RegisterSequentialThinkingTool(mcpServer)
		tools.RegisterSequentialThinkingHistoryTool(mcpServer)
	}

	if isEnabled("gchat") {
		tools.RegisterGChatTool(mcpServer)
	}

	tools.RegisterScreenshotTool(mcpServer)

	prompts.RegisterCodeTools(mcpServer)

	if isEnabled("google_maps") {
		tools.RegisterGoogleMapTools(mcpServer)
	}

	if err := server.ServeStdio(mcpServer); err != nil {
		panic(fmt.Sprintf("Server error: %v", err))
	}
}
