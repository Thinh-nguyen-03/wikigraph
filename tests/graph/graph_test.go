package graph_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
)

func TestNew(t *testing.T) {
	g := graph.New()
	if g.NodeCount() != 0 {
		t.Errorf("new graph should have 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("new graph should have 0 edges, got %d", g.EdgeCount())
	}
}

func TestNewWithCapacity(t *testing.T) {
	g := graph.NewWithCapacity(1000)
	if g.NodeCount() != 0 {
		t.Errorf("new graph should have 0 nodes, got %d", g.NodeCount())
	}
}

func TestAddNode(t *testing.T) {
	g := graph.New()

	n1 := g.AddNode("A")
	if n1.Title != "A" {
		t.Errorf("expected title A, got %s", n1.Title)
	}
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", g.NodeCount())
	}

	// Adding same node returns existing
	n2 := g.AddNode("A")
	if n1 != n2 {
		t.Error("adding same title should return same node")
	}
	if g.NodeCount() != 1 {
		t.Errorf("expected 1 node after duplicate add, got %d", g.NodeCount())
	}
}

func TestAddEdge(t *testing.T) {
	g := graph.New()

	g.AddEdge("A", "B")

	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}

	a := g.GetNode("A")
	b := g.GetNode("B")

	if len(a.OutLinks) != 1 || a.OutLinks[0] != b {
		t.Error("A should have outlink to B")
	}
	if len(b.InLinks) != 1 || b.InLinks[0] != a {
		t.Error("B should have inlink from A")
	}
	if len(a.InLinks) != 0 {
		t.Error("A should have no inlinks")
	}
	if len(b.OutLinks) != 0 {
		t.Error("B should have no outlinks")
	}
}

func TestAddEdge_Duplicate(t *testing.T) {
	g := graph.New()

	g.AddEdge("A", "B")
	g.AddEdge("A", "B") // duplicate — should be silently dropped

	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount = %d after duplicate AddEdge, want 1", g.EdgeCount())
	}
	a := g.GetNode("A")
	if len(a.OutLinks) != 1 {
		t.Errorf("OutLinks len = %d, want 1", len(a.OutLinks))
	}
	b := g.GetNode("B")
	if len(b.InLinks) != 1 {
		t.Errorf("InLinks len = %d after duplicate, want 1", len(b.InLinks))
	}
}

// TestAddEdge_DuplicateAfterCacheLoad verifies the lazy-init path: after a
// gob round-trip the unexported out set is nil and must be rebuilt on first AddEdge.
func TestAddEdge_DuplicateAfterCacheLoad(t *testing.T) {
	g := graph.New()
	g.AddEdge("A", "B")

	tmp, err := os.CreateTemp("", "graph-cache-*.gob")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	if err := g.Save(tmp.Name()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, _, err := graph.LoadFromCache(tmp.Name())
	if err != nil {
		t.Fatalf("LoadFromCache: %v", err)
	}

	// After cache load, out is nil. AddEdge must lazily rebuild it and not duplicate.
	loaded.AddEdge("A", "B")
	if loaded.EdgeCount() != 1 {
		t.Errorf("EdgeCount = %d after duplicate AddEdge on cache-loaded graph, want 1", loaded.EdgeCount())
	}
}

func TestAddEdgeUnchecked(t *testing.T) {
	g := graph.New()

	// Unchecked bypasses dedup — each call must increment edges.
	g.AddEdgeUnchecked("A", "B")
	g.AddEdgeUnchecked("A", "B")

	if g.EdgeCount() != 2 {
		t.Errorf("EdgeCount = %d after two AddEdgeUnchecked, want 2", g.EdgeCount())
	}
}

func TestGetNode(t *testing.T) {
	g := graph.New()
	g.AddNode("A")

	if n := g.GetNode("A"); n == nil {
		t.Error("GetNode should find existing node")
	}
	if n := g.GetNode("B"); n != nil {
		t.Error("GetNode should return nil for non-existent node")
	}
}

func TestMultipleEdges(t *testing.T) {
	g := graph.New()

	g.AddEdge("A", "B")
	g.AddEdge("A", "C")
	g.AddEdge("B", "C")

	if g.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 3 {
		t.Errorf("expected 3 edges, got %d", g.EdgeCount())
	}

	a := g.GetNode("A")
	c := g.GetNode("C")

	if len(a.OutLinks) != 2 {
		t.Errorf("A should have 2 outlinks, got %d", len(a.OutLinks))
	}
	if len(c.InLinks) != 2 {
		t.Errorf("C should have 2 inlinks, got %d", len(c.InLinks))
	}
}

func TestRemoveOutLinks_CleansInLinks(t *testing.T) {
	g := graph.New()
	g.AddEdge("A", "B")
	g.AddEdge("A", "C")

	if g.EdgeCount() != 2 {
		t.Fatalf("setup: EdgeCount = %d, want 2", g.EdgeCount())
	}

	g.RemoveOutLinks("A")

	if g.EdgeCount() != 0 {
		t.Errorf("EdgeCount = %d after RemoveOutLinks, want 0", g.EdgeCount())
	}

	a := g.GetNode("A")
	if len(a.OutLinks) != 0 {
		t.Errorf("A.OutLinks len = %d after remove, want 0", len(a.OutLinks))
	}

	b := g.GetNode("B")
	if len(b.InLinks) != 0 {
		t.Errorf("B.InLinks len = %d after remove, want 0", len(b.InLinks))
	}

	c := g.GetNode("C")
	if len(c.InLinks) != 0 {
		t.Errorf("C.InLinks len = %d after remove, want 0", len(c.InLinks))
	}
}

func TestRemoveOutLinks_NonExistent(t *testing.T) {
	g := graph.New()
	// Must not panic on unknown node.
	g.RemoveOutLinks("nobody")
}

func TestRemoveOutLinks_PreservesOtherEdges(t *testing.T) {
	g := graph.New()
	g.AddEdge("A", "C")
	g.AddEdge("B", "C") // C has two in-links

	g.RemoveOutLinks("A")

	// B→C must still be intact
	if g.EdgeCount() != 1 {
		t.Errorf("EdgeCount = %d, want 1 (B->C survives)", g.EdgeCount())
	}
	c := g.GetNode("C")
	if len(c.InLinks) != 1 {
		t.Errorf("C.InLinks len = %d, want 1 (only B remains)", len(c.InLinks))
	}
	if c.InLinks[0] != g.GetNode("B") {
		t.Error("C's remaining in-link should be B")
	}
}

func TestGetNeighborhood_Depth(t *testing.T) {
	// A → B → C → D
	g := graph.New()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("C", "D")

	sub1 := g.GetNeighborhood(context.Background(), "A", 1, 100)
	if sub1 == nil {
		t.Fatal("depth=1 returned nil")
	}
	titles1 := subgraphTitles(sub1)
	if !contains(titles1, "A") || !contains(titles1, "B") {
		t.Errorf("depth=1 should include A and B, got %v", titles1)
	}
	if contains(titles1, "C") {
		t.Errorf("depth=1 should not include C, got %v", titles1)
	}

	sub2 := g.GetNeighborhood(context.Background(), "A", 2, 100)
	titles2 := subgraphTitles(sub2)
	if !contains(titles2, "C") {
		t.Errorf("depth=2 should include C, got %v", titles2)
	}
	if contains(titles2, "D") {
		t.Errorf("depth=2 should not include D, got %v", titles2)
	}
}

func TestGetNeighborhood_MaxNodes(t *testing.T) {
	// A fans out to B, C, D, E, F
	g := graph.New()
	for _, target := range []string{"B", "C", "D", "E", "F"} {
		g.AddEdge("A", target)
	}

	sub := g.GetNeighborhood(context.Background(), "A", 2, 3)
	if len(sub.Nodes) > 3 {
		t.Errorf("got %d nodes, want <= 3 (maxNodes=3)", len(sub.Nodes))
	}
}

func TestGetNeighborhood_NotFound(t *testing.T) {
	g := graph.New()
	sub := g.GetNeighborhood(context.Background(), "nobody", 2, 100)
	if sub != nil {
		t.Error("expected nil for unknown node")
	}
}

func TestGetNeighborhood_ContextCancel(t *testing.T) {
	g := graph.New()
	g.AddEdge("A", "B")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Must not panic; returns partial (possibly empty) result.
	sub := g.GetNeighborhood(ctx, "A", 5, 1000)
	_ = sub // result may be nil or partial — just must not panic
}

func TestConcurrentAccess(t *testing.T) {
	g := graph.New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			src := string(rune('A' + i%26))
			tgt := string(rune('A' + (i+1)%26))
			g.AddEdge(src, tgt)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.NodeCount()
			g.EdgeCount()
			g.GetNode("A")
		}()
	}
	wg.Wait()
}

func BenchmarkAddEdge(b *testing.B) {
	g := graph.New()
	for i := 0; i < b.N; i++ {
		src := string(rune('A' + i%26))
		tgt := string(rune('A' + (i+1)%26))
		g.AddEdge(src, tgt)
	}
}

func BenchmarkGetNode(b *testing.B) {
	g := graph.New()
	for i := 0; i < 1000; i++ {
		g.AddNode(string(rune('A' + i%26)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetNode(string(rune('A' + i%26)))
	}
}

// helpers

func subgraphTitles(sub *graph.Subgraph) []string {
	titles := make([]string, len(sub.Nodes))
	for i, n := range sub.Nodes {
		titles[i] = n.Title
	}
	return titles
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
