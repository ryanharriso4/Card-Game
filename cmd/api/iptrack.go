package main

import (
	"sync"

	"golang.org/x/time/rate"
)

type IPLimiter struct {
	mu      sync.Mutex
	vistors map[string]*rate.Limiter
}

var ipTracker = &IPLimiter{
	vistors: make(map[string]*rate.Limiter),
}

// GetLimiter returns a unique limiter for each specific IP address
func (i *IPLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	limiter, exists := i.vistors[ip]
	if !exists {
		// 3 requests per second, max burst of 5 per unique IP
		limiter = rate.NewLimiter(rate.Limit(3), 5)
		i.vistors[ip] = limiter
	}

	return limiter
}
