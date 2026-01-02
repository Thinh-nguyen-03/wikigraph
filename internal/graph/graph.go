// Package graph provides an in-memory directed graph for Wikipedia pages.
package graph

import "sync"

// Represents a Wikipedia page in the graph.
type Node struct {
	Title    string
	OutLinks []*Node
	InLinks  []*Node
}

// Thread-safe directed graph of Wikipedia pages.
type Graph struct {
	nodes map[string]*Node
	edges int
	mu    sync.RWMutex
}

func New() *Graph {
	return &Graph{nodes: make(map[string]*Node)}
}

// Creates a graph with pre-allocated capacity for efficiency.
func NewWithCapacity(nodeCapacity int) *Graph {
	return &Graph{nodes: make(map[string]*Node, nodeCapacity)}
}

// Returns existing node or creates a new one.
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

// Adds a directed edge, creating nodes if needed.
func (g *Graph) AddEdge(source, target string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	src := g.addNode(source)
	tgt := g.addNode(target)

	for _, existing := range src.OutLinks {
		if existing == tgt {
			return // Edge already exists, skip
		}
	}

	src.OutLinks = append(src.OutLinks, tgt)
	tgt.InLinks = append(tgt.InLinks, src)
	g.edges++
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
