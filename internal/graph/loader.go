package graph

import "github.com/Thinh-nguyen-03/wikigraph/internal/cache"

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
		return nil, err
	}

	g := NewWithCapacity(len(data.Nodes))

	// Add all nodes first (ensures pages with no outlinks are included)
	for _, title := range data.Nodes {
		g.AddNode(title)
	}

	// Add all edges
	for _, edge := range data.Edges {
		g.AddEdge(edge[0], edge[1])
	}

	return g, nil
}
