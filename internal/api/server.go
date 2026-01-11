package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Thinh-nguyen-03/wikigraph/internal/cache"
	"github.com/Thinh-nguyen-03/wikigraph/internal/fetcher"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
	"github.com/gin-gonic/gin"
)

// Version is the API version.
const Version = "1.0.0"

// Server is the HTTP API server for WikiGraph.
type Server struct {
	router       *gin.Engine
	httpServer   *http.Server
	graphService *GraphService
	cache        *cache.Cache
	fetcher      *fetcher.Fetcher
	config       Config
}

// NewWithGraphService creates a new API server with GraphService for background loading.
// This is the preferred constructor for production use.
func NewWithGraphService(gs *GraphService, c *cache.Cache, f *fetcher.Fetcher, cfg Config) *Server {
	s := &Server{
		graphService: gs,
		cache:        c,
		fetcher:      f,
		config:       cfg,
	}
	s.setupRouter()
	return s
}

// New creates a new API server with a pre-loaded graph.
// This constructor is kept for backward compatibility and testing.
// For production use, prefer NewWithGraphService for background loading.
func New(g *graph.Graph, c *cache.Cache, f *fetcher.Fetcher, cfg Config) *Server {
	// Create a simple GraphService wrapper around the provided graph
	gs := &GraphService{
		g:     g,
		cache: c,
		state: StateReady,
		progress: LoadProgress{
			State: StateReady,
			Stage: "complete",
		},
	}

	return NewWithGraphService(gs, c, f, cfg)
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

// Graph returns the graph if ready, or nil if still loading.
func (s *Server) Graph() *graph.Graph {
	g, _ := s.graphService.GetGraph()
	return g
}

// GraphService returns the underlying GraphService.
func (s *Server) GraphService() *GraphService {
	return s.graphService
}

// ReloadGraph triggers a complete graph rebuild from the database.
// This should be called after background crawl jobs complete.
func (s *Server) ReloadGraph() error {
	return s.graphService.ForceReload(context.Background())
}

// IsGraphReady returns true if the graph has been loaded with data.
func (s *Server) IsGraphReady() bool {
	return s.graphService.IsReady()
}
