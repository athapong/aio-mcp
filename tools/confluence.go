package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/athapong/aio-mcp/pkg/adf"
	"github.com/athapong/aio-mcp/services"
	"github.com/athapong/aio-mcp/util"
	"github.com/ctreminiom/go-atlassian/pkg/infra/models"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// registerConfluenceTool is a function that registers the confluence tools to the server
func RegisterConfluenceTool(s *server.MCPServer) {
	tool := mcp.NewTool("confluence_search",
		mcp.WithDescription("Search Confluence"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Atlassian Confluence Query Language (CQL)")),
	)

	s.AddTool(tool, confluenceSearchHandler)

	// Add new tool for getting page content
	pageTool := mcp.NewTool("confluence_get_page",
		mcp.WithDescription("Get Confluence page content"),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Confluence page ID")),
	)
	s.AddTool(pageTool, util.ErrorGuard(confluencePageHandler))

	// Add new tool for creating Confluence pages
	createPageTool := mcp.NewTool("confluence_create_page",
		mcp.WithDescription("Create a new Confluence page"),
		mcp.WithString("space_key", mcp.Required(), mcp.Description("The key of the space where the page will be created")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Title of the page")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content of the page in storage format (XHTML)")),
		mcp.WithString("parent_id", mcp.Description("ID of the parent page (optional)")),
	)
	s.AddTool(createPageTool, util.ErrorGuard(confluenceCreatePageHandler))

	// Add new tool for updating Confluence pages
	updatePageTool := mcp.NewTool("confluence_update_page",
		mcp.WithDescription("Update an existing Confluence page"),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("ID of the page to update")),
		mcp.WithString("title", mcp.Description("New title of the page (optional)")),
		mcp.WithString("content", mcp.Description("New content of the page in storage format (XHTML)")),
		mcp.WithString("version_number", mcp.Description("Version number for optimistic locking (optional)")),
	)
	s.AddTool(updatePageTool, util.ErrorGuard(confluenceUpdatePageHandler))

	// Add new tool for comparing page versions
	compareTool := mcp.NewTool("confluence_compare_versions",
		mcp.WithDescription("Compare two versions of a Confluence page"),
		mcp.WithString("page_id", mcp.Required(), mcp.Description("Confluence page ID")),
		mcp.WithString("source_version", mcp.Required(), mcp.Description("Source version number")),
		mcp.WithString("target_version", mcp.Required(), mcp.Description("Target version number")),
	)
	s.AddTool(compareTool, util.ErrorGuard(confluenceCompareHandler))
}

// confluenceSearchHandler is a handler for the confluence search tool
func confluenceSearchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	client := services.ConfluenceClient()

	query, ok := arguments["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query argument is required")
	}

	// Use the provided context
	options := &models.PageOptionsScheme{
		PageIDs:    nil,
		SpaceIDs:   nil,
		Sort:       "created-date",
		Status:     []string{"current"},
		Title:      query, // Use query as title search
		BodyFormat: "atlas_doc_format",
	}

	var results strings.Builder
	var cursor string

	for {
		chunk, response, err := client.Page.Gets(ctx, options, cursor, 20)
		if err != nil {
			if response != nil {
				return nil, fmt.Errorf("search failed with status %d: %v", response.Code, err)
			}
			return nil, fmt.Errorf("search failed: %v", err)
		}

		// Process results
		for _, page := range chunk.Results {
			results.WriteString(fmt.Sprintf(`
Title: %s
ID: %s
Status: %s
SpaceId: %s
----------------------------------------
`,
				page.Title,
				page.ID,
				page.Status,
				page.SpaceID,
			))
		}

		// Check if there are more pages
		if chunk.Links == nil || chunk.Links.Next == "" {
			break
		}

		// Parse next cursor from URL
		values, err := url.ParseQuery(chunk.Links.Next)
		if err != nil {
			return nil, fmt.Errorf("failed to parse next page URL: %v", err)
		}

		if _, hasCursor := values["cursor"]; hasCursor {
			cursor = values["cursor"][0]
		} else {
			break
		}
	}

	if results.Len() == 0 {
		results.WriteString("No results found")
	}

	return mcp.NewToolResultText(results.String()), nil
}

func confluencePageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	client := services.ConfluenceClient()

	pageID, ok := arguments["page_id"].(string)
	if !ok {
		return nil, fmt.Errorf("page_id argument is required")
	}

	// Convert pageID to int
	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return nil, fmt.Errorf("invalid page ID: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	// Use new Page.Get method with atlas_doc_format, setting version to 0 to get latest
	// Setting draft=true to get only published content, version=-1 to get the latest version
	page, response, err := client.Page.Get(ctxWithTimeout, pageIDInt, "atlas_doc_format", false, -1)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("failed to get page: %s (endpoint: %s)", response.Bytes.String(), response.Endpoint)
		}
		return nil, fmt.Errorf("failed to get page: %v", err)
	}

	if page == nil {
		return nil, fmt.Errorf("no content returned for page ID: %s", pageID)
	}

	// Build response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Title: %s\n", page.Title))
	result.WriteString(fmt.Sprintf("ID: %s\n", page.ID))
	result.WriteString(fmt.Sprintf("Space ID: %s\n", page.SpaceID))
	result.WriteString(fmt.Sprintf("Status: %s\n", page.Status))

	if page.Version != nil {
		result.WriteString(fmt.Sprintf("Version: %d (Created: %s)\n",
			page.Version.Number,
			page.Version.CreatedAt,
		))
	}

	// Parse Atlas Doc Format content
	var contentValue string
	if page.Body != nil && page.Body.AtlasDocFormat != nil {
		adfBody := &models.CommentNodeScheme{}
		if err := json.Unmarshal([]byte(page.Body.AtlasDocFormat.Value), adfBody); err != nil {
			return nil, fmt.Errorf("failed to parse ADF content: %v", err)
		}
		contentValue = convertADFToMarkdown(adfBody)
	}

	result.WriteString("\nContent:\n")
	result.WriteString("----------------------------------------\n")
	result.WriteString(contentValue)
	result.WriteString("\n----------------------------------------\n")

	return mcp.NewToolResultText(result.String()), nil
}

// Helper function to convert ADF to markdown using our local implementation
func convertADFToMarkdown(node *models.CommentNodeScheme) string {
	if node == nil {
		return ""
	}

	// Convert CommentNodeScheme to our ADF Node
	adfNode := convertToADFNode(node)
	if adfNode == nil {
		return ""
	}

	// Convert to markdown string
	return adf.Convert(adfNode)
}

// Helper function to convert CommentNodeScheme to our ADF Node
func convertToADFNode(node *models.CommentNodeScheme) *adf.Node {
	if node == nil {
		return nil
	}

	adfNode := &adf.Node{
		Type:    node.Type,
		Text:    node.Text,
		Attrs:   make(map[string]interface{}),
		Marks:   make([]*adf.Mark, 0),
		Content: make([]*adf.Node, 0),
	}

	// Copy attributes
	for k, v := range node.Attrs {
		adfNode.Attrs[k] = v
	}

	// Convert marks
	if node.Marks != nil {
		for _, mark := range node.Marks {
			adfMark := &adf.Mark{
				Type:  mark.Type,
				Attrs: make(map[string]interface{}),
			}
			for k, v := range mark.Attrs {
				adfMark.Attrs[k] = v
			}
			adfNode.Marks = append(adfNode.Marks, adfMark)
		}
	}

	// Convert child nodes recursively
	if node.Content != nil {
		for _, child := range node.Content {
			if childNode := convertToADFNode(child); childNode != nil {
				adfNode.Content = append(adfNode.Content, childNode)
			}
		}
	}

	return adfNode
}

// confluenceCreatePageHandler handles the creation of new Confluence pages
func confluenceCreatePageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	client := services.ConfluenceClient()

	// Extract required arguments
	spaceKey, ok := arguments["space_key"].(string)
	if !ok {
		return nil, fmt.Errorf("space_key argument is required")
	}

	title, ok := arguments["title"].(string)
	if !ok {
		return nil, fmt.Errorf("title argument is required")
	}

	content, ok := arguments["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content argument is required")
	}

	// Create the ADF body
	body := models.CommentNodeScheme{}
	body.Version = 1
	body.Type = "doc"

	// Convert the content into a paragraph node
	body.AppendNode(&models.CommentNodeScheme{
		Type: "paragraph",
		Content: []*models.CommentNodeScheme{
			{
				Type: "text",
				Text: content,
			},
		},
	})

	// Convert ADF body to JSON string
	bodyValue, err := json.Marshal(&body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ADF body: %v", err)
	}

	// Create page payload using v2 models
	payload := &models.PageCreatePayloadScheme{
		SpaceID: spaceKey, // Note: You might need to convert spaceKey to int
		Status:  "current",
		Title:   title,
		Body: &models.PageBodyRepresentationScheme{
			Representation: "atlas_doc_format",
			Value:          string(bodyValue),
		},
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	// Create the page with v2 API
	page, response, err := client.Page.Create(ctxWithTimeout, payload)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("failed to create page: %s (endpoint: %s)", response.Bytes.String(), response.Endpoint)
		}
		return nil, fmt.Errorf("failed to create page: %v", err)
	}

	result := fmt.Sprintf("Page created successfully!\nTitle: %s\nID: %s\nStatus: %s\nVersion: %d",
		page.Title,
		page.ID,
		page.Status,
		page.Version.Number,
	)

	return mcp.NewToolResultText(result), nil
}

// confluenceUpdatePageHandler handles updating existing Confluence pages
func confluenceUpdatePageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	client := services.ConfluenceClient()

	// Extract required arguments
	pageID, ok := arguments["page_id"].(string)
	if !ok {
		return nil, fmt.Errorf("page_id argument is required")
	}

	// Convert pageID to int
	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return nil, fmt.Errorf("invalid page ID: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	// Get current page to get its current content and version
	page, response, err := client.Page.Get(ctxWithTimeout, pageIDInt, "atlas_doc_format", false, 0)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("failed to get current page: %s (endpoint: %s)", response.Bytes.String(), response.Endpoint)
		}
		return nil, fmt.Errorf("failed to get current page: %v", err)
	}

	// Parse existing content as ADF
	adfBody := &models.CommentNodeScheme{}
	if err := json.Unmarshal([]byte(page.Body.AtlasDocFormat.Value), adfBody); err != nil {
		return nil, fmt.Errorf("failed to parse existing content: %v", err)
	}

	// Handle content update
	if content, ok := arguments["content"].(string); ok && content != "" {
		// Create new content node
		contentNode := &models.CommentNodeScheme{
			Type: "paragraph",
			Content: []*models.CommentNodeScheme{
				{
					Type: "text",
					Text: content,
				},
			},
		}

		// Append new content to existing body
		adfBody.AppendNode(contentNode)
	}

	// Convert updated ADF body back to JSON
	bodyValue, err := json.Marshal(adfBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal updated content: %v", err)
	}

	// Create update payload
	payload := &models.PageUpdatePayloadScheme{
		ID: pageIDInt,
		SpaceID: func() int {
			spaceIDInt, err := strconv.Atoi(page.SpaceID)
			if err != nil {
				return 0 // or handle the error appropriately
			}
			return spaceIDInt
		}(),
		Status: "current",
		Title:  page.Title, // Keep existing title by default
		Body: &models.PageBodyRepresentationScheme{
			Representation: "atlas_doc_format",
			Value:          string(bodyValue),
		},
		Version: &models.PageUpdatePayloadVersionScheme{
			Number:  page.Version.Number + 1,
			Message: fmt.Sprintf("Updated to version %d", page.Version.Number+1),
		},
	}

	// Handle optional title update
	if title, ok := arguments["title"].(string); ok && title != "" {
		payload.Title = title
	}

	// Handle version number override
	if versionStr, ok := arguments["version_number"].(string); ok && versionStr != "" {
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return nil, fmt.Errorf("invalid version_number: %v", err)
		}
		payload.Version.Number = version
	}

	// Update the page
	updatedPage, response, err := client.Page.Update(ctx, pageIDInt, payload)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("failed to update page: %s (endpoint: %s)", response.Bytes.String(), response.Endpoint)
		}
		return nil, fmt.Errorf("failed to update page: %v", err)
	}

	result := fmt.Sprintf("Page updated successfully!\nTitle: %s\nID: %s\nStatus: %s\nVersion: %d",
		updatedPage.Title,
		updatedPage.ID,
		updatedPage.Status,
		updatedPage.Version.Number,
	)

	return mcp.NewToolResultText(result), nil
}

// Add this new handler function
func confluenceCompareHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	client := services.ConfluenceClient()
	if client == nil {
		return nil, fmt.Errorf("failed to get Confluence client")
	}

	// Extract page ID
	pageID, ok := arguments["page_id"].(string)
	if !ok || pageID == "" {
		return nil, fmt.Errorf("valid page_id argument is required")
	}

	pageIDInt, err := strconv.Atoi(pageID)
	if err != nil {
		return nil, fmt.Errorf("invalid page ID: %v", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Get the latest version first
	latestPage, response, err := client.Page.Get(ctxWithTimeout, pageIDInt, "atlas_doc_format", false, -1)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("failed to get latest version: %s (endpoint: %s)", response.Bytes.String(), response.Endpoint)
		}
		return nil, fmt.Errorf("failed to get latest version: %v", err)
	}

	if latestPage == nil || latestPage.Version == nil {
		return nil, fmt.Errorf("failed to get page version information")
	}

	// Determine versions to compare
	targetNum := latestPage.Version.Number
	sourceNum := targetNum - 1 // Default to previous version

	// Override with specific versions if provided
	if sourceVersion, ok := arguments["source_version"].(string); ok && sourceVersion != "" {
		if num, err := strconv.Atoi(sourceVersion); err == nil && num > 0 {
			sourceNum = num
		}
	}
	if targetVersion, ok := arguments["target_version"].(string); ok && targetVersion != "" {
		if num, err := strconv.Atoi(targetVersion); err == nil && num > 0 {
			targetNum = num
		}
	}

	// Validate version numbers
	if sourceNum <= 0 || targetNum <= 0 || sourceNum >= targetNum {
		return nil, fmt.Errorf("invalid version numbers: source=%d, target=%d", sourceNum, targetNum)
	}

	// Fetch source version
	sourceContent, sourceResp, err := client.Page.Get(ctx, pageIDInt, "atlas_doc_format", false, sourceNum)
	if err != nil {
		if sourceResp != nil {
			return nil, fmt.Errorf("failed to get source version: %s (endpoint: %s)", sourceResp.Bytes.String(), sourceResp.Endpoint)
		}
		return nil, fmt.Errorf("failed to get source version: %v", err)
	}

	// Convert source content to markdown
	sourceMarkdown := convertPageToMarkdown(sourceContent)

	// Convert target content to markdown
	targetMarkdown := convertPageToMarkdown(latestPage)

	// Perform semantic diff
	diffs := performSemanticDiff(sourceMarkdown, targetMarkdown)

	// Build comparison result
	var comparison strings.Builder
	comparison.WriteString(fmt.Sprintf("Comparing Page: %s (ID: %d)\n", latestPage.Title, pageIDInt))
	comparison.WriteString(fmt.Sprintf("Comparing versions: %d → %d\n\n", sourceNum, targetNum))

	// Compare titles
	if sourceContent.Title != latestPage.Title {
		comparison.WriteString("Title Changes:\n")
		comparison.WriteString(fmt.Sprintf("- Version %d: %s\n", sourceNum, sourceContent.Title))
		comparison.WriteString(fmt.Sprintf("+ Version %d: %s\n\n", targetNum, latestPage.Title))
	} else {
		comparison.WriteString(fmt.Sprintf("Title: %s (unchanged)\n\n", sourceContent.Title))
	}

	// Add metadata
	comparison.WriteString("Version Information:\n")
	comparison.WriteString(fmt.Sprintf("Source (v%d): Created %s\n",
		sourceContent.Version.Number,
		sourceContent.Version.CreatedAt))
	comparison.WriteString(fmt.Sprintf("Target (v%d): Created %s\n\n",
		latestPage.Version.Number,
		latestPage.Version.CreatedAt))

	// Add diff results
	comparison.WriteString("Content Changes:\n")
	comparison.WriteString("=================\n")
	comparison.WriteString(diffs)

	return mcp.NewToolResultText(comparison.String()), nil
}

// Helper function to convert a page to markdown
func convertPageToMarkdown(page *models.PageScheme) string {
	if page == nil || page.Body == nil || page.Body.AtlasDocFormat == nil {
		return ""
	}

	adfBody := &models.CommentNodeScheme{}
	if err := json.Unmarshal([]byte(page.Body.AtlasDocFormat.Value), adfBody); err != nil {
		return ""
	}

	return convertADFToMarkdown(adfBody)
}

// Helper function to perform semantic diff
func performSemanticDiff(source, target string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(source, target, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var result strings.Builder
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffDelete:
			result.WriteString("- " + strings.ReplaceAll(diff.Text, "\n", "\n- ") + "\n")
		case diffmatchpatch.DiffInsert:
			result.WriteString("+ " + strings.ReplaceAll(diff.Text, "\n", "\n+ ") + "\n")
		case diffmatchpatch.DiffEqual:
			result.WriteString("  " + strings.ReplaceAll(diff.Text, "\n", "\n  ") + "\n")
		}
	}

	return result.String()
}

// Update extractTextFromADF to use flowline
func extractTextFromADF(node *models.CommentNodeScheme) string {
	if node == nil {
		return ""
	}

	// Convert to markdown first using flowline
	markdown := convertADFToMarkdown(node)

	// Remove Markdown syntax to get plain text
	// This is a simple approach - you might want to enhance it based on your needs
	text := markdown
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "#", "")
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	text = strings.ReplaceAll(text, "`", "")
	text = strings.ReplaceAll(text, ">", "")
	text = strings.TrimSpace(text)

	return text
}
