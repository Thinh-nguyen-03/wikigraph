package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// Recovery recovers from panics and returns a JSON error response.
// It logs the panic with stack trace for debugging.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(c)

				// Log the panic with stack trace
				slog.Error("panic recovered",
					"request_id", requestID,
					"error", err,
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"stack", string(debug.Stack()),
				)

				// Return a generic error response
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "internal_error",
					"message":    "An unexpected error occurred",
					"request_id": requestID,
				})
			}
		}()

		c.Next()
	}
}
