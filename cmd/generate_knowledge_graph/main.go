package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/athapong/aio-mcp/pkg/graph"
	"github.com/athapong/aio-mcp/pkg/graph/processors"
	"github.com/athapong/aio-mcp/pkg/graph/storage"
	"github.com/athapong/aio-mcp/pkg/graph/visualizer"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	inputDir        = flag.String("input", "", "Directory containing input text files")
	outputFile      = flag.String("output", "knowledge_graph.json", "Output file path for the knowledge graph")
	visualize       = flag.Bool("visualize", false, "Generate a visualization of the knowledge graph")
	visualizeOutput = flag.String("viz-output", "knowledge_graph.html", "Output file for the visualization")
	logLevel        = flag.String("log-level", "info", "Logging level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	// Configure logging
	logger := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		logger.Fatalf("Invalid log level: %v", err)
	}
	logger.SetLevel(level)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	if *inputDir == "" {
		logger.Fatal("Input directory must be specified")
	}

	// Create the document processor pipeline
	pipeline := graph.NewPipeline()
	pipeline.AddProcessor(processors.NewNLPProcessor())

	// Create a graph store
	graphStore := storage.NewJSONGraphStore(*outputFile)

	// Process all input files
	var knowledgeGraph *graph.KnowledgeGraphData

	files, err := readInputFiles(*inputDir)
	if err != nil {
		logger.Fatalf("Failed to read input directory: %v", err)
	}

	if len(files) == 0 {
		logger.Fatal("No input files found")
	}

	logger.Infof("Processing %d input files...", len(files))

	documents := make([]*graph.Document, 0, len(files))
	for _, file := range files {
		content, err := os.ReadFile(file) // Using os.ReadFile instead of deprecated ioutil
		if err != nil {
			logger.Errorf("Failed to read file %s: %v", file, err)
			continue
		}

		// Create document with metadata
		doc := &graph.Document{
			ID:      uuid.New().String(),
			Content: string(content),
			Metadata: map[string]interface{}{
				"filename": filepath.Base(file),
				"filepath": file,
			},
		}
		documents = append(documents, doc)
	}

	// Process documents
	ctx := context.Background()
	err = pipeline.BatchProcess(ctx, documents)
	if err != nil {
		logger.Fatalf("Failed to process documents: %v", err)
	}

	// Build knowledge graph
	generator := graph.NewKnowledgeGraphGenerator()
	for _, doc := range documents {
		if err := generator.AddDocument(doc); err != nil {
			logger.Errorf("Failed to add document to graph: %v", err)
		}
	}
	knowledgeGraph = generator.Generate()

	// Store the knowledge graph
	if err := graphStore.StoreGraph(ctx, knowledgeGraph); err != nil {
		logger.Fatalf("Failed to store knowledge graph: %v", err)
	}

	logger.Infof("Knowledge graph generated with %d nodes and %d edges",
		len(knowledgeGraph.Nodes), len(knowledgeGraph.Edges))
	logger.Infof("Knowledge graph saved to %s", *outputFile)

	// Visualize the graph if requested
	if *visualize {
		viz := visualizer.NewD3Visualizer(*visualizeOutput)
		if err := viz.Visualize(knowledgeGraph); err != nil {
			logger.Errorf("Failed to visualize knowledge graph: %v", err)
		} else {
			logger.Infof("Visualization saved to %s", *visualizeOutput)
		}
	}
}

// readInputFiles reads all text files from the input directory
func readInputFiles(inputDir string) ([]string, error) {
	supportedExtensions := map[string]bool{
		".txt": true, ".md": true, ".json": true, ".html": true,
	}

	var files []string
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if supportedExtensions[ext] {
				files = append(files, path)
			}
		}
		return nil
	})

	return files, err
}
