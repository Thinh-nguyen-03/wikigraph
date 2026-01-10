package api

import (
	"github.com/Thinh-nguyen-03/wikigraph/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

// setupRouter configures the Gin router with middleware and routes.
func (s *Server) setupRouter() {
	// Set Gin mode based on configuration
	if s.config.Production {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router without default middleware (we use our own)
	router := gin.New()

	// Middleware chain (order matters!)
	// 1. Recovery - catch panics first
	router.Use(middleware.Recovery())

	// 2. Request ID - needed by all subsequent middleware
	router.Use(middleware.RequestID())

	// 3. Logging - logs all requests (uses request ID)
	router.Use(middleware.Logging())

	// 4. Timeout - set request deadline
	router.Use(middleware.Timeout(s.config.ReadTimeout))

	// 5. CORS - before rate limiting to allow preflight
	if s.config.EnableCORS {
		router.Use(middleware.CORS(s.config.CORSOrigins))
	}

	// 6. Rate limiting - per client IP
	router.Use(middleware.RateLimit(s.config.RateLimit, s.config.RateBurst))

	// Health endpoint (no versioning)
	router.GET("/health", s.handleHealth)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Page endpoints
		v1.GET("/page/:title", s.handleGetPage)

		// Path endpoints
		v1.GET("/path", s.handleFindPath)

		// Connections endpoints
		v1.GET("/connections/:title", s.handleGetConnections)

		// Crawl endpoints
		v1.POST("/crawl", s.handleCrawl)
	}

	s.router = router
}
