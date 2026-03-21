package proxy

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/finops-platform/api-gateway/circuitbreaker"
	"github.com/gin-gonic/gin"
)

// ServiceConfig holds the upstream URL for a microservice.
type ServiceConfig struct {
	Name    string
	BaseURL string
}

// ReverseProxy forwards requests to an upstream service, protected by a circuit breaker.
type ReverseProxy struct {
	client   *http.Client
	registry *circuitbreaker.Registry
}

// New creates a ReverseProxy with a shared circuit breaker registry.
func New(registry *circuitbreaker.Registry) *ReverseProxy {
	return &ReverseProxy{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		registry: registry,
	}
}

// ForwardToPath proxies the request to a fixed upstream path, ignoring the incoming path.
// Useful for webhook endpoints that need path rewriting.
func (p *ReverseProxy) ForwardToPath(serviceName, upstreamBase, upstreamPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.URL.Path = upstreamPath
		p.Forward(serviceName, upstreamBase)(c)
	}
}

// ForwardStripPrefix returns a Gin handler that strips the given prefix from the
// request path before proxying to the upstream. Useful when the gateway mounts
// routes under /api/* but upstreams don't have that prefix.
func (p *ReverseProxy) ForwardStripPrefix(serviceName, upstreamBase, prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		original := c.Request.URL.Path
		stripped := strings.TrimPrefix(original, prefix)
		if stripped == "" {
			stripped = "/"
		}
		c.Request.URL.Path = stripped
		p.Forward(serviceName, upstreamBase)(c)
	}
}

// The service name is used to look up the circuit breaker.
func (p *ReverseProxy) Forward(serviceName, upstreamBase string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cb := p.registry.Get(serviceName)

		if !cb.Allow() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "service temporarily unavailable",
				"service": serviceName,
			})
			c.Abort()
			return
		}

		// Build upstream URL: base + original path + query string
		upstreamURL := upstreamBase + c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			upstreamURL += "?" + c.Request.URL.RawQuery
		}

		// Create the upstream request
		req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, c.Request.Body)
		if err != nil {
			cb.RecordFailure()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upstream request"})
			c.Abort()
			return
		}

		// Copy headers from the original request
		for key, values := range c.Request.Header {
			for _, v := range values {
				req.Header.Add(key, v)
			}
		}

		// Propagate authenticated user context as internal headers
		if userID, exists := c.Get("user_id"); exists {
			req.Header.Set("X-User-ID", fmt.Sprintf("%v", userID))
		}
		if accountID, exists := c.Get("account_id"); exists {
			req.Header.Set("X-Account-ID", fmt.Sprintf("%v", accountID))
		}

		resp, err := p.client.Do(req)
		if err != nil {
			cb.RecordFailure()
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "upstream service unavailable",
				"service": serviceName,
			})
			c.Abort()
			return
		}
		defer resp.Body.Close()

		// Record circuit breaker outcome
		if resp.StatusCode >= 500 {
			cb.RecordFailure()
		} else {
			cb.RecordSuccess()
		}

		// Copy response headers
		for key, values := range resp.Header {
			for _, v := range values {
				c.Header(key, v)
			}
		}

		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	}
}
