package adf

import (
	"fmt"
	"strings"
)

// Convert converts an ADF node to Markdown
func Convert(node *Node) string {
	if node == nil {
		return ""
	}

	var result strings.Builder
	convertNode(node, &result, 0)
	return result.String()
}

func convertNode(node *Node, result *strings.Builder, depth int) {
	switch node.Type {
	case "doc":
		convertDoc(node, result, depth)
	case "paragraph":
		convertParagraph(node, result, depth)
	case "heading":
		convertHeading(node, result, depth)
	case "text":
		convertText(node, result)
	case "hardBreak":
		result.WriteString("\n")
	case "bulletList":
		convertBulletList(node, result, depth)
	case "orderedList":
		convertOrderedList(node, result, depth)
	case "listItem":
		convertListItem(node, result, depth)
	case "codeBlock":
		convertCodeBlock(node, result)
	case "blockquote":
		convertBlockquote(node, result, depth)
	case "rule":
		result.WriteString("---\n")
	case "table":
		convertTable(node, result)
	default:
		convertChildren(node, result, depth)
	}
}

func convertDoc(node *Node, result *strings.Builder, depth int) {
	convertChildren(node, result, depth)
}

func convertParagraph(node *Node, result *strings.Builder, depth int) {
	if depth > 0 {
		result.WriteString(strings.Repeat("  ", depth))
	}
	convertChildren(node, result, depth)
	result.WriteString("\n\n")
}

func convertHeading(node *Node, result *strings.Builder, depth int) {
	level := 1
	if l, ok := node.Attrs["level"].(float64); ok {
		level = int(l)
	}
	result.WriteString(strings.Repeat("#", level) + " ")
	convertChildren(node, result, depth)
	result.WriteString("\n\n")
}

func convertText(node *Node, result *strings.Builder) {
	text := node.Text
	if node.Marks != nil {
		for _, mark := range node.Marks {
			switch mark.Type {
			case "strong":
				text = "**" + text + "**"
			case "em":
				text = "_" + text + "_"
			case "code":
				text = "`" + text + "`"
			case "strike":
				text = "~~" + text + "~~"
			case "link":
				if href, ok := mark.Attrs["href"].(string); ok {
					text = fmt.Sprintf("[%s](%s)", text, href)
				}
			}
		}
	}
	result.WriteString(text)
}

func convertBulletList(node *Node, result *strings.Builder, depth int) {
	for _, child := range node.Content {
		result.WriteString(strings.Repeat("  ", depth) + "* ")
		convertChildren(child, result, depth+1)
		result.WriteString("\n")
	}
	result.WriteString("\n")
}

func convertOrderedList(node *Node, result *strings.Builder, depth int) {
	for i, child := range node.Content {
		result.WriteString(fmt.Sprintf("%s%d. ", strings.Repeat("  ", depth), i+1))
		convertChildren(child, result, depth+1)
		result.WriteString("\n")
	}
	result.WriteString("\n")
}

func convertListItem(node *Node, result *strings.Builder, depth int) {
	convertChildren(node, result, depth)
}

func convertCodeBlock(node *Node, result *strings.Builder) {
	language := ""
	if lang, ok := node.Attrs["language"].(string); ok {
		language = lang
	}
	result.WriteString("```" + language + "\n")
	convertChildren(node, result, 0)
	result.WriteString("```\n\n")
}

func convertBlockquote(node *Node, result *strings.Builder, depth int) {
	for _, child := range node.Content {
		result.WriteString("> ")
		convertChildren(child, result, depth+1)
	}
	result.WriteString("\n")
}

func convertTable(node *Node, result *strings.Builder) {
	if len(node.Content) == 0 {
		return
	}

	// Extract headers and calculate column widths
	columnWidths := make([]int, 0)
	rows := make([][]string, 0)

	// Process header row
	if len(node.Content) > 0 && len(node.Content[0].Content) > 0 {
		headerRow := make([]string, 0)
		for _, cell := range node.Content[0].Content {
			var cellContent strings.Builder
			convertChildren(cell, &cellContent, 0)
			content := strings.TrimSpace(cellContent.String())
			headerRow = append(headerRow, content)
			columnWidths = append(columnWidths, len(content))
		}
		rows = append(rows, headerRow)
	}

	// Process data rows and update column widths
	for i := 1; i < len(node.Content); i++ {
		row := make([]string, 0)
		for j, cell := range node.Content[i].Content {
			var cellContent strings.Builder
			convertChildren(cell, &cellContent, 0)
			content := strings.TrimSpace(cellContent.String())
			row = append(row, content)
			if j < len(columnWidths) && len(content) > columnWidths[j] {
				columnWidths[j] = len(content)
			}
		}
		rows = append(rows, row)
	}

	// Write table
	for i, row := range rows {
		result.WriteString("|")
		for j, cell := range row {
			if j < len(columnWidths) {
				padding := columnWidths[j] - len(cell)
				result.WriteString(" " + cell + strings.Repeat(" ", padding) + " |")
			}
		}
		result.WriteString("\n")

		// Write separator after header
		if i == 0 {
			result.WriteString("|")
			for _, width := range columnWidths {
				result.WriteString(strings.Repeat("-", width+2) + "|")
			}
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")
}

func convertChildren(node *Node, result *strings.Builder, depth int) {
	if node.Content != nil {
		for _, child := range node.Content {
			convertNode(child, result, depth)
		}
	}
}
