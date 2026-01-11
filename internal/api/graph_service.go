package api

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
)

// LoadState represents the current state of graph loading.
type LoadState int

const (
	// StateUninitialized means loading hasn't started yet.
	StateUninitialized LoadState = iota
	// StateLoading means the graph is currently being loaded.
	StateLoading
	// StateReady means the graph is loaded and ready for queries.
	StateReady
	// StateError means loading failed.
	StateError
)

func (s LoadState) String() string {
	switch s {
	case StateUninitialized:
		return "uninitialized"
	case StateLoading:
		return "loading"
	case StateReady:
		return "ready"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// GraphServiceConfig configures the GraphService behavior.
type GraphServiceConfig struct {
	// CachePath is the path to the graph cache file.
	CachePath string

	// MaxCacheAge is the maximum age before cache is considered stale.
	MaxCacheAge time.Duration

	// RefreshInterval is how often to check for and apply updates.
	// If zero, automatic refresh is disabled.
	RefreshInterval time.Duration

	// ForceRebuild forces a complete rebuild ignoring cache.
	ForceRebuild bool
}

// LoadProgress tracks the progress of graph loading.
type LoadProgress struct {
	State       LoadState     `json:"state"`
	Stage       string        `json:"stage"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration_ms,omitempty"`
	Error       string        `json:"error,omitempty"`
	CacheHit    bool          `json:"cache_hit"`
	CacheAge    time.Duration `json:"cache_age_seconds,omitempty"`
}

// GraphService manages the graph lifecycle including background loading,
// caching, and incremental updates.
type GraphService struct {
	loader *graph.Loader
	cache  *cache.Cache
	config GraphServiceConfig

	mu       sync.RWMutex
	g        *graph.Graph
	state    LoadState
	progress LoadProgress
	loadErr  error

	// For graceful shutdown
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewGraphService creates a new graph service.
func NewGraphService(c *cache.Cache, cfg GraphServiceConfig) *GraphService {
	loader := graph.NewLoaderWithConfig(c, graph.LoaderConfig{
		CachePath:    cfg.CachePath,
		MaxCacheAge:  cfg.MaxCacheAge,
		ForceRebuild: cfg.ForceRebuild,
	})

	return &GraphService{
		loader: loader,
		cache:  c,
		config: cfg,
		state:  StateUninitialized,
	}
}

// Start begins background loading and optional periodic refresh.
// This method returns immediately - use IsReady() to check load status.
func (gs *GraphService) Start(ctx context.Context) {
	ctx, gs.cancel = context.WithCancel(ctx)

	// Start background loading
	gs.wg.Add(1)
	go gs.backgroundLoad(ctx)

	// Start periodic refresh if configured
	if gs.config.RefreshInterval > 0 {
		gs.wg.Add(1)
		go gs.periodicRefresh(ctx)
	}
}

// Stop gracefully stops background operations.
func (gs *GraphService) Stop() {
	if gs.cancel != nil {
		gs.cancel()
	}
	gs.wg.Wait()
}

// backgroundLoad loads the graph in the background.
func (gs *GraphService) backgroundLoad(ctx context.Context) {
	defer gs.wg.Done()

	gs.mu.Lock()
	gs.state = StateLoading
	gs.progress = LoadProgress{
		State:     StateLoading,
		Stage:     "starting",
		StartedAt: time.Now(),
	}
	gs.mu.Unlock()

	slog.Info("starting background graph load")

	// Load the graph
	g, err := gs.loader.Load()

	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.progress.CompletedAt = time.Now()
	gs.progress.Duration = gs.progress.CompletedAt.Sub(gs.progress.StartedAt)

	if err != nil {
		gs.state = StateError
		gs.loadErr = err
		gs.progress.State = StateError
		gs.progress.Stage = "failed"
		gs.progress.Error = err.Error()
		slog.Error("background graph load failed", "error", err, "duration", gs.progress.Duration)
		return
	}

	gs.g = g
	gs.state = StateReady
	gs.progress.State = StateReady
	gs.progress.Stage = "complete"

	// Check if we used cache
	if info, err := gs.loader.GetCacheInfo(); err == nil {
		gs.progress.CacheHit = true
		gs.progress.CacheAge = info.Age
	}

	slog.Info("background graph load complete",
		"nodes", g.NodeCount(),
		"edges", g.EdgeCount(),
		"duration", gs.progress.Duration.Round(time.Millisecond),
		"cache_hit", gs.progress.CacheHit,
	)
}

// periodicRefresh periodically checks for updates and refreshes the graph.
func (gs *GraphService) periodicRefresh(ctx context.Context) {
	defer gs.wg.Done()

	ticker := time.NewTicker(gs.config.RefreshInterval)
	defer ticker.Stop()

	// Track when we last updated
	var lastUpdate time.Time

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping periodic graph refresh")
			return
		case <-ticker.C:
			if !gs.IsReady() {
				continue // Skip if graph not loaded yet
			}

			// Check for updates since last refresh
			if err := gs.checkAndApplyUpdates(ctx, lastUpdate); err != nil {
				slog.Error("periodic update failed", "error", err)
			} else {
				lastUpdate = time.Now()
			}
		}
	}
}

// checkAndApplyUpdates checks for database changes and applies them to the graph.
func (gs *GraphService) checkAndApplyUpdates(ctx context.Context, since time.Time) error {
	// Get pages updated since last check
	updates, err := gs.cache.GetUpdatedPages(since)
	if err != nil {
		return fmt.Errorf("getting updated pages: %w", err)
	}

	if len(updates) == 0 {
		return nil // No updates
	}

	slog.Info("applying incremental graph updates", "pages_changed", len(updates))

	gs.mu.Lock()
	defer gs.mu.Unlock()

	for _, update := range updates {
		// Remove old edges for this page
		gs.g.RemoveOutLinks(update.Title)

		// Add new edges if page was successfully fetched
		if update.FetchStatus == "success" {
			links, err := gs.cache.GetPageLinks(update.ID)
			if err != nil {
				slog.Warn("failed to get links for updated page",
					"title", update.Title,
					"error", err,
				)
				continue
			}
			for _, target := range links {
				gs.g.AddEdge(update.Title, target)
			}
		}
	}

	// Save updated graph to cache
	if gs.config.CachePath != "" {
		if err := gs.g.Save(gs.config.CachePath); err != nil {
			slog.Warn("failed to save updated cache", "error", err)
		}
	}

	slog.Info("incremental update complete", "pages_updated", len(updates))
	return nil
}

// IsReady returns true if the graph is loaded and ready for queries.
func (gs *GraphService) IsReady() bool {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.state == StateReady
}

// GetState returns the current load state.
func (gs *GraphService) GetState() LoadState {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.state
}

// GetProgress returns detailed loading progress.
func (gs *GraphService) GetProgress() LoadProgress {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.progress
}

// GetGraph returns the loaded graph if ready.
// Returns an error if the graph is not yet loaded or failed to load.
func (gs *GraphService) GetGraph() (*graph.Graph, error) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	switch gs.state {
	case StateReady:
		return gs.g, nil
	case StateError:
		return nil, fmt.Errorf("graph loading failed: %w", gs.loadErr)
	case StateLoading:
		return nil, fmt.Errorf("graph is still loading")
	default:
		return nil, fmt.Errorf("graph service not started")
	}
}

// GetGraphStats returns basic graph statistics even during loading.
func (gs *GraphService) GetGraphStats() (nodes int, edges int) {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	if gs.g != nil {
		return gs.g.NodeCount(), gs.g.EdgeCount()
	}
	return 0, 0
}

// ForceReload triggers a complete graph rebuild from the database.
// This runs in the background and saves to cache when complete.
func (gs *GraphService) ForceReload(ctx context.Context) error {
	gs.mu.Lock()
	if gs.state == StateLoading {
		gs.mu.Unlock()
		return fmt.Errorf("graph is already loading")
	}
	gs.state = StateLoading
	gs.progress = LoadProgress{
		State:     StateLoading,
		Stage:     "rebuilding",
		StartedAt: time.Now(),
	}
	gs.mu.Unlock()

	slog.Info("forcing graph rebuild")

	g, err := gs.loader.Rebuild()

	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.progress.CompletedAt = time.Now()
	gs.progress.Duration = gs.progress.CompletedAt.Sub(gs.progress.StartedAt)

	if err != nil {
		gs.state = StateError
		gs.loadErr = err
		gs.progress.State = StateError
		gs.progress.Stage = "failed"
		gs.progress.Error = err.Error()
		return err
	}

	gs.g = g
	gs.state = StateReady
	gs.progress.State = StateReady
	gs.progress.Stage = "complete"
	gs.progress.CacheHit = false

	slog.Info("graph rebuild complete",
		"nodes", g.NodeCount(),
		"edges", g.EdgeCount(),
		"duration", gs.progress.Duration.Round(time.Millisecond),
	)

	return nil
}
