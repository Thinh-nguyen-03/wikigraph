# Testing Guide

## Overview

This guide covers testing strategies and patterns for WikiGraph Phase 1 components.

---

## Test Structure

```
internal/
├── scraper/
│   ├── scraper.go
│   ├── scraper_test.go       # Integration tests
│   ├── fetcher.go
│   ├── fetcher_test.go       # HTTP client tests  
│   ├── parser.go
│   ├── parser_test.go        # HTML parsing tests
│   └── testdata/
│       ├── albert_einstein.html
│       ├── disambiguation.html
│       ├── redirect.html
│       ├── empty_page.html
│       └── special_chars.html
├── cache/
│   ├── cache.go
│   ├── cache_test.go
│   └── testdata/
│       └── test.db
```

---

## Running Tests

```bash
# Run all tests
make test

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/scraper/...

# Run specific test
go test -v ./internal/scraper -run TestParser_ExtractsInternalLinks

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. ./internal/scraper/
```

---

## Unit Tests

### Parser Tests

```go
// internal/scraper/parser_test.go

package scraper

import (
    "os"
    "path/filepath"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func loadTestData(t *testing.T, filename string) []byte {
    t.Helper()
    path := filepath.Join("testdata", filename)
    data, err := os.ReadFile(path)
    require.NoError(t, err, "failed to load test data: %s", filename)
    return data
}

func linkTitles(links []Link) []string {
    titles := make([]string, len(links))
    for i, l := range links {
        titles[i] = l.TargetTitle
    }
    return titles
}

func TestParser_ExtractsInternalLinks(t *testing.T) {
    html := loadTestData(t, "albert_einstein.html")
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Greater(t, len(links), 100, "expected many links")
    
    titles := linkTitles(links)
    assert.Contains(t, titles, "Physics")
    assert.Contains(t, titles, "Germany")
    assert.Contains(t, titles, "Nobel Prize in Physics")
}

func TestParser_ExcludesSpecialNamespaces(t *testing.T) {
    html := []byte(`
        <html>
        <body>
        <div id="mw-content-text">
            <a href="/wiki/Physics">Physics</a>
            <a href="/wiki/Wikipedia:About">About</a>
            <a href="/wiki/Help:Contents">Help</a>
            <a href="/wiki/File:Einstein.jpg">Image</a>
            <a href="/wiki/Category:Scientists">Category</a>
            <a href="/wiki/Template:Infobox">Template</a>
        </div>
        </body>
        </html>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    require.Len(t, links, 1)
    assert.Equal(t, "Physics", links[0].TargetTitle)
}

func TestParser_ExcludesCitations(t *testing.T) {
    html := []byte(`
        <div id="mw-content-text">
            <a href="/wiki/Physics">Physics</a>
            <a href="#cite_note-1">[1]</a>
            <a href="#cite_ref-2">[2]</a>
            <a href="/wiki/Chemistry">Chemistry</a>
        </div>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Len(t, links, 2)
}

func TestParser_ExcludesExternalLinks(t *testing.T) {
    html := []byte(`
        <div id="mw-content-text">
            <a href="/wiki/Physics">Physics</a>
            <a href="https://example.com">External</a>
            <a href="http://test.org">Another External</a>
            <a href="//other.com">Protocol-relative</a>
        </div>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Len(t, links, 1)
    assert.Equal(t, "Physics", links[0].TargetTitle)
}

func TestParser_HandlesEmptyPage(t *testing.T) {
    html := []byte(`
        <div id="mw-content-text">
            <p>This page has no links.</p>
        </div>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Empty(t, links)
}

func TestParser_DeduplicatesLinks(t *testing.T) {
    html := []byte(`
        <div id="mw-content-text">
            <a href="/wiki/Physics">Physics</a>
            <a href="/wiki/Physics">physics again</a>
            <a href="/wiki/Physics">PHYSICS</a>
        </div>
    `)
    
    p := NewParser()
    links, err := p.Parse(html)
    
    require.NoError(t, err)
    assert.Len(t, links, 1)
}
```

### Table-Driven Tests for Title Normalization

```go
func TestNormalizeTitle(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "simple title",
            input:    "Physics",
            expected: "Physics",
        },
        {
            name:     "with wiki prefix",
            input:    "/wiki/Physics",
            expected: "Physics",
        },
        {
            name:     "with underscores",
            input:    "Albert_Einstein",
            expected: "Albert Einstein",
        },
        {
            name:     "URL encoded spaces",
            input:    "Albert%20Einstein",
            expected: "Albert Einstein",
        },
        {
            name:     "URL encoded special chars",
            input:    "Caf%C3%A9",
            expected: "Café",
        },
        {
            name:     "parentheses",
            input:    "Go_(programming_language)",
            expected: "Go (programming language)",
        },
        {
            name:     "full URL",
            input:    "/wiki/Albert%20Einstein",
            expected: "Albert Einstein",
        },
        {
            name:     "leading/trailing spaces",
            input:    "  Physics  ",
            expected: "Physics",
        },
        {
            name:     "mixed case preserved",
            input:    "McDonald's",
            expected: "McDonald's",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := NormalizeTitle(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestIsSpecialNamespace(t *testing.T) {
    tests := []struct {
        title    string
        expected bool
    }{
        {"Physics", false},
        {"Albert Einstein", false},
        {"Wikipedia:About", true},
        {"Help:Contents", true},
        {"File:Einstein.jpg", true},
        {"Template:Infobox", true},
        {"Category:Scientists", true},
        {"Portal:Science", true},
        {"Talk:Physics", true},
        {"User:Someone", true},
        {"Special:Random", true},
        {"MediaWiki:Common.css", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.title, func(t *testing.T) {
            result := IsSpecialNamespace(tt.title)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Cache Tests

```go
// internal/cache/cache_test.go

package cache

import (
    "context"
    "os"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*Cache, func()) {
    t.Helper()
    
    tmpfile, err := os.CreateTemp("", "wikigraph-test-*.db")
    require.NoError(t, err)
    tmpfile.Close()
    
    c, err := New(tmpfile.Name())
    require.NoError(t, err)
    
    cleanup := func() {
        c.Close()
        os.Remove(tmpfile.Name())
    }
    
    return c, cleanup
}

func TestCache_SetAndGetPage(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    ctx := context.Background()
    
    // Set a page
    page := &CachedPage{
        Title:       "Albert Einstein",
        ContentHash: "abc123",
        FetchedAt:   time.Now(),
        Links: []Link{
            {TargetTitle: "Physics", AnchorText: "physics"},
            {TargetTitle: "Germany", AnchorText: "Germany"},
        },
    }
    
    err := c.SetPage(ctx, page)
    require.NoError(t, err)
    
    // Get the page back
    retrieved, err := c.GetPage(ctx, "Albert Einstein")
    require.NoError(t, err)
    require.NotNil(t, retrieved)
    
    assert.Equal(t, page.Title, retrieved.Title)
    assert.Equal(t, page.ContentHash, retrieved.ContentHash)
    assert.Len(t, retrieved.Links, 2)
}

func TestCache_GetPage_NotFound(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    ctx := context.Background()
    
    page, err := c.GetPage(ctx, "Nonexistent Page")
    
    assert.NoError(t, err)
    assert.Nil(t, page)
}

func TestCache_IsStale(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    // Fresh page
    freshPage := &CachedPage{
        FetchedAt: time.Now(),
    }
    assert.False(t, c.IsStale(freshPage))
    
    // Stale page (8 days old with 7 day TTL)
    stalePage := &CachedPage{
        FetchedAt: time.Now().Add(-8 * 24 * time.Hour),
    }
    assert.True(t, c.IsStale(stalePage))
}

func TestCache_Invalidate(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    ctx := context.Background()
    
    // Set a page
    page := &CachedPage{
        Title:     "Test Page",
        FetchedAt: time.Now(),
    }
    err := c.SetPage(ctx, page)
    require.NoError(t, err)
    
    // Verify it exists
    retrieved, err := c.GetPage(ctx, "Test Page")
    require.NoError(t, err)
    require.NotNil(t, retrieved)
    
    // Invalidate it
    err = c.Invalidate(ctx, "Test Page")
    require.NoError(t, err)
    
    // Verify it's gone
    retrieved, err = c.GetPage(ctx, "Test Page")
    require.NoError(t, err)
    assert.Nil(t, retrieved)
}

func TestCache_Stats(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    ctx := context.Background()
    
    // Add some pages
    for _, title := range []string{"Page1", "Page2", "Page3"} {
        page := &CachedPage{
            Title:     title,
            FetchedAt: time.Now(),
            Links: []Link{
                {TargetTitle: "Link1"},
                {TargetTitle: "Link2"},
            },
        }
        err := c.SetPage(ctx, page)
        require.NoError(t, err)
    }
    
    stats, err := c.Stats(ctx)
    require.NoError(t, err)
    
    assert.Equal(t, int64(3), stats.TotalPages)
    assert.Equal(t, int64(6), stats.TotalLinks)
}

func TestCache_UpdateExistingPage(t *testing.T) {
    c, cleanup := setupTestDB(t)
    defer cleanup()
    
    ctx := context.Background()
    
    // Set initial page
    page1 := &CachedPage{
        Title:       "Test Page",
        ContentHash: "hash1",
        FetchedAt:   time.Now().Add(-1 * time.Hour),
        Links: []Link{
            {TargetTitle: "OldLink"},
        },
    }
    err := c.SetPage(ctx, page1)
    require.NoError(t, err)
    
    // Update with new data
    page2 := &CachedPage{
        Title:       "Test Page",
        ContentHash: "hash2",
        FetchedAt:   time.Now(),
        Links: []Link{
            {TargetTitle: "NewLink1"},
            {TargetTitle: "NewLink2"},
        },
    }
    err = c.SetPage(ctx, page2)
    require.NoError(t, err)
    
    // Verify update
    retrieved, err := c.GetPage(ctx, "Test Page")
    require.NoError(t, err)
    
    assert.Equal(t, "hash2", retrieved.ContentHash)
    assert.Len(t, retrieved.Links, 2)
    assert.Equal(t, "NewLink1", retrieved.Links[0].TargetTitle)
}
```

---

## Integration Tests

### Scraper Integration Test

```go
// internal/scraper/scraper_integration_test.go

//go:build integration

package scraper

import (
    "context"
    "os"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/yourusername/wikigraph/internal/cache"
)

func TestScraper_Integration_FetchRealPage(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    // Setup temp database
    tmpfile, err := os.CreateTemp("", "wikigraph-integration-*.db")
    require.NoError(t, err)
    defer os.Remove(tmpfile.Name())
    tmpfile.Close()
    
    // Initialize cache
    c, err := cache.New(tmpfile.Name())
    require.NoError(t, err)
    defer c.Close()
    
    // Initialize scraper
    s := New(
        WithCache(c),
        WithRateLimit(2*time.Second), // Be nice to Wikipedia
        WithUserAgent("WikiGraph-Test/1.0"),
        WithTimeout(30*time.Second),
    )
    
    ctx := context.Background()
    
    // Fetch a real page
    result, err := s.Fetch(ctx, "Go (programming language)")
    require.NoError(t, err)
    
    assert.Equal(t, "Go (programming language)", result.Title)
    assert.Greater(t, len(result.Links), 50)
    assert.False(t, result.FromCache)
    
    // Fetch again - should be cached
    result2, err := s.Fetch(ctx, "Go (programming language)")
    require.NoError(t, err)
    
    assert.True(t, result2.FromCache)
    assert.Equal(t, len(result.Links), len(result2.Links))
}

func TestScraper_Integration_PageNotFound(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    s := New(
        WithRateLimit(2*time.Second),
        WithUserAgent("WikiGraph-Test/1.0"),
    )
    
    ctx := context.Background()
    
    _, err := s.Fetch(ctx, "ThisPageDefinitelyDoesNotExist12345XYZ")
    
    assert.ErrorIs(t, err, ErrPageNotFound)
}

func TestScraper_Integration_FollowsRedirect(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    s := New(
        WithRateLimit(2*time.Second),
        WithUserAgent("WikiGraph-Test/1.0"),
    )
    
    ctx := context.Background()
    
    // "Einstein" redirects to "Albert Einstein"
    result, err := s.Fetch(ctx, "Einstein")
    require.NoError(t, err)
    
    assert.Equal(t, "Albert Einstein", result.Title)
}
```

Run integration tests:

```bash
go test -v -tags=integration ./internal/scraper/
```

---

## Test Fixtures

### Creating Test HTML Files

Save actual Wikipedia HTML for consistent testing:

```bash
# Download sample pages for testing
curl -s "https://en.wikipedia.org/wiki/Albert_Einstein" > internal/scraper/testdata/albert_einstein.html

curl -s "https://en.wikipedia.org/wiki/Mercury_(disambiguation)" > internal/scraper/testdata/disambiguation.html
```

### Minimal Test HTML

For unit tests, use minimal HTML:

```go
var minimalHTML = []byte(`
<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<div id="mw-content-text">
    <p>This is a test page about <a href="/wiki/Physics">physics</a>.</p>
    <p>It also mentions <a href="/wiki/Chemistry">chemistry</a>.</p>
</div>
</body>
</html>
`)
```

---

## Mocking

### HTTP Client Mock

```go
// internal/scraper/mock_client.go

type MockHTTPClient struct {
    DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    return m.DoFunc(req)
}

// Usage in tests
func TestFetcher_WithMockClient(t *testing.T) {
    mockClient := &MockHTTPClient{
        DoFunc: func(req *http.Request) (*http.Response, error) {
            return &http.Response{
                StatusCode: 200,
                Body:       io.NopCloser(strings.NewReader("<html>...</html>")),
            }, nil
        },
    }
    
    f := NewFetcher(WithHTTPClient(mockClient))
    // ... test fetcher
}
```

### Cache Mock

```go
// internal/scraper/mock_cache.go

type MockCache struct {
    GetPageFunc    func(ctx context.Context, title string) (*CachedPage, error)
    SetPageFunc    func(ctx context.Context, page *CachedPage) error
    InvalidateFunc func(ctx context.Context, title string) error
}

func (m *MockCache) GetPage(ctx context.Context, title string) (*CachedPage, error) {
    if m.GetPageFunc != nil {
        return m.GetPageFunc(ctx, title)
    }
    return nil, nil
}

func (m *MockCache) SetPage(ctx context.Context, page *CachedPage) error {
    if m.SetPageFunc != nil {
        return m.SetPageFunc(ctx, page)
    }
    return nil
}
```

---

## Benchmarks

```go
// internal/scraper/parser_bench_test.go

func BenchmarkParser_SmallPage(b *testing.B) {
    html := loadBenchData(b, "small_page.html") // ~10KB
    p := NewParser()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = p.Parse(html)
    }
}

func BenchmarkParser_LargePage(b *testing.B) {
    html := loadBenchData(b, "large_page.html") // ~500KB
    p := NewParser()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = p.Parse(html)
    }
}

func BenchmarkNormalizeTitle(b *testing.B) {
    titles := []string{
        "/wiki/Albert%20Einstein",
        "Go_(programming_language)",
        "Simple_Title",
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        for _, t := range titles {
            _ = NormalizeTitle(t)
        }
    }
}

func BenchmarkCache_Get(b *testing.B) {
    c, cleanup := setupBenchDB(b)
    defer cleanup()
    
    ctx := context.Background()
    
    // Pre-populate
    page := &CachedPage{Title: "Test", FetchedAt: time.Now()}
    _ = c.SetPage(ctx, page)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = c.GetPage(ctx, "Test")
    }
}
```

Run benchmarks:

```bash
go test -bench=. -benchmem ./internal/scraper/
```

---

## Coverage Goals

| Package | Target | Notes |
|---------|--------|-------|
| `internal/scraper` | 80% | Core business logic |
| `internal/cache` | 80% | Data persistence |
| `internal/scraper/parser.go` | 90% | Critical parsing logic |
| `cmd/server` | 50% | CLI wiring, less critical |

Check coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
```

---

## CI Configuration

```yaml
# .github/workflows/test.yml

name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Check coverage
        run: |
          COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
          echo "Coverage: $COVERAGE%"
          if (( $(echo "$COVERAGE < 70" | bc -l) )); then
            echo "Coverage below 70%"
            exit 1
          fi
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
```
