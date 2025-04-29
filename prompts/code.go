package prompts

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterCodeTools(s *server.MCPServer) {
	tool := mcp.NewPrompt("code_review",
		mcp.WithPromptDescription("Review code and provide feedback"),
		mcp.WithArgument("developer_name", mcp.ArgumentDescription("The name of the developer who wrote the code")),
	)
	s.AddPrompt(tool, codeReviewHandler)
}

func codeReviewHandler(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	developerName := request.Params.Arguments["developer_name"]

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Code reviewed by %s", developerName),
		Messages: []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Use gitlab tools to review code written by %s; convert name to username if needed", developerName),
				},
			},
		},
	}, nil
}
