// Package middleware provides HTTP middleware for the API server.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header name for request ID.
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for request ID.
	RequestIDKey = "request_id"
)

// RequestID generates a unique request ID for each request.
// If the client provides X-Request-ID header, it uses that value.
// Otherwise, it generates a new UUID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Set(RequestIDKey, requestID)
		c.Header(RequestIDHeader, requestID)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from context.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(RequestIDKey); exists {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}
