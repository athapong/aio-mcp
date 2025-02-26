package tools

import (
	"fmt"
	"io"

	htmltomarkdownnnn "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterFetchTool(s *server.MCPServer) {
	tool := mcp.NewTool("get_web_content",
		mcp.WithDescription("Fetches content from a given HTTP/HTTPS URL. This tool allows you to retrieve text content from web pages, APIs, or any accessible HTTP endpoints. Returns the raw content as text."),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("The complete HTTP/HTTPS URL to fetch content from (e.g., https://example.com)"),
		),
	)

	s.AddTool(tool, util.ErrorGuard(fetchHandler))
}

func fetchHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	url, ok := arguments["url"].(string)
	if !ok {
		return mcp.NewToolResultError("url must be a string"), nil
	}

	resp, err := services.DefaultHttpClient().Get(url)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to fetch URL: %s", err)), nil
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read response body: %s", err)), nil
	}

	// Convert HTML content to Markdown
	mdContent, err := htmltomarkdownnnn.ConvertString(string(body))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to convert HTML to Markdown: %v", err)), nil
	}

	return mcp.NewToolResultText(mdContent), nil
}
