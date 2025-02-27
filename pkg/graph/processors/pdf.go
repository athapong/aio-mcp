package processors

import (
	"bytes"
	"context"

	"github.com/athapong/aio-mcp/pkg/graph"
	"github.com/ledongthuc/pdf"
)

type PDFProcessor struct{}

func NewPDFProcessor() *PDFProcessor {
	return &PDFProcessor{}
}

func (p *PDFProcessor) Process(ctx context.Context, content []byte, metadata map[string]interface{}) (*graph.Document, error) {
	reader := bytes.NewReader(content)

	r, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil, err
	}

	var textContent string
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		textContent += text
	}

	// Use NLP processor to process extracted text
	nlpProcessor := NewNLPProcessor()
	return nlpProcessor.Process(ctx, []byte(textContent), metadata)
}

func (p *PDFProcessor) SupportedTypes() []string {
	return []string{"application/pdf"}
}
