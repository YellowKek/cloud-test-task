package main

import (
	"cloud-test-task/rateLimiter"
	"cloud-test-task/util"
	"log"
	"net/url"
	"sync"
	"time"
)

type Backend struct {
	Url   url.URL
	Alive bool
}

type LoadBalancer struct {
	backends    []*Backend
	index       uint32
	rateLimiter *rateLimiter.RateLimiter
	mu          sync.Mutex
}

func (lb *LoadBalancer) NextBackend() *Backend {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for i := 0; i < len(lb.backends); i++ {
		backend := lb.backends[lb.index%uint32(len(lb.backends))]
		lb.index++
		if backend.Alive {
			return backend
		}

	}
	return nil
}

func (lb *LoadBalancer) HealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		lb.mu.Lock()
		for _, backend := range lb.backends {
			if util.IsBackendAlive(backend.Url) {
				backend.Alive = true
				log.Printf("Available backend: %v", backend)
			} else {
				backend.Alive = false
				log.Printf("Backend %v is down", backend)
			}
		}
		lb.mu.Unlock()
		<-ticker.C
	}
}
