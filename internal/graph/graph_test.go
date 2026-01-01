package graph

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	g := New()
	if g.NodeCount() != 0 {
		t.Errorf("new graph should have 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("new graph should have 0 edges, got %d", g.EdgeCount())
	}
}

func TestNewWithCapacity(t *testing.T) {
	g := NewWithCapacity(1000)
	if g.NodeCount() != 0 {
		t.Errorf("new graph should have 0 nodes, got %d", g.NodeCount())
	}
}

func TestAddNode(t *testing.T) {
	g := New()

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
	g := New()

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

func TestGetNode(t *testing.T) {
	g := New()
	g.AddNode("A")

	if n := g.GetNode("A"); n == nil {
		t.Error("GetNode should find existing node")
	}
	if n := g.GetNode("B"); n != nil {
		t.Error("GetNode should return nil for non-existent node")
	}
}

func TestMultipleEdges(t *testing.T) {
	g := New()

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

func TestConcurrentAccess(t *testing.T) {
	g := New()
	var wg sync.WaitGroup

	// Concurrent writes
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

	// Concurrent reads
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
	g := New()
	for i := 0; i < b.N; i++ {
		src := string(rune('A' + i%26))
		tgt := string(rune('A' + (i+1)%26))
		g.AddEdge(src, tgt)
	}
}

func BenchmarkGetNode(b *testing.B) {
	g := New()
	for i := 0; i < 1000; i++ {
		g.AddNode(string(rune('A' + i%26)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.GetNode(string(rune('A' + i%26)))
	}
}
