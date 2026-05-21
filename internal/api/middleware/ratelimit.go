package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages per-client rate limiters with TTL-based eviction so the
// internal map does not grow unbounded on public-facing servers.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	limit   rate.Limit
	burst   int
}

// NewRateLimiter creates a new rate limiter and starts a background goroutine
// that evicts entries inactive for more than 5 minutes.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*limiterEntry),
		limit:   rate.Limit(rps),
		burst:   burst,
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	e, ok := rl.entries[key]
	if !ok {
		e = &limiterEntry{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.entries[key] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// Len returns the number of tracked client entries.
func (rl *RateLimiter) Len() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.entries)
}

// CleanupOlderThan removes entries not seen within the given duration.
// Exported so tests can trigger eviction deterministically without waiting.
func (rl *RateLimiter) CleanupOlderThan(age time.Duration) {
	cutoff := time.Now().Add(-age)
	rl.mu.Lock()
	for key, e := range rl.entries {
		if e.lastSeen.Before(cutoff) {
			delete(rl.entries, key)
		}
	}
	rl.mu.Unlock()
}

// cleanupLoop runs every minute and removes entries not seen in 5 minutes.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.CleanupOlderThan(5 * time.Minute)
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	return rl.getLimiter(key).Allow()
}

// RateLimit returns a middleware that rate limits requests per client IP.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	rl := NewRateLimiter(rps, burst)

	return func(c *gin.Context) {
		if !rl.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":      "rate_limit_exceeded",
				"message":    "Too many requests. Please slow down.",
				"request_id": GetRequestID(c),
			})
			return
		}
		c.Next()
	}
}
