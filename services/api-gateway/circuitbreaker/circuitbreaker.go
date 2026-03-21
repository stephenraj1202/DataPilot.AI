package circuitbreaker

import (
	"fmt"
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // normal operation
	StateOpen                  // failing, reject requests
	StateHalfOpen              // testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern per service
type CircuitBreaker struct {
	mu               sync.Mutex
	state            State
	failureCount     int
	failureThreshold int
	timeout          time.Duration
	lastFailureTime  time.Time
}

// New creates a CircuitBreaker with the given failure threshold and open timeout.
func New(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		timeout:          timeout,
	}
}

// Allow returns true if the request should be allowed through.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has elapsed — transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.timeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess resets the circuit breaker on a successful call.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = StateClosed
}

// RecordFailure increments the failure counter and opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
	}
}

// State returns the current state as a string.
func (cb *CircuitBreaker) StateName() string {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	}
	return "unknown"
}

// Registry holds a circuit breaker per named service.
type Registry struct {
	mu        sync.RWMutex
	breakers  map[string]*CircuitBreaker
	threshold int
	timeout   time.Duration
}

// NewRegistry creates a Registry with shared threshold and timeout settings.
func NewRegistry(failureThreshold int, timeout time.Duration) *Registry {
	return &Registry{
		breakers:  make(map[string]*CircuitBreaker),
		threshold: failureThreshold,
		timeout:   timeout,
	}
}

// Get returns (or lazily creates) the circuit breaker for the named service.
func (r *Registry) Get(service string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[service]
	r.mu.RUnlock()
	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check after acquiring write lock
	if cb, ok = r.breakers[service]; ok {
		return cb
	}
	cb = New(r.threshold, r.timeout)
	r.breakers[service] = cb
	return cb
}

// Status returns a map of service name → state string for health reporting.
func (r *Registry) Status() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]string, len(r.breakers))
	for name, cb := range r.breakers {
		out[name] = cb.StateName()
	}
	return out
}

// Reset forces the circuit breaker back to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = StateClosed
}

// ErrCircuitOpen is returned when a circuit breaker is open.
var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")
