package graph

// Holds the result of a pathfinding operation.
type PathResult struct {
	Found    bool
	Path     []string
	Hops     int
	Explored int
}

// Finds the shortest path using BFS.
func (g *Graph) FindPath(from, to string) PathResult {
	return g.FindPathWithLimit(from, to, -1)
}

// Finds shortest path with a maximum depth. Use -1 for unlimited.
func (g *Graph) FindPathWithLimit(from, to string, maxDepth int) PathResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	fromNode := g.nodes[from]
	toNode := g.nodes[to]

	if fromNode == nil || toNode == nil {
		return PathResult{}
	}

	if fromNode == toNode {
		return PathResult{Found: true, Path: []string{from}, Hops: 0, Explored: 1}
	}

	visited := make(map[*Node]bool)
	parent := make(map[*Node]*Node)
	queue := []*Node{fromNode}
	visited[fromNode] = true
	explored := 0
	depth := 0

	for len(queue) > 0 {
		if maxDepth >= 0 && depth >= maxDepth {
			break
		}

		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			current := queue[i]
			explored++

			for _, neighbor := range current.OutLinks {
				if visited[neighbor] {
					continue
				}

				parent[neighbor] = current
				if neighbor == toNode {
					return PathResult{
						Found:    true,
						Path:     reconstructPath(parent, fromNode, toNode),
						Hops:     depth + 1,
						Explored: explored,
					}
				}

				visited[neighbor] = true
				queue = append(queue, neighbor)
			}
		}
		queue = queue[levelSize:]
		depth++
	}

	return PathResult{Explored: explored}
}

// Uses bidirectional BFS for faster results on large graphs.
func (g *Graph) FindPathBidirectional(from, to string) PathResult {
	g.mu.RLock()
	defer g.mu.RUnlock()

	fromNode := g.nodes[from]
	toNode := g.nodes[to]

	if fromNode == nil || toNode == nil {
		return PathResult{}
	}

	if fromNode == toNode {
		return PathResult{Found: true, Path: []string{from}, Hops: 0, Explored: 1}
	}

	// Forward search state (using OutLinks)
	visitedF := map[*Node]bool{fromNode: true}
	parentF := map[*Node]*Node{}
	queueF := []*Node{fromNode}

	// Backward search state (using InLinks)
	visitedB := map[*Node]bool{toNode: true}
	parentB := map[*Node]*Node{}
	queueB := []*Node{toNode}

	explored := 0

	for len(queueF) > 0 && len(queueB) > 0 {
		// Expand smaller frontier first for efficiency
		if len(queueF) <= len(queueB) {
			if meeting := expandForward(queueF, visitedF, parentF, visitedB, &explored); meeting != nil {
				return buildBidiPath(parentF, parentB, fromNode, toNode, meeting, explored)
			}
			queueF = nextLevel(queueF, visitedF, parentF, true)
		} else {
			if meeting := expandBackward(queueB, visitedB, parentB, visitedF, &explored); meeting != nil {
				return buildBidiPath(parentF, parentB, fromNode, toNode, meeting, explored)
			}
			queueB = nextLevel(queueB, visitedB, parentB, false)
		}
	}

	return PathResult{Explored: explored}
}

func expandForward(queue []*Node, visited map[*Node]bool, parent map[*Node]*Node, other map[*Node]bool, explored *int) *Node {
	for _, node := range queue {
		(*explored)++
		for _, neighbor := range node.OutLinks {
			if visited[neighbor] {
				continue
			}
			parent[neighbor] = node
			if other[neighbor] {
				return neighbor
			}
			visited[neighbor] = true
		}
	}
	return nil
}

func expandBackward(queue []*Node, visited map[*Node]bool, parent map[*Node]*Node, other map[*Node]bool, explored *int) *Node {
	for _, node := range queue {
		(*explored)++
		for _, neighbor := range node.InLinks {
			if visited[neighbor] {
				continue
			}
			parent[neighbor] = node
			if other[neighbor] {
				return neighbor
			}
			visited[neighbor] = true
		}
	}
	return nil
}

func nextLevel(queue []*Node, visited map[*Node]bool, parent map[*Node]*Node, forward bool) []*Node {
	var next []*Node
	for _, node := range queue {
		var neighbors []*Node
		if forward {
			neighbors = node.OutLinks
		} else {
			neighbors = node.InLinks
		}
		for _, neighbor := range neighbors {
			if !visited[neighbor] {
				continue
			}
			if parent[neighbor] == node {
				next = append(next, neighbor)
			}
		}
	}
	return next
}

func buildBidiPath(parentF, parentB map[*Node]*Node, from, to, meeting *Node, explored int) PathResult {
	// Build path from start to meeting point
	var pathF []*Node
	for n := meeting; n != nil; n = parentF[n] {
		pathF = append(pathF, n)
	}
	// Reverse to get from -> meeting order
	for i, j := 0, len(pathF)-1; i < j; i, j = i+1, j-1 {
		pathF[i], pathF[j] = pathF[j], pathF[i]
	}

	// Build path from meeting point to end (skip meeting, already included)
	for n := parentB[meeting]; n != nil; n = parentB[n] {
		pathF = append(pathF, n)
	}

	path := make([]string, len(pathF))
	for i, n := range pathF {
		path[i] = n.Title
	}

	return PathResult{
		Found:    true,
		Path:     path,
		Hops:     len(path) - 1,
		Explored: explored,
	}
}

func reconstructPath(parent map[*Node]*Node, from, to *Node) []string {
	var path []*Node
	for n := to; n != nil; n = parent[n] {
		path = append(path, n)
	}
	// Reverse
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}

	result := make([]string, len(path))
	for i, n := range path {
		result[i] = n.Title
	}
	return result
}
