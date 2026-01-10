package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout returns a middleware that sets a timeout on the request context.
// If the handler doesn't complete within the timeout, the context is cancelled.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Replace the request context
		c.Request = c.Request.WithContext(ctx)

		// Create a channel to signal completion
		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// Handler completed normally
			return
		case <-ctx.Done():
			// Timeout occurred
			if ctx.Err() == context.DeadlineExceeded {
				requestID := GetRequestID(c)

				c.AbortWithStatusJSON(http.StatusGatewayTimeout, gin.H{
					"error":      "request_timeout",
					"message":    "Request took too long to process",
					"request_id": requestID,
				})
			}
		}
	}
}
