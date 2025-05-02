package main

import (
	"cloud-test-task/config"
	"cloud-test-task/db"
	handler2 "cloud-test-task/handler"
	"cloud-test-task/load-balancer"
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
	"syscall"
	"time"
)

// NewReverseProxy основная функция проксирования запросов
// если в бакете для текущего клиента есть токены, то запросы передаются в него, если нет, то передаются на следующий сервер
func NewReverseProxy(lb *loadbalancer.LoadBalancer) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			backend, tokensEmpty := lb.NextBackend()
			if backend == nil {
				if tokensEmpty {
					req.URL.Host = "rate-limited"
					return
				}
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
			} else if len(r.URL.Host) > 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte("Rate limit exceeded"))
				log.Printf("[INFO] all bucketы are empty")
				return
			} else {
				w.WriteHeader(r.Response.StatusCode)
				_, err := io.Copy(w, r.Response.Body)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}
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

	// заполнение массива данными из конфига, но в правильном формате
	backends := make([]*loadbalancer.Backend, 0)
	for _, backendURL := range config.Backends {
		temp, err := url.Parse(backendURL)
		if err != nil {
			log.Fatalf("Failed to parse backend URL from config: %v", err)
		}
		backends = append(backends, &loadbalancer.Backend{Url: *temp, Alive: false})
	}

	rl := rateLimiter.NewRateLimiter(db)

	lb := &loadbalancer.LoadBalancer{
		Backends:    backends,
		Index:       0,
		RateLimiter: rl,
	}

	go lb.HealthCheck() // запуск функции для проверка доступности бэкендов

	mux := http.NewServeMux()

	proxy := NewReverseProxy(lb)

	mux.Handle("/", proxy)
	repo := repository.NewConfigRepository(db)
	service := service2.NewConfigService(repo)
	handler := handler2.NewConfigHandler(service, rl)
	// crud эндпоинты
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

	// graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, os.Kill)
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
