package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Backend struct {
	Url   url.URL
	Alive bool
}

// LoadBalancer TODO сделать два массива живые и мертвые
type LoadBalancer struct {
	backends []*Backend
	index    uint32
	mu       sync.Mutex
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

func isBackendAlive(url url.URL) bool {
	resp, err := http.Get(url.String() + "/health")
	if err != nil {
		log.Printf("Backend %v is down: %v", url, err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func NewReverseProxy(lb *LoadBalancer) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			backend := lb.NextBackend()
			if backend == nil {
				req.URL.Host = ""
				return
			}
			req.URL.Scheme = backend.Url.Scheme
			req.URL.Host = backend.Url.Host
			log.Printf("Forwarding request to %v", backend.Url)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.URL.Host == "" {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Backends are not available"))
			} else {
				w.WriteHeader(r.Response.StatusCode)
				_, err := io.Copy(w, r.Response.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
			log.Printf("Error: %v", err)
		},
	}
}

func (lb *LoadBalancer) HealthCheck() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		lb.mu.Lock()
		for _, backend := range lb.backends {
			if isBackendAlive(backend.Url) {
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

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	backends := make([]*Backend, 0)
	for _, backendURL := range config.Backends {
		temp, err := url.Parse(backendURL)
		if err != nil {
			log.Fatalf("Failed to parse backend URL from config: %v", err)
		}
		backends = append(backends, &Backend{Url: *temp, Alive: false})
	}

	lb := &LoadBalancer{
		backends: backends,
		index:    0,
	}

	go lb.HealthCheck()

	proxy := NewReverseProxy(lb)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: proxy,
	}

	log.Printf("Load balancer started on :%s", config.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatal("Server shutdown error:", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("Server error:", err)
	}
}
