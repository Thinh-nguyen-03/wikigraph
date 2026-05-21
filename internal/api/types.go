package api

import "time"

// ErrorResponse is the standard error response format.
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status     string       `json:"status"`
	Version    string       `json:"version"`
	Graph      GraphStats   `json:"graph"`
	GraphReady bool         `json:"graph_ready"`
	Neo4j      *Neo4jStatus `json:"neo4j,omitempty"`
}

// Neo4jStatus contains Neo4j connection status for health response.
type Neo4jStatus struct {
	Enabled   bool   `json:"enabled"`
	Connected bool   `json:"connected"`
	Nodes     int    `json:"nodes,omitempty"`
	Edges     int    `json:"edges,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GraphStats contains graph statistics for health response.
type GraphStats struct {
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

// PageResponse is returned by the page endpoint.
type PageResponse struct {
	Title       string    `json:"title"`
	Links       []string  `json:"links"`
	LinkCount   int       `json:"link_count"`
	InLinks     []string  `json:"in_links,omitempty"`
	InLinkCount int       `json:"in_link_count"`
	FetchedAt   time.Time `json:"fetched_at,omitempty"`
}

// PathResponse is returned by the path endpoint.
type PathResponse struct {
	Found      bool     `json:"found"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Path       []string `json:"path,omitempty"`
	Hops       int      `json:"hops"`
	Explored   int      `json:"explored"`
	Algorithm  string   `json:"algorithm"`
	DurationMs int64    `json:"duration_ms"`
}

// ConnectionsResponse is returned by the connections endpoint.
type ConnectionsResponse struct {
	Center    string      `json:"center"`
	Depth     int         `json:"depth"`
	Nodes     []GraphNode `json:"nodes"`
	Edges     []GraphEdge `json:"edges"`
	NodeCount int         `json:"node_count"`
	EdgeCount int         `json:"edge_count"`
}

// GraphNode represents a node in the subgraph response.
type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Hops  int    `json:"hops"`
}

// GraphEdge represents an edge in the subgraph response.
type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// CrawlRequest is the request body for starting a crawl job.
type CrawlRequest struct {
	Title    string `json:"title" binding:"required"`
	Depth    int    `json:"depth" binding:"min=1,max=50"`
	MaxPages int    `json:"max_pages" binding:"min=1,max=500000"`
}

// CrawlResponse is returned when a crawl job is started.
type CrawlResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CrawlJobStatus is returned by the job status endpoint.
type CrawlJobStatus struct {
	JobID       string     `json:"job_id"`
	Status      string     `json:"status"` // started | crawling | syncing | done | failed
	Title       string     `json:"title"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

// SimilarResponse is returned by the similar endpoint (Phase 3).
type SimilarResponse struct {
	Query     string        `json:"query"`
	Similar   []SimilarPage `json:"similar"`
	Count     int           `json:"count"`
	Threshold float64       `json:"threshold"`
}

// SimilarPage represents a similar page with its score.
type SimilarPage struct {
	Title string  `json:"title"`
	Score float64 `json:"score"`
}
