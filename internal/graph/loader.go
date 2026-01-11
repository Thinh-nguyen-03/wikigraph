package graph

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
)

// LoaderConfig configures the graph loader behavior.
type LoaderConfig struct {
	// CachePath is the path to the graph cache file.
	// If empty, caching is disabled.
	CachePath string

	// MaxCacheAge is the maximum age of a cache before it's considered stale.
	// A stale cache will still be used but a rebuild will be triggered.
	// If zero, caches never expire based on age.
	MaxCacheAge time.Duration

	// ForceRebuild forces a rebuild from the database, ignoring any cache.
	ForceRebuild bool
}

// Loader loads graphs from the cache/database with optional disk caching.
type Loader struct {
	cache  *cache.Cache
	config LoaderConfig
}

// NewLoader creates a new graph loader.
func NewLoader(c *cache.Cache) *Loader {
	return &Loader{cache: c}
}

// NewLoaderWithConfig creates a new graph loader with the given configuration.
func NewLoaderWithConfig(c *cache.Cache, cfg LoaderConfig) *Loader {
	return &Loader{cache: c, config: cfg}
}

// Load loads the graph, using disk cache if available and valid.
// Falls back to loading from database if cache is missing, invalid, or forced rebuild.
func (l *Loader) Load() (*Graph, error) {
	// If caching is disabled or force rebuild, go straight to database
	if l.config.CachePath == "" || l.config.ForceRebuild {
		return l.loadFromDatabase()
	}

	// Try to load from cache
	g, age, err := LoadFromCache(l.config.CachePath)
	if err == nil {
		// Check if cache is too old
		if l.config.MaxCacheAge > 0 && age > l.config.MaxCacheAge {
			slog.Warn("cache is stale, will rebuild in background",
				"age", age.Round(time.Second),
				"max_age", l.config.MaxCacheAge,
			)
			// Still return the stale cache - caller can trigger background rebuild
		}
		return g, nil
	}

	// Cache miss or error - log and fall back to database
	slog.Info("cache unavailable, loading from database", "reason", err)
	return l.loadFromDatabaseAndCache()
}

// loadFromDatabase loads the graph from the database without caching.
func (l *Loader) loadFromDatabase() (*Graph, error) {
	start := time.Now()
	slog.Info("loading graph from database...")

	data, err := l.cache.GetGraphData()
	if err != nil {
		return nil, fmt.Errorf("loading graph data: %w", err)
	}

	estimatedNodes := len(data.Edges)/5 + len(data.Nodes)
	g := NewWithCapacity(estimatedNodes)

	// Use unchecked add for bulk loading - database guarantees uniqueness
	for _, edge := range data.Edges {
		g.AddEdgeUnchecked(edge[0], edge[1])
	}

	for _, title := range data.Nodes {
		g.AddNode(title)
	}

	slog.Info("graph loaded from database",
		"nodes", g.NodeCount(),
		"edges", g.EdgeCount(),
		"duration", time.Since(start).Round(time.Millisecond),
	)

	return g, nil
}

// loadFromDatabaseAndCache loads from database and saves to cache.
func (l *Loader) loadFromDatabaseAndCache() (*Graph, error) {
	g, err := l.loadFromDatabase()
	if err != nil {
		return nil, err
	}

	// Skip caching for very large graphs where gob serialization becomes a bottleneck.
	// Threshold: 10M edges (~500MB cache file)
	const maxCacheableEdges = 10_000_000

	if l.config.CachePath != "" {
		if g.EdgeCount() > maxCacheableEdges {
			slog.Info("skipping cache save - graph exceeds cacheable size",
				"edges", g.EdgeCount(),
				"threshold", maxCacheableEdges)
		} else {
			start := time.Now()
			if err := g.Save(l.config.CachePath); err != nil {
				slog.Warn("failed to save graph cache", "error", err)
				// Non-fatal - graph is still valid
			} else {
				slog.Info("graph cache saved", "duration", time.Since(start).Round(time.Millisecond))
			}
		}
	}

	return g, nil
}

// Rebuild forces a complete rebuild from the database and updates the cache.
func (l *Loader) Rebuild() (*Graph, error) {
	slog.Info("forcing graph rebuild from database")
	return l.loadFromDatabaseAndCache()
}

// GetCacheInfo returns information about the current cache state.
func (l *Loader) GetCacheInfo() (*CacheInfo, error) {
	if l.config.CachePath == "" {
		return nil, fmt.Errorf("caching is disabled")
	}
	return GetCacheInfo(l.config.CachePath)
}

// InvalidateCache deletes the cache file, forcing a rebuild on next load.
func (l *Loader) InvalidateCache() error {
	if l.config.CachePath == "" {
		return nil
	}
	return DeleteCache(l.config.CachePath)
}
