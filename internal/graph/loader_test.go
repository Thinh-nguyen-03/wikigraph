package graph

import (
	"os"
	"testing"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	f, err := os.CreateTemp("", "wikigraph-test-*.db")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	f.Close()

	db, err := database.Open(f.Name())
	if err != nil {
		os.Remove(f.Name())
		t.Fatalf("opening database: %v", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		os.Remove(f.Name())
		t.Fatalf("migrating database: %v", err)
	}

	return db, func() {
		db.Close()
		os.Remove(f.Name())
	}
}

func TestLoader_Load(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	c := cache.New(db)

	// Create pages
	pageA, _ := c.CreatePage("A")
	pageB, _ := c.CreatePage("B")
	c.CreatePage("C")

	// Mark as successful
	c.UpdatePageStatus("A", cache.StatusSuccess, "", "")
	c.UpdatePageStatus("B", cache.StatusSuccess, "", "")
	c.UpdatePageStatus("C", cache.StatusSuccess, "", "")

	// Add links: A -> B, A -> C, B -> C
	c.AddLinks(pageA.ID, []cache.Link{{TargetTitle: "B"}, {TargetTitle: "C"}})
	c.AddLinks(pageB.ID, []cache.Link{{TargetTitle: "C"}})

	// Load graph
	loader := NewLoader(c)
	g, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify nodes
	if g.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", g.NodeCount())
	}

	// Verify edges
	if g.EdgeCount() != 3 {
		t.Errorf("expected 3 edges, got %d", g.EdgeCount())
	}

	// Verify structure
	nodeA := g.GetNode("A")
	nodeB := g.GetNode("B")
	nodeC := g.GetNode("C")

	if nodeA == nil || nodeB == nil || nodeC == nil {
		t.Fatal("missing nodes")
	}

	if len(nodeA.OutLinks) != 2 {
		t.Errorf("A should have 2 outlinks, got %d", len(nodeA.OutLinks))
	}
	if len(nodeC.InLinks) != 2 {
		t.Errorf("C should have 2 inlinks, got %d", len(nodeC.InLinks))
	}
}

func TestLoader_ExcludesPendingPages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	c := cache.New(db)

	// Create pages - only A is successful
	pageA, _ := c.CreatePage("A")
	c.CreatePage("B") // stays pending
	c.UpdatePageStatus("A", cache.StatusSuccess, "", "")
	c.AddLinks(pageA.ID, []cache.Link{{TargetTitle: "B"}})

	loader := NewLoader(c)
	g, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Only A should be a node (from successful pages)
	// B appears as edge target but not as a node from Nodes list
	// However, AddEdge creates both nodes
	if g.NodeCount() != 2 {
		t.Errorf("expected 2 nodes (A + B from edge), got %d", g.NodeCount())
	}

	// Only 1 edge (A->B)
	if g.EdgeCount() != 1 {
		t.Errorf("expected 1 edge, got %d", g.EdgeCount())
	}
}

func TestLoader_EmptyCache(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	c := cache.New(db)
	loader := NewLoader(c)

	g, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if g.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", g.NodeCount())
	}
	if g.EdgeCount() != 0 {
		t.Errorf("expected 0 edges, got %d", g.EdgeCount())
	}
}
