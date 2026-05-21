// Package graph provides an in-memory directed graph for Wikipedia pages.
package graph

import (
	"context"
	"sync"
)

type Node struct {
	Title    string
	OutLinks []*Node
	InLinks  []*Node
	// out is an O(1) duplicate-detection set for AddEdge.
	// It is unexported so gob ignores it; AddEdge initialises it lazily
	// from OutLinks the first time it is called on a gob-loaded node.
	out map[*Node]bool
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
	n := &Node{Title: title, out: make(map[*Node]bool)}
	g.nodes[title] = n
	return n
}

func (g *Graph) AddEdge(source, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src := g.addNode(source)
	tgt := g.addNode(target)

	// Lazily rebuild the out-set when loading from a gob cache where the
	// unexported field is not serialised.
	if src.out == nil {
		src.out = make(map[*Node]bool, len(src.OutLinks))
		for _, n := range src.OutLinks {
			src.out[n] = true
		}
	}

	if src.out[tgt] {
		return
	}

	src.OutLinks = append(src.OutLinks, tgt)
	tgt.InLinks = append(tgt.InLinks, src)
	src.out[tgt] = true
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
	if src.out != nil {
		src.out[tgt] = true
	}
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

	node.OutLinks = nil
	if node.out != nil {
		clear(node.out)
	}
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
// Returns a partial result if ctx is cancelled mid-traversal.
func (g *Graph) GetNeighborhood(ctx context.Context, title string, maxDepth, maxNodes int) *Subgraph {
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

	type queueItem struct {
		node  *Node
		depth int
	}

	// Use a head index instead of queue[1:] to avoid O(n) slice shifts.
	queue := []queueItem{{center, 0}}
	head := 0

	visited := make(map[*Node]int)
	visited[center] = 0
	result.Nodes = append(result.Nodes, SubgraphNode{Title: title, Hops: 0})

	for head < len(queue) && len(result.Nodes) < maxNodes {
		if ctx.Err() != nil {
			return result
		}

		item := queue[head]
		head++

		if item.depth >= maxDepth {
			continue
		}

		for _, neighbor := range item.node.OutLinks {
			result.Edges = append(result.Edges, SubgraphEdge{
				Source: item.node.Title,
				Target: neighbor.Title,
			})

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
