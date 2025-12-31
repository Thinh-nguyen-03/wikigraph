package cache

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Thinh-nguyen-03/wikigraph/internal/database"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "wikigraph-cache-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}

	db, err := database.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("opening database: %v", err)
	}

	if err := db.Migrate(); err != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("running migrations: %v", err)
	}

	return db, func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}
}

func TestGetPage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page, err := c.GetPage("Nonexistent")
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if page != nil {
		t.Error("expected nil for nonexistent page")
	}

	_, err = c.CreatePage("Test Page")
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	page, err = c.GetPage("Test Page")
	if err != nil {
		t.Fatalf("GetPage error: %v", err)
	}
	if page == nil {
		t.Fatal("expected page, got nil")
	}
	if page.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", page.Title, "Test Page")
	}
	if page.FetchStatus != StatusPending {
		t.Errorf("FetchStatus = %q, want %q", page.FetchStatus, StatusPending)
	}
}

func TestGetOrCreatePage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page1, err := c.GetOrCreatePage("New Page")
	if err != nil {
		t.Fatalf("GetOrCreatePage error: %v", err)
	}
	if page1.Title != "New Page" {
		t.Errorf("Title = %q, want %q", page1.Title, "New Page")
	}

	page2, err := c.GetOrCreatePage("New Page")
	if err != nil {
		t.Fatalf("GetOrCreatePage error: %v", err)
	}
	if page2.ID != page1.ID {
		t.Errorf("ID = %d, want %d (same page)", page2.ID, page1.ID)
	}
}

func TestUpdatePageStatus(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	_, err := c.CreatePage("Test")
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	err = c.UpdatePageStatus("Test", StatusSuccess, "abc123", "")
	if err != nil {
		t.Fatalf("UpdatePageStatus error: %v", err)
	}

	page, _ := c.GetPage("Test")
	if page.FetchStatus != StatusSuccess {
		t.Errorf("FetchStatus = %q, want %q", page.FetchStatus, StatusSuccess)
	}
	if !page.ContentHash.Valid || page.ContentHash.String != "abc123" {
		t.Errorf("ContentHash = %v, want 'abc123'", page.ContentHash)
	}
	if !page.FetchedAt.Valid {
		t.Error("FetchedAt should be set")
	}
}

func TestUpdatePageStatus_Redirect(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	_, err := c.CreatePage("Einstein")
	if err != nil {
		t.Fatalf("CreatePage error: %v", err)
	}

	err = c.UpdatePageStatus("Einstein", StatusRedirect, "", "Albert Einstein")
	if err != nil {
		t.Fatalf("UpdatePageStatus error: %v", err)
	}

	page, _ := c.GetPage("Einstein")
	if page.FetchStatus != StatusRedirect {
		t.Errorf("FetchStatus = %q, want %q", page.FetchStatus, StatusRedirect)
	}
	if !page.RedirectTo.Valid || page.RedirectTo.String != "Albert Einstein" {
		t.Errorf("RedirectTo = %v, want 'Albert Einstein'", page.RedirectTo)
	}
}

func TestGetPendingPages(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	c.CreatePage("Page1")
	c.CreatePage("Page2")
	c.CreatePage("Page3")
	c.UpdatePageStatus("Page2", StatusSuccess, "", "")

	pages, err := c.GetPendingPages(10)
	if err != nil {
		t.Fatalf("GetPendingPages error: %v", err)
	}
	if len(pages) != 2 {
		t.Errorf("got %d pending pages, want 2", len(pages))
	}
}

func TestAddLinks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page, _ := c.CreatePage("Source")

	links := []Link{
		{TargetTitle: "Target1", AnchorText: sql.NullString{String: "link text", Valid: true}},
		{TargetTitle: "Target2"},
		{TargetTitle: "Target3"},
	}

	err := c.AddLinks(page.ID, links)
	if err != nil {
		t.Fatalf("AddLinks error: %v", err)
	}

	outgoing, err := c.GetOutgoingLinks(page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks error: %v", err)
	}
	if len(outgoing) != 3 {
		t.Errorf("got %d outgoing links, want 3", len(outgoing))
	}
}

func TestAddLinks_Duplicates(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page, _ := c.CreatePage("Source")

	links := []Link{{TargetTitle: "Target"}}
	c.AddLinks(page.ID, links)
	c.AddLinks(page.ID, links)

	outgoing, _ := c.GetOutgoingLinks(page.ID)
	if len(outgoing) != 1 {
		t.Errorf("got %d links, want 1 (duplicates ignored)", len(outgoing))
	}
}

func TestGetIncomingLinks(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page1, _ := c.CreatePage("Source1")
	page2, _ := c.CreatePage("Source2")
	c.CreatePage("Target")

	c.AddLinks(page1.ID, []Link{{TargetTitle: "Target"}})
	c.AddLinks(page2.ID, []Link{{TargetTitle: "Target"}})

	incoming, err := c.GetIncomingLinks("Target")
	if err != nil {
		t.Fatalf("GetIncomingLinks error: %v", err)
	}
	if len(incoming) != 2 {
		t.Errorf("got %d incoming links, want 2", len(incoming))
	}
}

func TestEnsureTargetPagesExist(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	c.CreatePage("Existing")

	err := c.EnsureTargetPagesExist([]string{"Existing", "New1", "New2"})
	if err != nil {
		t.Fatalf("EnsureTargetPagesExist error: %v", err)
	}

	for _, title := range []string{"Existing", "New1", "New2"} {
		page, _ := c.GetPage(title)
		if page == nil {
			t.Errorf("page %q should exist", title)
		}
	}
}

func TestDeleteLinksFromPage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	c := New(db)

	page, _ := c.CreatePage("Source")
	c.AddLinks(page.ID, []Link{{TargetTitle: "T1"}, {TargetTitle: "T2"}})

	err := c.DeleteLinksFromPage(page.ID)
	if err != nil {
		t.Fatalf("DeleteLinksFromPage error: %v", err)
	}

	outgoing, _ := c.GetOutgoingLinks(page.ID)
	if len(outgoing) != 0 {
		t.Errorf("got %d links, want 0", len(outgoing))
	}
}
