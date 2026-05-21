package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Thinh-nguyen-03/wikigraph/internal/neostore"
	"github.com/Thinh-nguyen-03/wikigraph/internal/scraper"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleHealth returns the health status of the server.
// GET /health
func (s *Server) handleHealth(c *gin.Context) {
	progress := s.graphService.GetProgress()
	nodes, edges := s.graphService.GetGraphStats()

	// Determine status based on graph state
	var status string
	var httpStatus int

	switch progress.State {
	case StateReady:
		status = "healthy"
		httpStatus = http.StatusOK
	case StateLoading:
		status = "loading"
		httpStatus = http.StatusOK // Health check returns 200 even when loading
	case StateError:
		status = "error"
		httpStatus = http.StatusServiceUnavailable
	default:
		status = "initializing"
		httpStatus = http.StatusOK
	}

	// Build response
	response := HealthResponse{
		Status:  status,
		Version: Version,
		Graph: GraphStats{
			Nodes: nodes,
			Edges: edges,
		},
		GraphReady: progress.State == StateReady,
	}

	// Add Neo4j status if enabled
	if s.neo4jEnabled {
		neo4jStatus := &Neo4jStatus{
			Enabled:   true,
			Connected: s.neo4jClient != nil,
		}

		if s.neo4jClient != nil {
			stats, connected := s.getCachedNeo4jStats(c.Request.Context())
			neo4jStatus.Connected = connected
			if stats != nil {
				neo4jStatus.Nodes = int(stats.NodeCount)
				neo4jStatus.Edges = int(stats.EdgeCount)
			}
			if !connected {
				neo4jStatus.Error = "stats temporarily unavailable"
			}
		}

		response.Neo4j = neo4jStatus

		// If Neo4j is the primary backend and connected, we're healthy
		if neo4jStatus.Connected && neo4jStatus.Error == "" {
			status = "healthy"
			httpStatus = http.StatusOK
			response.Status = status
		}
	}

	c.JSON(httpStatus, response)
}

// requireGraphReady is a helper that returns 503 if graph is not ready.
// Returns the graph if ready, or nil if not ready (response already sent).
func (s *Server) requireGraphReady(c *gin.Context) bool {
	if !s.graphService.IsReady() {
		progress := s.graphService.GetProgress()
		c.Header("Retry-After", "2")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "graph_loading",
			"message": "Graph is still loading, please retry in a few seconds",
			"stage":   progress.Stage,
		})
		return false
	}
	return true
}

// handleGetPage returns a page and its links.
// GET /api/v1/page/:title
func (s *Server) handleGetPage(c *gin.Context) {
	title := strings.ReplaceAll(c.Param("title"), "_", " ")
	if title == "" {
		RespondWithMissingParam(c, "title")
		return
	}

	// Use Neo4j if enabled
	if s.neo4jEnabled && s.neo4jClient != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		// Check if page exists
		exists, err := s.neo4jClient.PageExists(ctx, title)
		if err != nil {
			slog.Error("neo4j page exists check failed", "error", err, "title", title)
			RespondWithError(c, NewAPIError("internal_error", "Failed to query graph database", http.StatusInternalServerError))
			return
		}

		if !exists {
			RespondWithNotFound(c, "Page", title)
			return
		}

		// Get outgoing and incoming links
		outLinks, err := s.neo4jClient.GetOutLinks(ctx, title, 1000)
		if err != nil {
			slog.Error("neo4j get out links failed", "error", err, "title", title)
			outLinks = []string{}
		}

		inLinks, err := s.neo4jClient.GetInLinks(ctx, title, 1000)
		if err != nil {
			slog.Error("neo4j get in links failed", "error", err, "title", title)
			inLinks = []string{}
		}

		c.JSON(http.StatusOK, PageResponse{
			Title:       title,
			Links:       outLinks,
			LinkCount:   len(outLinks),
			InLinks:     inLinks,
			InLinkCount: len(inLinks),
		})
		return
	}

	// Fall back to in-memory graph
	if !s.requireGraphReady(c) {
		return
	}

	g, _ := s.graphService.GetGraph()
	node := g.GetNode(title)

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
	})
}

// handleFindPath finds the shortest path between two pages.
// GET /api/v1/path?from=X&to=Y&algorithm=bfs|bidirectional&backend=auto|neo4j|memory&max_depth=6
func (s *Server) handleFindPath(c *gin.Context) {
	from := strings.ReplaceAll(c.Query("from"), "_", " ")
	to := strings.ReplaceAll(c.Query("to"), "_", " ")

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

	// Backend selection: auto (prefer neo4j if available), neo4j (force neo4j), memory (force in-memory)
	backend := c.DefaultQuery("backend", "auto")
	if backend != "auto" && backend != "neo4j" && backend != "memory" {
		RespondWithValidationError(c, "backend", "must be 'auto', 'neo4j', or 'memory'")
		return
	}

	maxDepth := parseIntQuery(c, "max_depth", 6)
	if maxDepth < 1 || maxDepth > 20 {
		RespondWithValidationError(c, "max_depth", "must be between 1 and 20")
		return
	}

	start := time.Now()

	var result struct {
		Found    bool
		Path     []string
		Hops     int
		Explored int
	}

	// Determine which backend to use
	useNeo4j := false
	if backend == "neo4j" {
		if !s.neo4jEnabled || s.neo4jClient == nil {
			RespondWithError(c, NewAPIError("neo4j_unavailable", "Neo4j backend is not available", http.StatusServiceUnavailable))
			return
		}
		useNeo4j = true
	} else if backend == "auto" && s.neo4jEnabled && s.neo4jClient != nil {
		useNeo4j = true
	}

	if useNeo4j {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		var pathResult *neostore.PathResult
		var err error

		// Use bidirectional if requested
		if algorithm == "bidirectional" {
			pathResult, err = s.neo4jClient.FindShortestPathBidirectional(ctx, from, to, maxDepth)
			algorithm = "neo4j-bidirectional"
		} else {
			pathResult, err = s.neo4jClient.FindShortestPath(ctx, from, to, maxDepth)
			algorithm = "neo4j-bfs"
		}

		if err != nil {
			slog.Error("neo4j path query failed", "error", err, "from", from, "to", to)
			RespondWithError(c, NewAPIError("internal_error", "Failed to query graph database", http.StatusInternalServerError))
			return
		}

		if pathResult != nil {
			result.Found = true
			result.Path = pathResult.Titles
			result.Hops = pathResult.Length
			result.Explored = 0 // Neo4j doesn't track explored nodes
		} else {
			result.Found = false
			result.Path = nil
			result.Hops = 0
		}
	} else {
		// Use in-memory graph
		if !s.requireGraphReady(c) {
			return
		}

		g, _ := s.graphService.GetGraph()

		switch algorithm {
		case "bidirectional":
			r := g.FindPathBidirectional(c.Request.Context(), from, to)
			result.Found = r.Found
			result.Path = r.Path
			result.Hops = r.Hops
			result.Explored = r.Explored
			algorithm = "memory-bidirectional"
		default:
			r := g.FindPathWithLimit(c.Request.Context(), from, to, maxDepth)
			result.Found = r.Found
			result.Path = r.Path
			result.Hops = r.Hops
			result.Explored = r.Explored
			algorithm = "memory-bfs"
		}
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
	title := strings.ReplaceAll(c.Param("title"), "_", " ")
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

	// Use Neo4j if enabled
	if s.neo4jEnabled && s.neo4jClient != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Check if page exists
		exists, err := s.neo4jClient.PageExists(ctx, title)
		if err != nil {
			slog.Error("neo4j page exists check failed", "error", err, "title", title)
			RespondWithError(c, NewAPIError("internal_error", "Failed to query graph database", http.StatusInternalServerError))
			return
		}

		if !exists {
			RespondWithNotFound(c, "Page", title)
			return
		}

		// Get neighborhood from Neo4j
		neighborhood, err := s.neo4jClient.GetNeighborhood(ctx, title, depth, maxNodes)
		if err != nil {
			slog.Error("neo4j get neighborhood failed", "error", err, "title", title)
			RespondWithError(c, NewAPIError("internal_error", "Failed to query graph database", http.StatusInternalServerError))
			return
		}

		// Convert to response format
		nodes := make([]GraphNode, len(neighborhood.Nodes))
		for i, n := range neighborhood.Nodes {
			nodes[i] = GraphNode{
				ID:    n.Title,
				Title: n.Title,
				Hops:  n.Hops,
			}
		}

		edges := make([]GraphEdge, len(neighborhood.Edges))
		for i, e := range neighborhood.Edges {
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
		return
	}

	// Fall back to in-memory graph
	if !s.requireGraphReady(c) {
		return
	}

	g, _ := s.graphService.GetGraph()

	node := g.GetNode(title)
	if node == nil {
		RespondWithNotFound(c, "Page", title)
		return
	}

	subgraph := g.GetNeighborhood(c.Request.Context(), title, depth, maxNodes)
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

	// Generate job ID and record initial status
	jobID := "crawl_" + uuid.New().String()[:8]
	now := time.Now()
	job := &CrawlJobStatus{
		JobID:     jobID,
		Status:    "started",
		Title:     req.Title,
		StartedAt: now,
	}
	s.jobs.Store(jobID, job)

	// Start crawl in background
	go func() {
		crawlStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()

		slog.Info("starting crawl job",
			"job_id", jobID,
			"title", req.Title,
			"depth", req.Depth,
			"max_pages", req.MaxPages,
		)

		job.Status = "crawling"
		scr := scraper.New(s.cache, s.fetcher, scraper.Config{
			MaxDepth:  req.Depth,
			MaxPages:  req.MaxPages,
			BatchSize: 10,
			Workers:   30,
		})

		if _, err := scr.Crawl(ctx, []string{req.Title}); err != nil {
			slog.Error("crawl job failed", "job_id", jobID, "error", err)
			t := time.Now()
			job.Status = "failed"
			job.Error = err.Error()
			job.CompletedAt = &t
			return
		}

		slog.Info("crawl job completed, reloading graph", "job_id", jobID)

		if err := s.ReloadGraph(); err != nil {
			slog.Error("failed to reload graph after crawl", "job_id", jobID, "error", err)
		}

		if s.neo4jEnabled && s.neo4jSyncer != nil {
			job.Status = "syncing"
			slog.Info("syncing new crawl data to Neo4j", "job_id", jobID)
			syncStats, err := s.neo4jSyncer.IncrementalSync(ctx, crawlStart, 0)
			if err != nil {
				slog.Error("neo4j sync after crawl failed", "job_id", jobID, "error", err)
			} else {
				slog.Info("neo4j sync complete",
					"job_id", jobID,
					"nodes", syncStats.NodesCreated,
					"edges", syncStats.EdgesCreated,
					"duration", syncStats.Duration,
				)
			}
		}

		t := time.Now()
		job.Status = "done"
		job.CompletedAt = &t
	}()

	c.JSON(http.StatusAccepted, CrawlResponse{
		JobID:   jobID,
		Status:  "started",
		Message: "Crawl job started for '" + req.Title + "'",
	})
}

// handleGetCrawlJob returns the status of a background crawl job.
// GET /api/v1/crawl/:id
func (s *Server) handleGetCrawlJob(c *gin.Context) {
	jobID := c.Param("id")
	val, ok := s.jobs.Load(jobID)
	if !ok {
		RespondWithNotFound(c, "job", jobID)
		return
	}
	c.JSON(http.StatusOK, val.(*CrawlJobStatus))
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
