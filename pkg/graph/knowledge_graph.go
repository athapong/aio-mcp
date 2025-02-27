package graph

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Node represents a node in the knowledge graph
type Node struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Sources    []string               `json:"sources,omitempty"` // Document IDs where this node was found
}

// Edge represents a relationship between nodes in the knowledge graph
type Edge struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"` // Source node ID
	Target     string                 `json:"target"` // Target node ID
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Weight     float64                `json:"weight"`
}

// KnowledgeGraphData represents a graph of knowledge extracted from documents
type KnowledgeGraphData struct {
	Nodes       []Node    `json:"nodes"`
	Edges       []Edge    `json:"edges"`
	GeneratedAt time.Time `json:"generated_at"`
}

// MemoryKnowledgeGraph implements the KnowledgeGraph interface with in-memory storage
type MemoryKnowledgeGraph struct {
	data    *KnowledgeGraphData
	nodeMap map[string]*Node // For quick lookup by ID
	edgeMap map[string]*Edge // For quick lookup by ID
	mutex   sync.RWMutex
	logger  *logrus.Logger
}

// NewMemoryKnowledgeGraph creates a new in-memory knowledge graph
func NewMemoryKnowledgeGraph() *MemoryKnowledgeGraph {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &MemoryKnowledgeGraph{
		data: &KnowledgeGraphData{
			Nodes:       make([]Node, 0),
			Edges:       make([]Edge, 0),
			GeneratedAt: time.Now(),
		},
		nodeMap: make(map[string]*Node),
		edgeMap: make(map[string]*Edge),
		logger:  logger,
	}
}

// AddEntity adds an entity to the graph
func (g *MemoryKnowledgeGraph) AddEntity(ctx context.Context, entity *Entity) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Convert Entity to Node
	nodeID := uuid.New().String()
	node := Node{
		ID:         nodeID,
		Label:      entity.Label,
		Type:       entity.Type,
		Properties: entity.Properties,
		Sources:    []string{entity.Source},
	}

	g.data.Nodes = append(g.data.Nodes, node)
	g.nodeMap[nodeID] = &g.data.Nodes[len(g.data.Nodes)-1]
	return nil
}

// AddRelationship adds a relationship to the graph
func (g *MemoryKnowledgeGraph) AddRelationship(ctx context.Context, rel *Relationship) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	// Ensure source and target nodes exist
	sourceExists := g.nodeMap[rel.From]
	targetExists := g.nodeMap[rel.To]

	if sourceExists == nil || targetExists == nil {
		return fmt.Errorf("source or target node not found")
	}

	// Create edge
	edgeID := fmt.Sprintf("%s-%s-%s", rel.From, rel.Type, rel.To)
	edge := Edge{
		ID:         edgeID,
		Source:     rel.From,
		Target:     rel.To,
		Type:       rel.Type,
		Properties: rel.Properties,
		Weight:     rel.Confidence,
	}

	g.data.Edges = append(g.data.Edges, edge)
	g.edgeMap[edgeID] = &g.data.Edges[len(g.data.Edges)-1]
	return nil
}

// GetEntity retrieves an entity by ID
func (g *MemoryKnowledgeGraph) GetEntity(ctx context.Context, id string) (*Entity, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	node, exists := g.nodeMap[id]
	if !exists {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	// Convert Node to Entity
	entity := &Entity{
		ID:         node.ID,
		Label:      node.Label,
		Type:       node.Type,
		Properties: node.Properties,
	}

	return entity, nil
}

// GetRelatedEntities gets entities related to a given entity
func (g *MemoryKnowledgeGraph) GetRelatedEntities(ctx context.Context, entityID string, relationType string) ([]Entity, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	related := make([]Entity, 0)

	for _, edge := range g.data.Edges {
		// Match source entity and optionally relationship type
		if edge.Source == entityID && (relationType == "" || edge.Type == relationType) {
			targetNode, exists := g.nodeMap[edge.Target]
			if exists {
				related = append(related, Entity{
					ID:         targetNode.ID,
					Label:      targetNode.Label,
					Type:       targetNode.Type,
					Properties: targetNode.Properties,
				})
			}
		}
		// Match target entity and optionally relationship type
		if edge.Target == entityID && (relationType == "" || edge.Type == relationType) {
			sourceNode, exists := g.nodeMap[edge.Source]
			if exists {
				related = append(related, Entity{
					ID:         sourceNode.ID,
					Label:      sourceNode.Label,
					Type:       sourceNode.Type,
					Properties: sourceNode.Properties,
				})
			}
		}
	}

	return related, nil
}

// Query executes a query against the graph (simplified implementation)
func (g *MemoryKnowledgeGraph) Query(ctx context.Context, query string) (interface{}, error) {
	// This is a placeholder for more complex query capabilities
	return nil, fmt.Errorf("query not implemented: %s", query)
}

// DeleteEntity removes an entity from the graph
func (g *MemoryKnowledgeGraph) DeleteEntity(ctx context.Context, id string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if _, exists := g.nodeMap[id]; !exists {
		return fmt.Errorf("entity not found: %s", id)
	}

	// Remove related edges first
	for i := len(g.data.Edges) - 1; i >= 0; i-- {
		if g.data.Edges[i].Source == id || g.data.Edges[i].Target == id {
			delete(g.edgeMap, g.data.Edges[i].ID)
			g.data.Edges = append(g.data.Edges[:i], g.data.Edges[i+1:]...)
		}
	}

	// Remove the node
	for i, node := range g.data.Nodes {
		if node.ID == id {
			g.data.Nodes = append(g.data.Nodes[:i], g.data.Nodes[i+1:]...)
			break
		}
	}
	delete(g.nodeMap, id)

	return nil
}

// DeleteRelationship removes a relationship from the graph
func (g *MemoryKnowledgeGraph) DeleteRelationship(ctx context.Context, id string) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if _, exists := g.edgeMap[id]; !exists {
		return fmt.Errorf("relationship not found: %s", id)
	}

	// Remove the edge
	for i, edge := range g.data.Edges {
		if edge.ID == id {
			g.data.Edges = append(g.data.Edges[:i], g.data.Edges[i+1:]...)
			break
		}
	}
	delete(g.edgeMap, id)

	return nil
}

// BatchAdd adds multiple entities and relationships in a batch
func (g *MemoryKnowledgeGraph) BatchAdd(ctx context.Context, entities []Entity, relationships []Relationship) error {
	for _, entity := range entities {
		e := entity // Create a copy to avoid issues with loop variable
		if err := g.AddEntity(ctx, &e); err != nil {
			return err
		}
	}

	for _, rel := range relationships {
		r := rel // Create a copy to avoid issues with loop variable
		if err := g.AddRelationship(ctx, &r); err != nil {
			return err
		}
	}

	return nil
}

// GetData returns the graph data for serialization or visualization
func (g *MemoryKnowledgeGraph) GetData() *KnowledgeGraphData {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	return g.data
}

// KnowledgeGraphGenerator creates a knowledge graph from processed documents
type KnowledgeGraphGenerator struct {
	nodes       map[string]Node   // map of node label to node
	edges       map[string]Edge   // map of edge ID to edge
	documentMap map[string]bool   // tracking processed document IDs
	nodeIDMap   map[string]string // maps entity label to node ID
	mutex       sync.RWMutex
	logger      *logrus.Logger
}

// NewKnowledgeGraphGenerator creates a new knowledge graph generator
func NewKnowledgeGraphGenerator() *KnowledgeGraphGenerator {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &KnowledgeGraphGenerator{
		nodes:       make(map[string]Node),
		edges:       make(map[string]Edge),
		documentMap: make(map[string]bool),
		nodeIDMap:   make(map[string]string),
		logger:      logger,
	}
}

// AddDocument adds a document to the knowledge graph
func (g *KnowledgeGraphGenerator) AddDocument(doc *Document) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	if doc == nil {
		return fmt.Errorf("cannot add nil document to graph")
	}

	// Skip if document was already processed
	if _, exists := g.documentMap[doc.ID]; exists {
		return nil
	}
	g.documentMap[doc.ID] = true

	// Add entities as nodes
	for _, entity := range doc.Entities {
		nodeID, exists := g.nodeIDMap[entity.Label]

		if !exists {
			// Create new node
			nodeID = uuid.New().String()
			node := Node{
				ID:         nodeID,
				Label:      entity.Label,
				Type:       entity.Type,
				Properties: entity.Properties,
				Sources:    []string{doc.ID},
			}
			g.nodes[nodeID] = node
			g.nodeIDMap[entity.Label] = nodeID
		} else {
			// Update existing node with new source
			node := g.nodes[nodeID]
			node.Sources = append(node.Sources, doc.ID)
			g.nodes[nodeID] = node
		}
	}

	// Add relationships as edges
	for _, rel := range doc.Relations {
		sourceNodeID, sourceExists := g.nodeIDMap[rel.From]
		targetNodeID, targetExists := g.nodeIDMap[rel.To]

		if !sourceExists || !targetExists {
			g.logger.WithFields(logrus.Fields{
				"relation": rel,
				"doc_id":   doc.ID,
			}).Warn("Skipping relation with unknown entities")
			continue
		}

		edgeID := fmt.Sprintf("%s-%s-%s", sourceNodeID, rel.Type, targetNodeID)

		if _, exists := g.edges[edgeID]; !exists {
			// Create new edge
			edge := Edge{
				ID:     edgeID,
				Source: sourceNodeID,
				Target: targetNodeID,
				Type:   rel.Type,
				Properties: map[string]interface{}{
					"confidence": rel.Confidence,
				},
				Weight: rel.Confidence,
			}
			g.edges[edgeID] = edge
		} else {
			// Update edge weight
			edge := g.edges[edgeID]
			edge.Weight = (edge.Weight + rel.Confidence) / 2 // Average confidence
			g.edges[edgeID] = edge
		}
	}

	return nil
}

// Generate builds and returns the final knowledge graph
func (g *KnowledgeGraphGenerator) Generate() *KnowledgeGraphData {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	// Convert maps to slices
	nodes := make([]Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}

	edges := make([]Edge, 0, len(g.edges))
	for _, edge := range g.edges {
		edges = append(edges, edge)
	}

	return &KnowledgeGraphData{
		Nodes:       nodes,
		Edges:       edges,
		GeneratedAt: time.Now(),
	}
}
