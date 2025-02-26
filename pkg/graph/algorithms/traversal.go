package algorithms

import (
	"context"
	"fmt"

	"github.com/athapong/aio-mcp/pkg/graph"
)

type TraversalType string

const (
	BFS TraversalType = "BFS"
	DFS TraversalType = "DFS"
)

type GraphTraversal struct {
	graph graph.KnowledgeGraph
}

func NewGraphTraversal(g graph.KnowledgeGraph) *GraphTraversal {
	return &GraphTraversal{graph: g}
}

func (t *GraphTraversal) Traverse(ctx context.Context, startID string, maxDepth int, traversalType TraversalType) ([]graph.Entity, error) {
	visited := make(map[string]bool)
	result := make([]graph.Entity, 0)

	switch traversalType {
	case BFS:
		return t.bfs(ctx, startID, maxDepth, visited)
	case DFS:
		return t.dfs(ctx, startID, maxDepth, visited, &result)
	default:
		return nil, fmt.Errorf("unsupported traversal type: %s", traversalType)
	}
}

func (t *GraphTraversal) bfs(ctx context.Context, startID string, maxDepth int, visited map[string]bool) ([]graph.Entity, error) {
	// BFS implementation
	queue := []string{startID}
	result := make([]graph.Entity, 0)
	depth := 0

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			current := queue[0]
			queue = queue[1:]

			if visited[current] {
				continue
			}

			visited[current] = true
			entity, err := t.graph.GetEntity(ctx, current)
			if err != nil {
				return nil, err
			}
			result = append(result, *entity)

			// Get related entities
			related, err := t.graph.GetRelatedEntities(ctx, current, "")
			if err != nil {
				return nil, err
			}

			for _, r := range related {
				if !visited[r.ID] {
					queue = append(queue, r.ID)
				}
			}
		}
		depth++
	}

	return result, nil
}

func (t *GraphTraversal) dfs(ctx context.Context, currentID string, maxDepth int, visited map[string]bool, result *[]graph.Entity) ([]graph.Entity, error) {
	// DFS implementation
	if maxDepth < 0 || visited[currentID] {
		return *result, nil
	}

	visited[currentID] = true
	entity, err := t.graph.GetEntity(ctx, currentID)
	if err != nil {
		return nil, err
	}
	*result = append(*result, *entity)

	related, err := t.graph.GetRelatedEntities(ctx, currentID, "")
	if err != nil {
		return nil, err
	}

	for _, r := range related {
		if !visited[r.ID] {
			if _, err := t.dfs(ctx, r.ID, maxDepth-1, visited, result); err != nil {
				return nil, err
			}
		}
	}

	return *result, nil
}
