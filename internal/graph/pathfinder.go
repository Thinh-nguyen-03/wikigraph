package graph

type PathResult struct {
	Found    bool
	Path     []string
	Hops     int
	Explored int
}

// nodeQueue implements a simple queue using head/tail indices to avoid
// repeated memory allocations during BFS traversal.
type nodeQueue struct {
	items []*Node
	head  int
	tail  int
}

func newNodeQueue(capacity int) *nodeQueue {
	return &nodeQueue{
		items: make([]*Node, capacity),
	}
}

func (q *nodeQueue) push(n *Node) {
	if q.tail >= len(q.items) {
		newItems := make([]*Node, len(q.items)*2)
		copy(newItems, q.items[q.head:q.tail])
		q.items = newItems
		q.tail -= q.head
		q.head = 0
	}
	q.items[q.tail] = n
	q.tail++
}

func (q *nodeQueue) pop() *Node {
	if q.head >= q.tail {
		return nil
	}
	n := q.items[q.head]
	q.items[q.head] = nil
	q.head++
	return n
}

func (q *nodeQueue) len() int {
	return q.tail - q.head
}

func (q *nodeQueue) reset() {
	q.head = 0
	q.tail = 0
}

func (g *Graph) FindPath(from, to string) PathResult {
	return g.FindPathWithLimit(from, to, -1)
}

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

	queue := newNodeQueue(64)
	queue.push(fromNode)
	visited[fromNode] = true
	explored := 0
	depth := 0

	currentLevelCount := 1
	nextLevelCount := 0

	for queue.len() > 0 {
		if maxDepth >= 0 && depth >= maxDepth {
			break
		}

		for i := 0; i < currentLevelCount; i++ {
			current := queue.pop()
			if current == nil {
				break
			}
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
				queue.push(neighbor)
				nextLevelCount++
			}
		}
		currentLevelCount = nextLevelCount
		nextLevelCount = 0
		depth++
	}

	return PathResult{Explored: explored}
}

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
	var pathF []*Node
	for n := meeting; n != nil; n = parentF[n] {
		pathF = append(pathF, n)
	}
	for i, j := 0, len(pathF)-1; i < j; i, j = i+1, j-1 {
		pathF[i], pathF[j] = pathF[j], pathF[i]
	}

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
	length := 0
	for n := to; n != nil; n = parent[n] {
		length++
	}

	result := make([]string, length)
	i := length - 1
	for n := to; n != nil; n = parent[n] {
		result[i] = n.Title
		i--
	}

	return result
}
