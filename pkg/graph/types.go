package graph

import (
	"context"
	"time"
)

// Entity represents a node in the knowledge graph
type Entity struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	Properties map[string]interface{} `json:"properties"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Confidence float64                `json:"confidence"`
	Source     string                 `json:"source"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// Relationship represents an edge in the knowledge graph
type Relationship struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	From       string                 `json:"from"`
	To         string                 `json:"to"`
	Properties map[string]interface{} `json:"properties"`
	Weight     float64                `json:"weight"`
	CreatedAt  time.Time              `json:"created_at"`
	UpdatedAt  time.Time              `json:"updated_at"`
	Confidence float64                `json:"confidence"`
	Source     string                 `json:"source"`
}

// DocumentProcessor interface for processing different document types
type DocumentProcessor interface {
	Process(ctx context.Context, content []byte, metadata map[string]interface{}) (*Document, error)
	SupportedTypes() []string
}

// Document represents a processed document with extracted information
type Document struct {
	ID          string
	Content     string
	Sentences   []Sentence
	Entities    []Entity
	Relations   []Relationship
	Keywords    []Keyword
	Metadata    map[string]interface{}
	ProcessedAt time.Time
}

// Sentence represents a processed sentence with NLP information
type Sentence struct {
	Text      string
	StartPos  int
	EndPos    int
	Tokens    []Token
	Entities  []Entity
	Relations []Relationship
}

// Token represents a processed token with linguistic information
type Token struct {
	Text     string
	Type     string
	StartPos int
	EndPos   int
	Lemma    string
	POS      string
}

// Keyword represents an extracted keyword with relevance score
type Keyword struct {
	Text      string
	Score     float64
	StartPos  int
	EndPos    int
	Type      string
	Relations []string
}

// KnowledgeGraph interface defines the main operations for the graph
type KnowledgeGraph interface {
	AddEntity(ctx context.Context, entity *Entity) error
	AddRelationship(ctx context.Context, rel *Relationship) error
	GetEntity(ctx context.Context, id string) (*Entity, error)
	GetRelatedEntities(ctx context.Context, entityID string, relationType string) ([]Entity, error)
	Query(ctx context.Context, query string) (interface{}, error)
	DeleteEntity(ctx context.Context, id string) error
	DeleteRelationship(ctx context.Context, id string) error
	BatchAdd(ctx context.Context, entities []Entity, relationships []Relationship) error
}

// Pipeline represents the text processing pipeline
type Pipeline interface {
	Process(ctx context.Context, doc *Document) error
	AddProcessor(processor DocumentProcessor)
}

// Storage interface defines storage operations for the graph
type Storage interface {
	Connect(ctx context.Context) error
	Close() error
	KnowledgeGraph
}
