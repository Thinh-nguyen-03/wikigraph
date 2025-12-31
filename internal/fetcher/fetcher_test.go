package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBuildURL(t *testing.T) {
	f := &Fetcher{}

	tests := []struct {
		title string
		want  string
	}{
		{"Albert Einstein", "https://en.wikipedia.org/wiki/Albert_Einstein"},
		{"Schrödinger's cat", "https://en.wikipedia.org/wiki/Schr%C3%B6dinger%27s_cat"},
		{"C++", "https://en.wikipedia.org/wiki/C++"},
	}

	for _, tt := range tests {
		got := f.buildURL(tt.title)
		if got != tt.want {
			t.Errorf("buildURL(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestDetectRedirect(t *testing.T) {
	tests := []struct {
		original string
		final    string
		want     string
	}{
		{
			"https://en.wikipedia.org/wiki/Einstein",
			"https://en.wikipedia.org/wiki/Albert_Einstein",
			"Albert Einstein",
		},
		{
			"https://en.wikipedia.org/wiki/Albert_Einstein",
			"https://en.wikipedia.org/wiki/Albert_Einstein",
			"",
		},
		{
			"https://en.wikipedia.org/wiki/Schr%C3%B6dinger",
			"https://en.wikipedia.org/wiki/Erwin_Schr%C3%B6dinger",
			"Erwin Schrödinger",
		},
	}

	for _, tt := range tests {
		got := detectRedirect(tt.original, tt.final)
		if got != tt.want {
			t.Errorf("detectRedirect(%q, %q) = %q, want %q", tt.original, tt.final, got, tt.want)
		}
	}
}

func TestHashContent(t *testing.T) {
	hash1 := hashContent("hello world")
	hash2 := hashContent("hello world")
	hash3 := hashContent("different content")

	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}
	if len(hash1) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash1))
	}
}

func TestFetch_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><body>
		<div id="mw-content-text">
			<a href="/wiki/Physics">Physics</a>
			<a href="/wiki/Mathematics">Math</a>
		</div>
		</body></html>`
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(html))
	}))
	defer server.Close()

	// Note: This test is limited because Colly enforces domain restrictions
	// and we can't easily override it for unit tests with a mock server.
	// Integration tests with actual Wikipedia would be needed for full coverage.
	t.Skip("Colly domain restrictions prevent mocking; use integration tests")
}

func TestFetch_ContextCancellation(t *testing.T) {
	f := New(Config{
		RateLimit:      1.0,
		RequestTimeout: 5 * time.Second,
		UserAgent:      "WikiGraph-Test/1.0",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := f.Fetch(ctx, "Test")
	if result.Error == nil {
		t.Error("expected error for cancelled context")
	}
}
