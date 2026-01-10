package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
	"github.com/gin-gonic/gin"
)

// Version is the API version.
const Version = "1.0.0"

// Server is the HTTP API server for WikiGraph.
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	graph      *graph.Graph
	cache      *cache.Cache
	fetcher    *fetcher.Fetcher
	config     Config
	mu         sync.RWMutex // protects graph and graphReady
	graphReady bool         // true if graph has been loaded with data
}

// New creates a new API server with the given dependencies.
func New(g *graph.Graph, c *cache.Cache, f *fetcher.Fetcher, cfg Config) *Server {
	s := &Server{
		graph:      g,
		cache:      c,
		fetcher:    f,
		config:     cfg,
		graphReady: g != nil && g.NodeCount() > 0,
	}
	s.setupRouter()
	return s
}

// Start starts the HTTP server and blocks until the context is cancelled
// or an error occurs.
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		slog.Info("server starting",
			"host", s.config.Host,
			"port", s.config.Port,
			"address", addr,
		)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return s.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("server shutting down")

	shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	slog.Info("server stopped")
	return nil
}

// Router returns the Gin router for testing.
func (s *Server) Router() *gin.Engine {
	return s.router
}

// Graph returns the graph with read lock for safe concurrent access.
func (s *Server) Graph() *graph.Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graph
}

// ReloadGraph reloads the graph from the cache.
// This should be called after background crawl jobs complete.
func (s *Server) ReloadGraph() error {
	slog.Info("reloading graph...")

	loader := graph.NewLoader(s.cache)
	newGraph, err := loader.Load()
	if err != nil {
		return fmt.Errorf("reloading graph: %w", err)
	}

	s.mu.Lock()
	s.graph = newGraph
	s.graphReady = newGraph.NodeCount() > 0
	s.mu.Unlock()

	slog.Info("graph reloaded",
		"nodes", newGraph.NodeCount(),
		"edges", newGraph.EdgeCount(),
		"ready", s.graphReady,
	)

	return nil
}

// IsGraphReady returns true if the graph has been loaded with data.
func (s *Server) IsGraphReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.graphReady
}
