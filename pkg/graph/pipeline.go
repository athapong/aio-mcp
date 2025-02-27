package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

var (
	pipelineProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "pipeline_processing_duration_seconds",
			Help: "Time spent processing documents in pipeline",
		},
		[]string{"status"},
	)

	documentProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pipeline_documents_processed_total",
			Help: "Total number of documents processed",
		},
		[]string{"status"},
	)
)

func init() {
	prometheus.MustRegister(pipelineProcessingDuration)
	prometheus.MustRegister(documentProcessedTotal)
}

// TextPipeline implements a document processing pipeline
type TextPipeline struct {
	processors []DocumentProcessor
	mutex      sync.RWMutex
	logger     *logrus.Logger
	batchSize  int
}

// NewPipeline creates a new text processing pipeline
func NewPipeline() *TextPipeline {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &TextPipeline{
		processors: make([]DocumentProcessor, 0),
		batchSize:  10,
		logger:     logger,
	}
}

// AddProcessor adds a new processor to the pipeline
func (p *TextPipeline) AddProcessor(processor DocumentProcessor) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.processors = append(p.processors, processor)
}

// BatchProcess processes multiple documents concurrently
func (p *TextPipeline) BatchProcess(ctx context.Context, docs []*Document) error {
	p.logger.WithField("document_count", len(docs)).Info("Starting batch processing")

	// Process in batches
	for i := 0; i < len(docs); i += p.batchSize {
		end := i + p.batchSize
		if end > len(docs) {
			end = len(docs)
		}

		batch := docs[i:end]
		errors := make(chan error, len(batch))
		var wg sync.WaitGroup

		// Process batch concurrently
		for _, doc := range batch {
			wg.Add(1)
			go func(d *Document) {
				defer wg.Done()

				timer := prometheus.NewTimer(pipelineProcessingDuration.WithLabelValues("processing"))
				err := p.Process(ctx, d)
				timer.ObserveDuration()

				if err != nil {
					p.logger.WithError(err).WithField("doc_id", d.ID).Error("Failed to process document")
					documentProcessedTotal.WithLabelValues("error").Inc()
					errors <- err
					return
				}

				documentProcessedTotal.WithLabelValues("success").Inc()
			}(doc)
		}

		// Wait for batch to complete
		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			if err != nil {
				return fmt.Errorf("batch processing failed: %v", err)
			}
		}
	}

	p.logger.Info("Batch processing completed successfully")
	return nil
}

// Process runs the document through all processors in the pipeline
func (p *TextPipeline) Process(ctx context.Context, doc *Document) error {
	if doc == nil {
		return fmt.Errorf("cannot process nil document")
	}

	p.logger.WithField("doc_id", doc.ID).Info("Processing document")

	p.mutex.RLock()
	processorCount := len(p.processors)
	p.mutex.RUnlock()

	if processorCount == 0 {
		return fmt.Errorf("no processors configured in pipeline")
	}

	// Single document processing
	timer := prometheus.NewTimer(pipelineProcessingDuration.WithLabelValues("single"))
	defer timer.ObserveDuration()

	var processedDoc *Document
	var err error

	// Apply each processor sequentially
	for i, processor := range p.processors {
		// For the first processor, use the input document
		if i == 0 {
			processedDoc, err = processor.Process(ctx, []byte(doc.Content), doc.Metadata)
		} else {
			// For subsequent processors, use the output of the previous processor
			processedDoc, err = processor.Process(ctx, []byte(processedDoc.Content), processedDoc.Metadata)
		}

		if err != nil {
			return fmt.Errorf("processor %d failed: %v", i, err)
		}
	}

	// Update the original document with processed content
	*doc = *processedDoc

	p.logger.WithField("doc_id", doc.ID).Info("Document processing completed")
	return nil
}
