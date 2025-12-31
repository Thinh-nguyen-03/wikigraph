# Scraper Package API Reference

## Package Overview

```go
import "github.com/yourusername/wikigraph/internal/scraper"
```

The scraper package provides functionality for fetching Wikipedia pages and extracting their internal links.

---

## Types

### Scraper

Main scraper type that orchestrates fetching and parsing.

```go
type Scraper struct {
    // contains filtered or unexported fields
}
```

#### Constructor

```go
func New(opts ...Option) *Scraper
```

Creates a new Scraper with the given options.

**Example:**

```go
s := scraper.New(
    scraper.WithCache(cache),
    scraper.WithRateLimit(time.Second),
    scraper.WithUserAgent("WikiGraph/1.0"),
    scraper.WithTimeout(30 * time.Second),
)
```

---

### Option

Functional option for configuring the Scraper.

```go
type Option func(*Scraper)
```

#### Available Options

```go
// WithCache sets the cache implementation
func WithCache(c Cache) Option

// WithRateLimit sets the minimum duration between requests
func WithRateLimit(d time.Duration) Option

// WithUserAgent sets the HTTP User-Agent header
func WithUserAgent(ua string) Option

// WithTimeout sets the HTTP request timeout
func WithTimeout(d time.Duration) Option

// WithRetries sets the number of retry attempts for failed requests
func WithRetries(n int) Option

// WithLogger sets a custom logger
func WithLogger(l Logger) Option
```

---

### PageResult

Result of fetching a Wikipedia page.

```go
type PageResult struct {
    // Title is the canonical page title (after following redirects)
    Title string
    
    // Links contains all internal Wikipedia links found on the page
    Links []Link
    
    // FetchedAt is when the page was fetched
    FetchedAt time.Time
    
    // FromCache indicates if the result came from cache
    FromCache bool
    
    // FetchDuration is how long the fetch took (zero if cached)
    FetchDuration time.Duration
    
    // ContentHash is a hash of the page content
    ContentHash string
}
```

---

### Link

Represents a link from one Wikipedia page to another.

```go
type Link struct {
    // TargetTitle is the title of the linked page
    TargetTitle string
    
    // AnchorText is the visible text of the link (may be empty)
    AnchorText string
}
```

---

### FetchOptions

Options for customizing a single fetch operation.

```go
type FetchOptions struct {
    // BypassCache forces fetching from Wikipedia even if cached
    BypassCache bool
    
    // MaxLinks limits the number of links returned (0 = no limit)
    MaxLinks int
    
    // IncludeAnchors includes anchor text in results
    IncludeAnchors bool
    
    // FollowRedirects follows Wikipedia redirects (default: true)
    FollowRedirects bool
}
```

---

## Methods

### Scraper.Fetch

```go
func (s *Scraper) Fetch(ctx context.Context, title string) (*PageResult, error)
```

Fetches a Wikipedia page and returns its links. Uses cache if available and fresh.

**Parameters:**
- `ctx` - Context for cancellation and timeouts
- `title` - Wikipedia page title (with or without underscores)

**Returns:**
- `*PageResult` - The page data and links
- `error` - Any error that occurred

**Example:**

```go
ctx := context.Background()
result, err := s.Fetch(ctx, "Albert Einstein")
if err != nil {
    if errors.Is(err, scraper.ErrPageNotFound) {
        log.Println("Page does not exist")
    }
    return err
}

fmt.Printf("Found %d links\n", len(result.Links))
```

---

### Scraper.FetchWithOptions

```go
func (s *Scraper) FetchWithOptions(ctx context.Context, title string, opts FetchOptions) (*PageResult, error)
```

Fetches a Wikipedia page with custom options.

**Example:**

```go
result, err := s.FetchWithOptions(ctx, "Albert Einstein", scraper.FetchOptions{
    BypassCache:    true,
    MaxLinks:       100,
    IncludeAnchors: true,
})
```

---

### Scraper.FetchBatch

```go
func (s *Scraper) FetchBatch(ctx context.Context, titles []string) ([]*PageResult, error)
```

Fetches multiple pages concurrently (respecting rate limits).

**Example:**

```go
titles := []string{"Physics", "Chemistry", "Biology"}
results, err := s.FetchBatch(ctx, titles)
if err != nil {
    return err
}

for _, r := range results {
    fmt.Printf("%s: %d links\n", r.Title, len(r.Links))
}
```

---

### Scraper.Close

```go
func (s *Scraper) Close() error
```

Closes the scraper and releases resources.

---

## Errors

```go
var (
    // ErrPageNotFound is returned when the Wikipedia page doesn't exist
    ErrPageNotFound = errors.New("page not found")
    
    // ErrInvalidTitle is returned for malformed page titles
    ErrInvalidTitle = errors.New("invalid page title")
    
    // ErrRateLimited is returned when Wikipedia rate limits the request
    ErrRateLimited = errors.New("rate limited by Wikipedia")
    
    // ErrTimeout is returned when the request times out
    ErrTimeout = errors.New("request timeout")
    
    // ErrNetworkError is returned for network-related failures
    ErrNetworkError = errors.New("network error")
    
    // ErrContextCanceled is returned when the context is canceled
    ErrContextCanceled = errors.New("context canceled")
)
```

### Error Handling Example

```go
result, err := s.Fetch(ctx, title)
if err != nil {
    switch {
    case errors.Is(err, scraper.ErrPageNotFound):
        // Handle missing page
    case errors.Is(err, scraper.ErrRateLimited):
        // Wait and retry
    case errors.Is(err, scraper.ErrTimeout):
        // Increase timeout or skip
    case errors.Is(err, context.Canceled):
        // Request was canceled
    default:
        // Unknown error
    }
}
```

---

## Utility Functions

### NormalizeTitle

```go
func NormalizeTitle(title string) string
```

Normalizes a Wikipedia page title to canonical form.

**Transformations:**
- Removes `/wiki/` prefix
- Decodes URL encoding (`%20` → space)
- Replaces underscores with spaces
- Trims whitespace

**Example:**

```go
NormalizeTitle("/wiki/Albert%20Einstein")  // "Albert Einstein"
NormalizeTitle("Albert_Einstein")           // "Albert Einstein"
NormalizeTitle("  Physics  ")               // "Physics"
```

---

### IsValidTitle

```go
func IsValidTitle(title string) bool
```

Checks if a title is valid for Wikipedia.

**Rules:**
- Not empty
- Not longer than 256 characters
- Doesn't contain `< > { } [ ] |`
- Doesn't start with special namespace

---

### IsSpecialNamespace

```go
func IsSpecialNamespace(title string) bool
```

Checks if a title belongs to a special Wikipedia namespace.

**Special namespaces:**
- `Wikipedia:`
- `Help:`
- `File:`
- `Template:`
- `Category:`
- `Portal:`
- `Talk:`
- `User:`
- `Special:`
- `MediaWiki:`

---

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/yourusername/wikigraph/internal/cache"
    "github.com/yourusername/wikigraph/internal/scraper"
)

func main() {
    // Initialize cache
    c, err := cache.New("wikigraph.db", cache.WithTTL(7*24*time.Hour))
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Initialize scraper
    s := scraper.New(
        scraper.WithCache(c),
        scraper.WithRateLimit(time.Second),
        scraper.WithUserAgent("WikiGraph/1.0 (github.com/user/wikigraph)"),
        scraper.WithTimeout(30*time.Second),
        scraper.WithRetries(3),
    )
    defer s.Close()
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    // Fetch a page
    result, err := s.Fetch(ctx, "Albert Einstein")
    if err != nil {
        log.Fatalf("Failed to fetch: %v", err)
    }
    
    // Print results
    fmt.Printf("Title: %s\n", result.Title)
    fmt.Printf("Links: %d\n", len(result.Links))
    fmt.Printf("Cached: %v\n", result.FromCache)
    fmt.Printf("Fetch time: %v\n", result.FetchDuration)
    
    // Print first 10 links
    fmt.Println("\nFirst 10 links:")
    for i, link := range result.Links {
        if i >= 10 {
            break
        }
        fmt.Printf("  - %s\n", link.TargetTitle)
    }
    
    // Fetch multiple pages
    titles := []string{"Physics", "Chemistry", "Biology"}
    results, err := s.FetchBatch(ctx, titles)
    if err != nil {
        log.Fatalf("Batch fetch failed: %v", err)
    }
    
    fmt.Println("\nBatch results:")
    for _, r := range results {
        fmt.Printf("  %s: %d links\n", r.Title, len(r.Links))
    }
}
```

---

## Testing

### Mocking the Scraper

```go
type MockScraper struct {
    FetchFunc func(ctx context.Context, title string) (*PageResult, error)
}

func (m *MockScraper) Fetch(ctx context.Context, title string) (*PageResult, error) {
    return m.FetchFunc(ctx, title)
}

// Usage in tests
func TestSomething(t *testing.T) {
    mock := &MockScraper{
        FetchFunc: func(ctx context.Context, title string) (*PageResult, error) {
            return &PageResult{
                Title: title,
                Links: []Link{{TargetTitle: "Physics"}},
            }, nil
        },
    }
    
    // Use mock in your tests
}
```

### Test Helpers

```go
// LoadTestHTML loads HTML from testdata directory
func LoadTestHTML(t *testing.T, filename string) []byte {
    t.Helper()
    data, err := os.ReadFile(filepath.Join("testdata", filename))
    if err != nil {
        t.Fatalf("Failed to load test data: %v", err)
    }
    return data
}
```
