package graph

import (
	"testing"
)

func TestFindPath(t *testing.T) {
	tests := []struct {
		name     string
		edges    [][2]string
		from     string
		to       string
		wantPath []string
		wantHops int
	}{
		{
			name:     "direct link",
			edges:    [][2]string{{"A", "B"}},
			from:     "A",
			to:       "B",
			wantPath: []string{"A", "B"},
			wantHops: 1,
		},
		{
			name:     "two hops",
			edges:    [][2]string{{"A", "B"}, {"B", "C"}},
			from:     "A",
			to:       "C",
			wantPath: []string{"A", "B", "C"},
			wantHops: 2,
		},
		{
			name:     "same node",
			edges:    [][2]string{{"A", "B"}},
			from:     "A",
			to:       "A",
			wantPath: []string{"A"},
			wantHops: 0,
		},
		{
			name:     "no path",
			edges:    [][2]string{{"A", "B"}, {"C", "D"}},
			from:     "A",
			to:       "D",
			wantPath: nil,
			wantHops: 0,
		},
		{
			name:     "shortest of multiple paths",
			edges:    [][2]string{{"A", "B"}, {"B", "C"}, {"A", "C"}},
			from:     "A",
			to:       "C",
			wantPath: []string{"A", "C"},
			wantHops: 1,
		},
		{
			name:     "node not in graph",
			edges:    [][2]string{{"A", "B"}},
			from:     "A",
			to:       "Z",
			wantPath: nil,
			wantHops: 0,
		},
		{
			name:     "longer path",
			edges:    [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "E"}},
			from:     "A",
			to:       "E",
			wantPath: []string{"A", "B", "C", "D", "E"},
			wantHops: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			for _, edge := range tt.edges {
				g.AddEdge(edge[0], edge[1])
			}

			result := g.FindPath(tt.from, tt.to)

			if tt.wantPath == nil {
				if result.Found {
					t.Errorf("expected not found, got path %v", result.Path)
				}
			} else {
				if !result.Found {
					t.Errorf("expected found, got not found")
					return
				}
				if !equalSlices(result.Path, tt.wantPath) {
					t.Errorf("path = %v, want %v", result.Path, tt.wantPath)
				}
				if result.Hops != tt.wantHops {
					t.Errorf("hops = %d, want %d", result.Hops, tt.wantHops)
				}
			}
		})
	}
}

func TestFindPathWithLimit(t *testing.T) {
	g := New()
	// A -> B -> C -> D -> E
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")
	g.AddEdge("D", "E")

	// Can find with sufficient depth
	result := g.FindPathWithLimit("A", "E", 4)
	if !result.Found {
		t.Error("should find path with maxDepth=4")
	}

	// Cannot find with insufficient depth
	result = g.FindPathWithLimit("A", "E", 3)
	if result.Found {
		t.Error("should not find path with maxDepth=3")
	}

	// Edge case: maxDepth=0 only finds same node
	result = g.FindPathWithLimit("A", "A", 0)
	if !result.Found {
		t.Error("should find same node with maxDepth=0")
	}

	result = g.FindPathWithLimit("A", "B", 0)
	if result.Found {
		t.Error("should not find neighbor with maxDepth=0")
	}
}

func TestFindPathBidirectional(t *testing.T) {
	tests := []struct {
		name     string
		edges    [][2]string
		from     string
		to       string
		wantHops int
		wantFind bool
	}{
		{
			name:     "direct link",
			edges:    [][2]string{{"A", "B"}},
			from:     "A",
			to:       "B",
			wantHops: 1,
			wantFind: true,
		},
		{
			name:     "two hops",
			edges:    [][2]string{{"A", "B"}, {"B", "C"}},
			from:     "A",
			to:       "C",
			wantHops: 2,
			wantFind: true,
		},
		{
			name:     "same node",
			edges:    [][2]string{{"A", "B"}},
			from:     "A",
			to:       "A",
			wantHops: 0,
			wantFind: true,
		},
		{
			name:     "no path",
			edges:    [][2]string{{"A", "B"}, {"C", "D"}},
			from:     "A",
			to:       "D",
			wantHops: 0,
			wantFind: false,
		},
		{
			name:     "longer path",
			edges:    [][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "E"}, {"E", "F"}},
			from:     "A",
			to:       "F",
			wantHops: 5,
			wantFind: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New()
			for _, edge := range tt.edges {
				g.AddEdge(edge[0], edge[1])
			}

			result := g.FindPathBidirectional(tt.from, tt.to)

			if result.Found != tt.wantFind {
				t.Errorf("found = %v, want %v", result.Found, tt.wantFind)
			}
			if tt.wantFind && result.Hops != tt.wantHops {
				t.Errorf("hops = %d, want %d", result.Hops, tt.wantHops)
			}
		})
	}
}

func TestPathfinderExploredCount(t *testing.T) {
	g := New()
	// Create a wide graph
	for i := 0; i < 10; i++ {
		g.AddEdge("A", string(rune('B'+i)))
	}
	g.AddEdge("K", "Z") // K is 'B'+9

	result := g.FindPath("A", "Z")
	if !result.Found {
		t.Fatal("should find path")
	}
	if result.Explored < 1 {
		t.Error("explored should be positive")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BenchmarkFindPath_SmallGraph(b *testing.B) {
	g := buildChainGraph(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.FindPath("node_0", "node_99")
	}
}

func BenchmarkFindPath_LargeGraph(b *testing.B) {
	g := buildChainGraph(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.FindPath("node_0", "node_9999")
	}
}

func BenchmarkFindPathBidirectional_LargeGraph(b *testing.B) {
	g := buildChainGraph(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.FindPathBidirectional("node_0", "node_9999")
	}
}

func buildChainGraph(n int) *Graph {
	g := NewWithCapacity(n)
	for i := 0; i < n-1; i++ {
		g.AddEdge(nodeName(i), nodeName(i+1))
	}
	return g
}

func nodeName(i int) string {
	return "node_" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
