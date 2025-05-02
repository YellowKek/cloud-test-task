package loadbalancer

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
	Backends    []*Backend               // slice бэкендов
	Index       uint32                   // текущий индекс в массиве бэкендов
	RateLimiter *rateLimiter.RateLimiter // ограничитель запросов
	mu          sync.Mutex
}

// NextBackend проходит по массиву серверов и возвращает первый живой и с не пустым бакетом
// если у всех закончились токены, то флаг - true
func (lb *LoadBalancer) NextBackend() (*Backend, bool) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	tokensEmpty := false
	for i := 0; i < len(lb.Backends); i++ {
		backend := lb.Backends[lb.Index%uint32(len(lb.Backends))]
		lb.Index++
		if backend.Alive {
			clientId := backend.Url.Host
			if lb.RateLimiter.Allow(clientId) {
				return backend, false
			}
			tokensEmpty = true
		}
	}
	return nil, tokensEmpty
}

// HealthCheck каждые 10 секунд проверяет все бэкенды из массивы и проверяет доступны ли они
// если бэкенд стал доступен после того как он был помечен неработающим, то он помечается живым, и может быть использован снова
func (lb *LoadBalancer) HealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		lb.mu.Lock()
		for _, backend := range lb.Backends {
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
