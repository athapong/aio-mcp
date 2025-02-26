package storage

import (
	"context"
	"fmt"

	"github.com/athapong/aio-mcp/pkg/graph"
	"github.com/neo4j/neo4j-go-driver/v4/neo4j"
)

// Neo4jStorage implements the Storage interface using Neo4j
type Neo4jStorage struct {
	driver  neo4j.Driver
	uri     string
	auth    neo4j.AuthToken
	session neo4j.Session
}

// NewNeo4jStorage creates a new Neo4j storage instance
func NewNeo4jStorage(uri, username, password string) (*Neo4jStorage, error) {
	auth := neo4j.BasicAuth(username, password, "")
	driver, err := neo4j.NewDriver(uri, auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %v", err)
	}

	return &Neo4jStorage{
		driver: driver,
		uri:    uri,
		auth:   auth,
	}, nil
}

// Connect implements Storage interface
func (s *Neo4jStorage) Connect(ctx context.Context) error {
	session := s.driver.NewSession(neo4j.SessionConfig{})
	s.session = session
	return nil
}

// Close implements Storage interface
func (s *Neo4jStorage) Close() error {
	if s.session != nil {
		s.session.Close()
	}
	if s.driver != nil {
		return s.driver.Close()
	}
	return nil
}

// AddEntity implements KnowledgeGraph interface
func (s *Neo4jStorage) AddEntity(ctx context.Context, entity *graph.Entity) error {
	query := `
		CREATE (e:Entity {
			id: $id,
			type: $type,
			label: $label,
			properties: $properties,
			created_at: datetime(),
			updated_at: datetime(),
			confidence: $confidence,
			source: $source
		})
	`

	params := map[string]interface{}{
		"id":         entity.ID,
		"type":       entity.Type,
		"label":      entity.Label,
		"properties": entity.Properties,
		"confidence": entity.Confidence,
		"source":     entity.Source,
	}

	_, err := s.session.Run(query, params)
	return err
}

// AddRelationship implements KnowledgeGraph interface
func (s *Neo4jStorage) AddRelationship(ctx context.Context, rel *graph.Relationship) error {
	query := `
		MATCH (from:Entity {id: $fromID})
		MATCH (to:Entity {id: $toID})
		CREATE (from)-[r:RELATES {
			id: $id,
			type: $type,
			properties: $properties,
			weight: $weight,
			created_at: datetime(),
			updated_at: datetime(),
			confidence: $confidence,
			source: $source
		}]->(to)
	`

	params := map[string]interface{}{
		"id":         rel.ID,
		"type":       rel.Type,
		"fromID":     rel.From,
		"toID":       rel.To,
		"properties": rel.Properties,
		"weight":     rel.Weight,
		"confidence": rel.Confidence,
		"source":     rel.Source,
	}

	_, err := s.session.Run(query, params)
	return err
}

// GetEntity implements KnowledgeGraph interface
func (s *Neo4jStorage) GetEntity(ctx context.Context, id string) (*graph.Entity, error) {
	query := `
		MATCH (e:Entity {id: $id})
		RETURN e
	`

	result, err := s.session.Run(query, map[string]interface{}{"id": id})
	if err != nil {
		return nil, err
	}

	if result.Next() {
		record := result.Record()
		nodeData := record.Values[0].(neo4j.Node)

		entity := &graph.Entity{
			ID:         nodeData.Props["id"].(string),
			Type:       nodeData.Props["type"].(string),
			Label:      nodeData.Props["label"].(string),
			Properties: nodeData.Props["properties"].(map[string]interface{}),
			Confidence: nodeData.Props["confidence"].(float64),
			Source:     nodeData.Props["source"].(string),
		}
		return entity, nil
	}

	return nil, fmt.Errorf("entity not found: %s", id)
}

// GetRelatedEntities implements KnowledgeGraph interface
func (s *Neo4jStorage) GetRelatedEntities(ctx context.Context, entityID string, relationType string) ([]graph.Entity, error) {
	var query string
	params := map[string]interface{}{"id": entityID}

	if relationType != "" {
		query = `
			MATCH (e:Entity {id: $id})-[r:RELATES {type: $type}]->(related:Entity)
			RETURN related
		`
		params["type"] = relationType
	} else {
		query = `
			MATCH (e:Entity {id: $id})-[r:RELATES]->(related:Entity)
			RETURN related
		`
	}

	result, err := s.session.Run(query, params)
	if err != nil {
		return nil, err
	}

	entities := make([]graph.Entity, 0)
	for result.Next() {
		record := result.Record()
		nodeData := record.Values[0].(neo4j.Node)

		entity := graph.Entity{
			ID:         nodeData.Props["id"].(string),
			Type:       nodeData.Props["type"].(string),
			Label:      nodeData.Props["label"].(string),
			Properties: nodeData.Props["properties"].(map[string]interface{}),
			Confidence: nodeData.Props["confidence"].(float64),
			Source:     nodeData.Props["source"].(string),
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// Query implements KnowledgeGraph interface
func (s *Neo4jStorage) Query(ctx context.Context, query string) (interface{}, error) {
	result, err := s.session.Run(query, nil)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for result.Next() {
		record := result.Record()
		data := make(map[string]interface{})
		for i, key := range record.Keys {
			data[key] = record.Values[i]
		}
		results = append(results, data)
	}

	return results, nil
}

// DeleteEntity implements KnowledgeGraph interface
func (s *Neo4jStorage) DeleteEntity(ctx context.Context, id string) error {
	query := `
		MATCH (e:Entity {id: $id})
		DETACH DELETE e
	`

	_, err := s.session.Run(query, map[string]interface{}{"id": id})
	return err
}

// DeleteRelationship implements KnowledgeGraph interface
func (s *Neo4jStorage) DeleteRelationship(ctx context.Context, id string) error {
	query := `
		MATCH ()-[r:RELATES {id: $id}]->()
		DELETE r
	`

	_, err := s.session.Run(query, map[string]interface{}{"id": id})
	return err
}

// BatchAdd implements KnowledgeGraph interface
func (s *Neo4jStorage) BatchAdd(ctx context.Context, entities []graph.Entity, relationships []graph.Relationship) error {
	session := s.driver.NewSession(neo4j.SessionConfig{})
	defer session.Close()

	_, err := session.WriteTransaction(func(tx neo4j.Transaction) (interface{}, error) {
		// Add entities
		for _, entity := range entities {
			params := map[string]interface{}{
				"id":         entity.ID,
				"type":       entity.Type,
				"label":      entity.Label,
				"properties": entity.Properties,
				"confidence": entity.Confidence,
				"source":     entity.Source,
			}

			_, err := tx.Run(`
				CREATE (e:Entity {
					id: $id,
					type: $type,
					label: $label,
					properties: $properties,
					created_at: datetime(),
					updated_at: datetime(),
					confidence: $confidence,
					source: $source
				})
			`, params)

			if err != nil {
				return nil, err
			}
		}

		// Add relationships
		for _, rel := range relationships {
			params := map[string]interface{}{
				"id":         rel.ID,
				"type":       rel.Type,
				"fromID":     rel.From,
				"toID":       rel.To,
				"properties": rel.Properties,
				"weight":     rel.Weight,
				"confidence": rel.Confidence,
				"source":     rel.Source,
			}

			_, err := tx.Run(`
				MATCH (from:Entity {id: $fromID})
				MATCH (to:Entity {id: $toID})
				CREATE (from)-[r:RELATES {
					id: $id,
					type: $type,
					properties: $properties,
					weight: $weight,
					created_at: datetime(),
					updated_at: datetime(),
					confidence: $confidence,
					source: $source
				}]->(to)
			`, params)

			if err != nil {
				return nil, err
			}
		}

		return nil, nil
	})

	return err
}
