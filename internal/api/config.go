// Package api provides the HTTP API server for WikiGraph.
package api

import "time"

// Config holds API server configuration.
type Config struct {
	Host            string
	Port            int
	EnableCORS      bool
	CORSOrigins     []string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	RateLimit       float64 // requests per second per IP
	RateBurst       int     // burst capacity for rate limiter
	Production      bool    // set gin.ReleaseMode
}

// DefaultConfig returns sensible defaults for the API server.
var DefaultConfig = Config{
	Host:            "localhost",
	Port:            8080,
	EnableCORS:      true,
	CORSOrigins:     []string{"*"},
	ReadTimeout:     30 * time.Second,
	WriteTimeout:    30 * time.Second,
	ShutdownTimeout: 10 * time.Second,
	RateLimit:       100.0,
	RateBurst:       200,
	Production:      false,
}
