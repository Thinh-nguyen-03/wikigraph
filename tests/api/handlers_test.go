package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Thinh-nguyen-03/wikigraph/internal/api"
	"github.com/Thinh-nguyen-03/wikigraph/internal/graph"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// testConfig returns a permissive config suitable for handler tests.
func testConfig() api.Config {
	cfg := api.DefaultConfig
	cfg.RateLimit = 10000
	cfg.RateBurst = 10000
	cfg.EnableCORS = false
	return cfg
}

// newReadyServer builds a server with the given graph pre-loaded and ready.
func newReadyServer(t *testing.T, g *graph.Graph) http.Handler {
	t.Helper()
	s := api.New(g, nil, nil, testConfig())
	return s.Router()
}

// newLoadingServer builds a server where the graph is not yet ready
// (StateUninitialized — IsReady() returns false).
func newLoadingServer(t *testing.T) http.Handler {
	t.Helper()
	gs := api.NewGraphService(nil, api.GraphServiceConfig{})
	s := api.NewWithGraphService(gs, nil, nil, testConfig())
	return s.Router()
}

// do performs a request against the given handler and returns the recorder.
func do(handler http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req, _ := http.NewRequest(method, path, reqBody)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// smallGraph builds: A→B→C  and  A→C (A has two out-links, C has two in-links).
func smallGraph() *graph.Graph {
	g := graph.New()
	g.AddEdge("A", "B")
	g.AddEdge("B", "C")
	g.AddEdge("A", "C")
	return g
}

// ---- /health ----------------------------------------------------------------

func TestHealth_Ready(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/health", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp api.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("status = %q, want 'healthy'", resp.Status)
	}
	if !resp.GraphReady {
		t.Error("graph_ready should be true")
	}
	if resp.Version == "" {
		t.Error("version should be set")
	}
}

func TestHealth_GraphNotReady(t *testing.T) {
	h := newLoadingServer(t)
	w := do(h, "GET", "/health", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp api.HealthResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.GraphReady {
		t.Error("graph_ready should be false when not loaded")
	}
}

// ---- GET /api/v1/page/:title -----------------------------------------------

func TestGetPage_Found(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/page/A", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp api.PageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Title != "A" {
		t.Errorf("title = %q, want 'A'", resp.Title)
	}
	if resp.LinkCount != 2 {
		t.Errorf("link_count = %d, want 2", resp.LinkCount)
	}
}

func TestGetPage_NotFound(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/page/Nonexistent", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}

	var resp api.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "not_found" {
		t.Errorf("error = %q, want 'not_found'", resp.Error)
	}
}

func TestGetPage_TitleNormalization(t *testing.T) {
	g := graph.New()
	g.AddEdge("Albert Einstein", "Physics")

	h := newReadyServer(t, g)

	// Underscore in URL → space in lookup
	w := do(h, "GET", "/api/v1/page/Albert_Einstein", "")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (underscore should be normalised to space)", w.Code)
	}

	var resp api.PageResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Title != "Albert Einstein" {
		t.Errorf("title = %q, want 'Albert Einstein'", resp.Title)
	}
}

func TestGetPage_GraphNotReady(t *testing.T) {
	h := newLoadingServer(t)
	w := do(h, "GET", "/api/v1/page/Anything", "")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header should be set when graph is loading")
	}
}

// ---- GET /api/v1/path -------------------------------------------------------

func TestFindPath_Found(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp api.PathResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Found {
		t.Error("found should be true")
	}
	if resp.From != "A" || resp.To != "C" {
		t.Errorf("from=%q to=%q, want A and C", resp.From, resp.To)
	}
	if resp.Hops < 1 {
		t.Errorf("hops = %d, want >= 1", resp.Hops)
	}
}

func TestFindPath_NoPath(t *testing.T) {
	g := graph.New()
	g.AddEdge("A", "B")
	g.AddEdge("C", "D") // disconnected

	h := newReadyServer(t, g)
	w := do(h, "GET", "/api/v1/path?from=A&to=D", "")

	// No path is NOT a 404 — it's a 200 with found=false
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp api.PathResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Found {
		t.Error("found should be false for disconnected nodes")
	}
}

func TestFindPath_MissingFrom(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?to=C", "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var resp api.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "missing_parameter" {
		t.Errorf("error = %q, want 'missing_parameter'", resp.Error)
	}
}

func TestFindPath_MissingTo(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A", "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestFindPath_InvalidAlgorithm(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&algorithm=dijkstra", "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	var resp api.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid_parameter" {
		t.Errorf("error = %q, want 'invalid_parameter'", resp.Error)
	}
}

func TestFindPath_MaxDepthTooLow(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&max_depth=0", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestFindPath_MaxDepthTooHigh(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&max_depth=21", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestFindPath_Bidirectional(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&algorithm=bidirectional", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp api.PathResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Algorithm != "memory-bidirectional" {
		t.Errorf("algorithm = %q, want 'memory-bidirectional'", resp.Algorithm)
	}
}

func TestFindPath_Neo4jForced_Unavailable(t *testing.T) {
	// Server has no Neo4j client configured.
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&backend=neo4j", "")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when neo4j unavailable", w.Code)
	}
	var resp api.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "neo4j_unavailable" {
		t.Errorf("error = %q, want 'neo4j_unavailable'", resp.Error)
	}
}

func TestFindPath_InvalidBackend(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&backend=spark", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- GET /api/v1/connections/:title ----------------------------------------

func TestGetConnections_Found(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/connections/A?depth=1", "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp api.ConnectionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Center != "A" {
		t.Errorf("center = %q, want 'A'", resp.Center)
	}
	if resp.NodeCount < 2 {
		t.Errorf("node_count = %d, want >= 2 (A + at least one neighbour)", resp.NodeCount)
	}
	if resp.EdgeCount < 1 {
		t.Errorf("edge_count = %d, want >= 1", resp.EdgeCount)
	}
}

func TestGetConnections_NotFound(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/connections/Nobody", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetConnections_DepthTooLow(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/connections/A?depth=0", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGetConnections_DepthTooHigh(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/connections/A?depth=6", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGetConnections_MaxNodesTooHigh(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/connections/A?max_nodes=10001", "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- GET /api/v1/crawl/:id --------------------------------------------------

func TestGetCrawlJob_NotFound(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/crawl/unknown_job_id", "")

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	var resp api.ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "not_found" {
		t.Errorf("error = %q, want 'not_found'", resp.Error)
	}
}

// ---- POST /api/v1/crawl -----------------------------------------------------

func TestHandleCrawl_MissingTitle(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	// title is binding:"required" so empty body returns 400 before any crawl starts.
	w := do(h, "POST", "/api/v1/crawl", `{}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (title required)", w.Code)
	}
}

func TestHandleCrawl_InvalidJSON(t *testing.T) {
	h := newReadyServer(t, smallGraph())
	w := do(h, "POST", "/api/v1/crawl", `not-json`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (invalid JSON)", w.Code)
	}
}

// ---- parseIntQuery (tested via endpoint params) -----------------------------

func TestParseIntQuery_Default(t *testing.T) {
	// max_depth omitted → default 6, which is valid → 200
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C", "")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (default max_depth used)", w.Code)
	}
}

func TestParseIntQuery_NonNumeric(t *testing.T) {
	// Non-numeric max_depth falls back to default (6), which is valid → 200
	h := newReadyServer(t, smallGraph())
	w := do(h, "GET", "/api/v1/path?from=A&to=C&max_depth=abc", "")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (non-numeric falls back to default)", w.Code)
	}
}
