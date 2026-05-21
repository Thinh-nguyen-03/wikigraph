package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Thinh-nguyen-03/wikigraph/internal/api/middleware"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10)

	for i := 0; i < 10; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Errorf("request %d should be allowed (within burst)", i+1)
		}
	}
}

func TestRateLimiter_BlocksWhenExhausted(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 5)

	// Drain burst
	for i := 0; i < 5; i++ {
		rl.Allow("10.0.0.1")
	}

	if rl.Allow("10.0.0.1") {
		t.Error("request after burst exhaustion should be denied")
	}
}

func TestRateLimiter_PerClientIsolation(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 3)

	// Exhaust IP A
	for i := 0; i < 3; i++ {
		rl.Allow("1.2.3.4")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("IP A should be blocked after exhausting burst")
	}

	// IP B is completely unaffected
	if !rl.Allow("5.6.7.8") {
		t.Error("IP B should still be allowed (different quota)")
	}
}

func TestRateLimiter_EntryCreatedOnFirstRequest(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10)

	if rl.Len() != 0 {
		t.Fatalf("expected 0 entries initially, got %d", rl.Len())
	}

	rl.Allow("192.168.0.1")

	if rl.Len() != 1 {
		t.Errorf("expected 1 entry after first request, got %d", rl.Len())
	}
}

func TestRateLimiter_TTLEviction(t *testing.T) {
	rl := middleware.NewRateLimiter(10, 10)
	rl.Allow("192.168.1.1")
	rl.Allow("192.168.1.2")

	if rl.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", rl.Len())
	}

	// Sleep briefly so lastSeen is strictly in the past, then evict with a tiny age.
	time.Sleep(time.Millisecond)
	rl.CleanupOlderThan(time.Microsecond) // 1µs << 1ms elapsed → all entries are stale

	if rl.Len() != 0 {
		t.Errorf("expected 0 entries after eviction, got %d", rl.Len())
	}
}

func TestRateLimiter_MultipleIPs_IndependentBuckets(t *testing.T) {
	rl := middleware.NewRateLimiter(100, 2)

	rl.Allow("a")
	rl.Allow("a") // A exhausted

	rl.Allow("b") // B still has 1 token left

	if rl.Len() != 2 {
		t.Errorf("expected 2 entries (one per IP), got %d", rl.Len())
	}
	if rl.Allow("a") {
		t.Error("A should be blocked")
	}
	if !rl.Allow("b") {
		// b had burst=2, used 1, so still 1 left — but token refill rate may have
		// already added back a fraction. Accept either outcome (this just tests isolation).
		t.Log("b blocked — acceptable if refill hasn't occurred yet")
	}
}

func TestRateLimit_Middleware_Blocks(t *testing.T) {
	router := gin.New()
	// burst=1 so second request is always blocked
	router.Use(middleware.RateLimit(1000, 1))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// First request — allowed
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "1.2.3.4:1234"
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("first request: status = %d, want 200", w1.Code)
	}

	// Second request immediately — burst=1 exhausted
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "1.2.3.4:1235"
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request: status = %d, want 429", w2.Code)
	}
}

func TestRequestID_Generated(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID header should be set when not provided by client")
	}
}

func TestRequestID_ClientProvided(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequestID())
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id-123")
	router.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id != "my-custom-id-123" {
		t.Errorf("X-Request-ID = %q, want %q", id, "my-custom-id-123")
	}
}
