package main

import (
	"cloud-test-task/config"
	"cloud-test-task/db"
	handler2 "cloud-test-task/handler"
	"cloud-test-task/rateLimiter"
	"cloud-test-task/repository"
	service2 "cloud-test-task/service"
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
	"strings"
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
			if !lb.rateLimiter.Allow(clientId) {
				req.URL.Host = "rate-limited|" + clientId
				return
			}
			log.Printf("Forwarding request to %v", backend.Url)
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.URL.Host == "" {
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Backends are not available"))
			} else if len(r.URL.Host) > 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Rate limit exceeded"))
				log.Printf("[INFO] bucket is empty for client %s", strings.Split(r.URL.Host, "|")[1])
				return
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

	mux := http.NewServeMux()

	proxy := NewReverseProxy(lb)

	mux.Handle("/", proxy)
	repo := repository.NewConfigRepository(db)
	service := service2.NewConfigService(repo)
	handler := handler2.NewConfigHandler(service, rl)
	mux.HandleFunc("/rate-limits", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.GetAll(w, r)
		case http.MethodPost:
			handler.Create(w, r)
		case http.MethodPut:
			handler.Update(w, r)
		case http.MethodDelete:
			handler.Delete(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.Handle("/rate-limits/", http.StripPrefix("/rate-limits/", http.HandlerFunc(handler.GetById)))

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.Port),
		Handler: mux,
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
