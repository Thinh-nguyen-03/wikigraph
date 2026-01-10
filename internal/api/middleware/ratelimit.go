package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter manages per-client rate limiters.
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	limit    rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter with the specified
// requests per second limit and burst capacity.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		limit:    rate.Limit(rps),
		burst:    burst,
	}
}

// getLimiter returns the rate limiter for the given key (usually client IP).
// It creates a new limiter if one doesn't exist.
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	// Fast path: check if limiter exists with read lock
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Slow path: create new limiter with write lock
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.limit, rl.burst)
	rl.limiters[key] = limiter
	return limiter
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	return rl.getLimiter(key).Allow()
}

// RateLimit returns a middleware that rate limits requests per client IP.
// rps is the requests per second limit, burst is the burst capacity.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	rl := NewRateLimiter(rps, burst)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		if !rl.Allow(clientIP) {
			requestID := GetRequestID(c)

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "rate_limit_exceeded",
				"message":    "Too many requests. Please slow down.",
				"request_id": requestID,
			})
			return
		}

		c.Next()
	}
}
