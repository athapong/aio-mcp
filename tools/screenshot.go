package tools

import (
	"fmt"
	"image/png"
	"os"
	"time"

	"github.com/athapong/aio-mcp/util"
	"github.com/kbinani/screenshot"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterScreenshotTool registers the screenshot capturing tool with the MCP server
func RegisterScreenshotTool(s *server.MCPServer) {
	tool := mcp.NewTool("capture_screenshot",
		mcp.WithDescription("Capture a screenshot of the entire screen"),
	)
	s.AddTool(tool, util.ErrorGuard(screenshotHandler))
}

func screenshotHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return mcp.NewToolResultError("No active displays found"), nil
	}

	// Capture the screenshot of the first display
	bounds := screenshot.GetDisplayBounds(0)
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to capture screenshot: %v", err)), nil
	}

	// Save the screenshot to a file
	fileName := fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	file, err := os.Create(fileName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create file: %v", err)), nil
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to encode image: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Screenshot saved to %s", fileName)), nil
}
