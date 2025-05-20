package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/athapong/aio-mcp/prompts"
	"github.com/athapong/aio-mcp/resources"
	"github.com/athapong/aio-mcp/tools"
	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	envFile := flag.String("env", ".env", "Path to environment file")
	enableSSE := flag.Bool("sse", false, "Enable SSE server")
	sseAddr := flag.String("sse-addr", ":8080", "Address for SSE server to listen on")
	sseBasePath := flag.String("sse-base-path", "/mcp", "Base path for SSE endpoints")
	flag.Parse()

	if err := godotenv.Load(*envFile); err != nil {
		log.Printf("Warning: Error loading env file %s: %v\n", *envFile, err)
	}
	// Create MCP server
	mcpServer := server.NewMCPServer(
		"aio-mcp",
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

	// Check if SSE server should be enabled
	if *enableSSE || os.Getenv("ENABLE_SSE") == "true" {
		// Create SSE server
		sseServer := server.NewSSEServer(
			mcpServer,
			server.WithBasePath(*sseBasePath),
			server.WithKeepAlive(true),
		)

		// Start SSE server in a goroutine
		go func() {
			log.Printf("Starting SSE server on %s with base path %s", *sseAddr, *sseBasePath)
			if err := sseServer.Start(*sseAddr); err != nil {
				log.Fatalf("Failed to start SSE server: %v", err)
			}
		}()

		// Set up signal handling for graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Wait for termination signal
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)

		// Gracefully shutdown the SSE server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := sseServer.Shutdown(ctx); err != nil {
			log.Printf("Error during SSE server shutdown: %v", err)
		}
		log.Println("SSE server shutdown complete")
	} else {
		// Use stdio server as before
		if err := server.ServeStdio(mcpServer); err != nil {
			panic(fmt.Sprintf("Server error: %v", err))
		}
	}
}
