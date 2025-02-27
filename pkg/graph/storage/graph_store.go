package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/athapong/aio-mcp/pkg/graph"
)

// GraphStore defines an interface for storing knowledge graphs
type GraphStore interface {
	// StoreGraph persists a knowledge graph
	StoreGraph(ctx context.Context, graph *graph.KnowledgeGraphData) error

	// LoadGraph loads a knowledge graph from storage
	LoadGraph(ctx context.Context) (*graph.KnowledgeGraphData, error)
}

// JSONGraphStore implements GraphStore using JSON files
type JSONGraphStore struct {
	filePath string
}

// NewJSONGraphStore creates a new JSON graph store
func NewJSONGraphStore(filePath string) *JSONGraphStore {
	return &JSONGraphStore{
		filePath: filePath,
	}
}

// StoreGraph stores the knowledge graph as JSON
func (s *JSONGraphStore) StoreGraph(ctx context.Context, graph *graph.KnowledgeGraphData) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Encode graph to JSON
	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(s.filePath, data, 0644)
}

// LoadGraph loads a knowledge graph from a JSON file
func (s *JSONGraphStore) LoadGraph(ctx context.Context) (*graph.KnowledgeGraphData, error) {
	// Read file
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, err
	}

	// Decode JSON
	var graph graph.KnowledgeGraphData
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, err
	}

	return &graph, nil
}
