package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"google.golang.org/api/chat/v1"
)

func RegisterGChatTool(s *server.MCPServer) {
	// List spaces tool
	listSpacesTool := mcp.NewTool("gchat_list_spaces",
		mcp.WithDescription("List all available Google Chat spaces/rooms"),
	)

	// Send message tool
	sendMessageTool := mcp.NewTool("gchat_send_message",
		mcp.WithDescription("Send a message to a Google Chat space or direct message"),
		mcp.WithString("space_name", mcp.Required(), mcp.Description("Name of the space to send the message to")),
		mcp.WithString("message", mcp.Required(), mcp.Description("Text message to send")),
	)

	s.AddTool(listSpacesTool, util.ErrorGuard(gChatListSpacesHandler))
	s.AddTool(sendMessageTool, util.ErrorGuard(gChatSendMessageHandler))
}

func gChatListSpacesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// No arguments needed for this handler
	spaces, err := services.DefaultGChatService().Spaces.List().Do()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list spaces: %v", err)), nil
	}

	result := make([]map[string]interface{}, 0)
	for _, space := range spaces.Spaces {
		spaceInfo := map[string]interface{}{
			"name":        space.Name,
			"displayName": space.DisplayName,
			"type":        space.Type,
		}
		result = append(result, spaceInfo)
	}

	jsonResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal spaces: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonResult)), nil
}

func gChatSendMessageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	spaceName := arguments["space_name"].(string)
	message := arguments["message"].(string)

	msg := &chat.Message{
		Text: message,
	}

	resp, err := services.DefaultGChatService().Spaces.Messages.Create(spaceName, msg).Do()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to send message: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Message sent successfully. Message ID: %s", resp.Name)), nil
}
