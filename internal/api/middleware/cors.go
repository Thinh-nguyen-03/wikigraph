package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSConfig holds CORS configuration options.
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int // preflight cache duration in seconds
}

// DefaultCORSConfig returns a permissive CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-ID",
		},
		ExposeHeaders: []string{
			"X-Request-ID",
			"Content-Length",
		},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
	}
}

// CORS returns a CORS middleware with the given allowed origins.
func CORS(allowOrigins []string) gin.HandlerFunc {
	cfg := DefaultCORSConfig()
	if len(allowOrigins) > 0 {
		cfg.AllowOrigins = allowOrigins
	}
	return CORSWithConfig(cfg)
}

// CORSWithConfig returns a CORS middleware with custom configuration.
func CORSWithConfig(cfg CORSConfig) gin.HandlerFunc {
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, o := range cfg.AllowOrigins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}

		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		c.Header("Access-Control-Expose-Headers", exposeHeaders)

		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if cfg.MaxAge > 0 {
			c.Header("Access-Control-Max-Age", string(rune(cfg.MaxAge)))
		}

		// Handle preflight requests
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
