// Package graph provides an in-memory directed graph for Wikipedia pages.
package graph

import "sync"

type Node struct {
	Title    string
	OutLinks []*Node
	InLinks  []*Node
}

type Graph struct {
	nodes map[string]*Node
	edges int
	mu    sync.RWMutex
}

func New() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

func NewWithCapacity(nodeCapacity int) *Graph {
	return &Graph{nodes: make(map[string]*Node, nodeCapacity)}
}

func (g *Graph) AddNode(title string) *Node {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addNode(title)
}

func (g *Graph) addNode(title string) *Node {
	if n := g.nodes[title]; n != nil {
		return n
	}
	n := &Node{Title: title}
	g.nodes[title] = n
	return n
}

func (g *Graph) AddEdge(source, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src := g.addNode(source)
	tgt := g.addNode(target)

	for _, existing := range src.OutLinks {
		if existing == tgt {
			return
		}
	}

	src.OutLinks = append(src.OutLinks, tgt)
	tgt.InLinks = append(tgt.InLinks, src)
	g.edges++
}

// AddEdgeUnchecked adds an edge without duplicate checking.
// Use only for bulk loading from trusted sources where uniqueness is guaranteed.
func (g *Graph) AddEdgeUnchecked(source, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src := g.addNode(source)
	tgt := g.addNode(target)

	src.OutLinks = append(src.OutLinks, tgt)
	tgt.InLinks = append(tgt.InLinks, src)
	g.edges++
}

// RemoveOutLinks removes all outgoing edges from a node.
// Used for incremental updates when a page's links have changed.
func (g *Graph) RemoveOutLinks(title string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := g.nodes[title]
	if node == nil {
		return
	}

	// Remove this node from each target's InLinks
	for _, target := range node.OutLinks {
		newInLinks := make([]*Node, 0, len(target.InLinks)-1)
		for _, inLink := range target.InLinks {
			if inLink != node {
				newInLinks = append(newInLinks, inLink)
			}
		}
		target.InLinks = newInLinks
		g.edges--
	}

	// Clear outlinks
	node.OutLinks = nil
}

func (g *Graph) GetNode(title string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[title]
}

func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.edges
}

// Subgraph represents a subset of the graph.
type Subgraph struct {
	Nodes []SubgraphNode
	Edges []SubgraphEdge
}

// SubgraphNode represents a node in a subgraph with distance from center.
type SubgraphNode struct {
	Title string
	Hops  int
}

// SubgraphEdge represents an edge in a subgraph.
type SubgraphEdge struct {
	Source string
	Target string
}

// GetNeighborhood returns the N-hop neighborhood around a node using BFS.
func (g *Graph) GetNeighborhood(title string, maxDepth, maxNodes int) *Subgraph {
	g.mu.RLock()
	defer g.mu.RUnlock()

	center := g.nodes[title]
	if center == nil {
		return nil
	}

	result := &Subgraph{
		Nodes: make([]SubgraphNode, 0, maxNodes),
		Edges: make([]SubgraphEdge, 0),
	}

	// Track visited nodes with their hop distance
	visited := make(map[*Node]int)
	visited[center] = 0
	result.Nodes = append(result.Nodes, SubgraphNode{Title: title, Hops: 0})

	// BFS queue: pairs of (node, depth)
	type queueItem struct {
		node  *Node
		depth int
	}
	queue := []queueItem{{center, 0}}

	for len(queue) > 0 && len(result.Nodes) < maxNodes {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Process outgoing links
		for _, neighbor := range item.node.OutLinks {
			// Add edge (even if neighbor was visited, we want all edges)
			result.Edges = append(result.Edges, SubgraphEdge{
				Source: item.node.Title,
				Target: neighbor.Title,
			})

			// Add node if not visited
			if _, seen := visited[neighbor]; !seen {
				if len(result.Nodes) >= maxNodes {
					break
				}
				visited[neighbor] = item.depth + 1
				result.Nodes = append(result.Nodes, SubgraphNode{
					Title: neighbor.Title,
					Hops:  item.depth + 1,
				})
				queue = append(queue, queueItem{neighbor, item.depth + 1})
			}
		}
	}

	return result
}
