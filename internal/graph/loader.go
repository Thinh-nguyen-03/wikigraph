package graph

import (
	"fmt"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
)

// Loads a graph from the cache.
type Loader struct {
	cache *cache.Cache
}

func NewLoader(c *cache.Cache) *Loader {
	return &Loader{cache: c}
}

// Reads all successfully fetched pages and their links into a graph.
func (l *Loader) Load() (*Graph, error) {
	data, err := l.cache.GetGraphData()
	if err != nil {
		return nil, fmt.Errorf("loading graph data: %w", err)
	}

	estimatedNodes := len(data.Edges)/5 + len(data.Nodes)
	g := NewWithCapacity(estimatedNodes)

	for _, edge := range data.Edges {
		g.AddEdge(edge[0], edge[1])
	}

	for _, title := range data.Nodes {
		g.AddNode(title)
	}

	return g, nil
}
