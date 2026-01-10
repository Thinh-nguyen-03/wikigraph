package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/scraper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleHealth returns the health status of the server.
// GET /health
func (s *Server) handleHealth(c *gin.Context) {
	s.mu.RLock()
	g := s.graph
	ready := s.graphReady
	s.mu.RUnlock()

	// Status is "healthy" if graph is ready, "degraded" if empty
	status := "healthy"
	if !ready {
		status = "degraded"
	}

	c.JSON(http.StatusOK, HealthResponse{
		Status:  status,
		Version: Version,
		Graph: GraphStats{
			Nodes: g.NodeCount(),
			Edges: g.EdgeCount(),
		},
		GraphReady:        ready,
		EmbeddingsEnabled: false, // Phase 3
	})
}

// handleGetPage returns a page and its links.
// GET /api/v1/page/:title
func (s *Server) handleGetPage(c *gin.Context) {
	title := c.Param("title")
	if title == "" {
		RespondWithMissingParam(c, "title")
		return
	}

	s.mu.RLock()
	node := s.graph.GetNode(title)
	s.mu.RUnlock()

	if node == nil {
		RespondWithNotFound(c, "Page", title)
		return
	}

	// Extract link titles
	outLinks := make([]string, len(node.OutLinks))
	for i, n := range node.OutLinks {
		outLinks[i] = n.Title
	}

	inLinks := make([]string, len(node.InLinks))
	for i, n := range node.InLinks {
		inLinks[i] = n.Title
	}

	c.JSON(http.StatusOK, PageResponse{
		Title:       node.Title,
		Links:       outLinks,
		LinkCount:   len(outLinks),
		InLinks:     inLinks,
		InLinkCount: len(inLinks),
		Cached:      true,
	})
}

// handleFindPath finds the shortest path between two pages.
// GET /api/v1/path?from=X&to=Y&algorithm=bfs|bidirectional&max_depth=6
func (s *Server) handleFindPath(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	if from == "" {
		RespondWithMissingParam(c, "from")
		return
	}
	if to == "" {
		RespondWithMissingParam(c, "to")
		return
	}

	algorithm := c.DefaultQuery("algorithm", "bfs")
	if algorithm != "bfs" && algorithm != "bidirectional" {
		RespondWithValidationError(c, "algorithm", "must be 'bfs' or 'bidirectional'")
		return
	}

	maxDepth := parseIntQuery(c, "max_depth", 6)
	if maxDepth < 1 || maxDepth > 20 {
		RespondWithValidationError(c, "max_depth", "must be between 1 and 20")
		return
	}

	start := time.Now()

	s.mu.RLock()
	g := s.graph
	s.mu.RUnlock()

	var result struct {
		Found    bool
		Path     []string
		Hops     int
		Explored int
	}

	switch algorithm {
	case "bidirectional":
		r := g.FindPathBidirectional(from, to)
		result.Found = r.Found
		result.Path = r.Path
		result.Hops = r.Hops
		result.Explored = r.Explored
	default:
		r := g.FindPathWithLimit(from, to, maxDepth)
		result.Found = r.Found
		result.Path = r.Path
		result.Hops = r.Hops
		result.Explored = r.Explored
	}

	duration := time.Since(start)

	c.JSON(http.StatusOK, PathResponse{
		Found:      result.Found,
		From:       from,
		To:         to,
		Path:       result.Path,
		Hops:       result.Hops,
		Explored:   result.Explored,
		Algorithm:  algorithm,
		DurationMs: duration.Milliseconds(),
	})
}

// handleGetConnections returns the N-hop neighborhood of a page.
// GET /api/v1/connections/:title?depth=2&max_nodes=1000
func (s *Server) handleGetConnections(c *gin.Context) {
	title := c.Param("title")
	if title == "" {
		RespondWithMissingParam(c, "title")
		return
	}

	depth := parseIntQuery(c, "depth", 2)
	if depth < 1 || depth > 5 {
		RespondWithValidationError(c, "depth", "must be between 1 and 5")
		return
	}

	maxNodes := parseIntQuery(c, "max_nodes", 1000)
	if maxNodes < 1 || maxNodes > 10000 {
		RespondWithValidationError(c, "max_nodes", "must be between 1 and 10000")
		return
	}

	s.mu.RLock()
	g := s.graph
	s.mu.RUnlock()

	node := g.GetNode(title)
	if node == nil {
		RespondWithNotFound(c, "Page", title)
		return
	}

	subgraph := g.GetNeighborhood(title, depth, maxNodes)
	if subgraph == nil {
		RespondWithNotFound(c, "Page", title)
		return
	}

	// Convert to response format
	nodes := make([]GraphNode, len(subgraph.Nodes))
	for i, n := range subgraph.Nodes {
		nodes[i] = GraphNode{
			ID:    n.Title,
			Title: n.Title,
			Hops:  n.Hops,
		}
	}

	edges := make([]GraphEdge, len(subgraph.Edges))
	for i, e := range subgraph.Edges {
		edges[i] = GraphEdge{
			Source: e.Source,
			Target: e.Target,
		}
	}

	c.JSON(http.StatusOK, ConnectionsResponse{
		Center:    title,
		Depth:     depth,
		Nodes:     nodes,
		Edges:     edges,
		NodeCount: len(nodes),
		EdgeCount: len(edges),
	})
}

// handleCrawl starts a background crawl job.
// POST /api/v1/crawl
func (s *Server) handleCrawl(c *gin.Context) {
	var req CrawlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondWithError(c, NewAPIError("invalid_request", err.Error(), http.StatusBadRequest))
		return
	}

	// Validate
	if req.Depth < 1 {
		req.Depth = 1
	}
	if req.Depth > 50 {
		req.Depth = 50
	}
	if req.MaxPages < 1 {
		req.MaxPages = 100
	}
	if req.MaxPages > 500000 {
		req.MaxPages = 500000
	}

	// Generate job ID
	jobID := "crawl_" + uuid.New().String()[:8]

	// Start crawl in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()

		slog.Info("starting crawl job",
			"job_id", jobID,
			"title", req.Title,
			"depth", req.Depth,
			"max_pages", req.MaxPages,
		)

		scr := scraper.New(s.cache, s.fetcher, scraper.Config{
			MaxDepth:  req.Depth,
			MaxPages:  req.MaxPages,
			BatchSize: 10,
			Workers:   30,
		})

		if _, err := scr.Crawl(ctx, []string{req.Title}); err != nil {
			slog.Error("crawl job failed",
				"job_id", jobID,
				"error", err,
			)
			return
		}

		slog.Info("crawl job completed, reloading graph", "job_id", jobID)

		if err := s.ReloadGraph(); err != nil {
			slog.Error("failed to reload graph after crawl",
				"job_id", jobID,
				"error", err,
			)
		}
	}()

	c.JSON(http.StatusAccepted, CrawlResponse{
		JobID:   jobID,
		Status:  "started",
		Message: "Crawl job started for '" + req.Title + "'",
	})
}

// parseIntQuery parses an integer query parameter with a default value.
func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}

	return n
}
