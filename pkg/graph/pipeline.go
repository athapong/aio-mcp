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

// TextPipeline implements the Pipeline interface
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
	p.logger.WithField("doc_id", doc.ID).Info("Processing document")
	timer := prometheus.NewTimer(pipelineProcessingDuration.WithLabelValues("single"))
	defer timer.ObserveDuration()

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.processors) == 0 {
		return fmt.Errorf("no processors configured in pipeline")
	}

	// Create channels for pipeline stages
	type stage struct {
		doc *Document
		err error
	}

	stages := make([]chan stage, len(p.processors))
	for i := range stages {
		stages[i] = make(chan stage, 1)
	}

	// Start goroutine for each processor
	var wg sync.WaitGroup
	for i, processor := range p.processors {
		wg.Add(1)
		go func(i int, proc DocumentProcessor) {
			defer wg.Done()
			defer close(stages[i])

			// Get input from previous stage or use initial document
			var input *Document
			if i == 0 {
				input = doc
			} else {
				select {
				case <-ctx.Done():
					return
				case prevStage := <-stages[i-1]:
					if prevStage.err != nil {
						stages[i] <- stage{err: prevStage.err}
						return
					}
					input = prevStage.doc
				}
			}

			// Process document
			processed, err := proc.Process(ctx, []byte(input.Content), input.Metadata)
			stages[i] <- stage{doc: processed, err: err}

		}(i, processor)
	}

	// Wait for all processors to complete
	wg.Wait()

	// Check final result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case finalStage := <-stages[len(stages)-1]:
		if finalStage.err != nil {
			return finalStage.err
		}
		*doc = *finalStage.doc
		p.logger.WithField("doc_id", doc.ID).Info("Document processing completed")
		return nil
	}
}
