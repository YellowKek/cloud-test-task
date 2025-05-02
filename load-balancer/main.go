package main

import (
	"cloud-test-task/config"
	"cloud-test-task/db"
	"cloud-test-task/rateLimiter"
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
	"syscall"
	"time"
)

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

			clientId := req.URL.Host
			log.Print("clientId: ", clientId)
			if !lb.rateLimiter.Allow(clientId) {
				req.URL.Host = "rate-limited"
				return
			}
			log.Printf("Forwarding request to %v", backend.Url)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.URL.Host == "rate-limited" {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Rate limit exceeded"))
				return
			}
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

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	db, err := db.NewDB(config.Db)
	if err != nil {
		log.Fatal("failed to connect to db: ", err)
	}

	backends := make([]*Backend, 0)
	for _, backendURL := range config.Backends {
		temp, err := url.Parse(backendURL)
		if err != nil {
			log.Fatalf("Failed to parse backend URL from config: %v", err)
		}
		backends = append(backends, &Backend{Url: *temp, Alive: false})
	}

	rl := rateLimiter.NewRateLimiter(db)

	lb := &LoadBalancer{
		backends:    backends,
		index:       0,
		rateLimiter: rl,
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
