package processors

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/athapong/aio-mcp/pkg/graph"
)

// HTMLProcessor is responsible for processing HTML content.
type HTMLProcessor struct{}

// NewHTMLProcessor creates a new instance of HTMLProcessor.
func NewHTMLProcessor() *HTMLProcessor {
	return &HTMLProcessor{}
}

// Process parses the HTML content and processes it using an NLP processor.
func (p *HTMLProcessor) Process(ctx context.Context, content []byte, metadata map[string]interface{}) (*graph.Document, error) {
	// Create a new document from the HTML content
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to create document from HTML content: %w", err)
	}

	// Extract and trim text content from the body
	text := strings.TrimSpace(doc.Find("body").Text())

	// Process the extracted text using the NLP processor
	nlpProcessor := NewNLPProcessor()
	return nlpProcessor.Process(ctx, []byte(text), metadata)
}

// SupportedTypes returns the MIME types supported by the HTMLProcessor.
func (p *HTMLProcessor) SupportedTypes() []string {
	return []string{"text/html"}
}
