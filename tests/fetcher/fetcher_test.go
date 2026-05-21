package fetcher_test

import (
	"context"
	"testing"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
)

func TestBuildURL(t *testing.T) {
	f := fetcher.New(fetcher.Config{
		RateLimit:      1.0,
		RequestTimeout: 5 * time.Second,
		UserAgent:      "WikiGraph-Test/1.0",
	})

	tests := []struct {
		title string
		want  string
	}{
		{"Albert Einstein", "https://en.wikipedia.org/wiki/Albert_Einstein"},
		{"Schrödinger's cat", "https://en.wikipedia.org/wiki/Schr%C3%B6dinger%27s_cat"},
		{"C++", "https://en.wikipedia.org/wiki/C++"},
	}

	for _, tt := range tests {
		got := f.BuildURL(tt.title)
		if got != tt.want {
			t.Errorf("BuildURL(%q) = %q, want %q", tt.title, got, tt.want)
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
		got := fetcher.DetectRedirect(tt.original, tt.final)
		if got != tt.want {
			t.Errorf("DetectRedirect(%q, %q) = %q, want %q", tt.original, tt.final, got, tt.want)
		}
	}
}

func TestFetch_ContextCancellation(t *testing.T) {
	f := fetcher.New(fetcher.Config{
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
