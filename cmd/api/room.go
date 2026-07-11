package main

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Room struct {
	Sequence    uint64
	ID          string
	Players     map[string]*Client
	Game        *GameState
	mu          sync.Mutex
	CancelTimer context.CancelFunc
	limiter     *rate.Limiter
}

func (r *Room) AllowAction() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	allowed := r.limiter.Allow()

	log.Printf(
		"room=%p limiter=%p allowed=%v time=%v",
		r,
		r.limiter,
		allowed,
		time.Now(),
	)

	return allowed
}
