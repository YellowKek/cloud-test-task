package rateLimiter

import (
	"database/sql"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity       int
	tokens         int
	lastRefill     time.Time
	refillInterval time.Duration
	mu             sync.Mutex
}

func NewTokenBucket(capacity int, refillInterval time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:       capacity,
		tokens:         capacity,
		lastRefill:     time.Now(),
		refillInterval: refillInterval,
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	elapsed := time.Now().Sub(tb.lastRefill)

	if elapsed >= tb.refillInterval {
		refillCount := int(elapsed/tb.refillInterval) * tb.capacity
		tb.tokens += refillCount
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = time.Now()
	}

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

type TokenBucketConfig struct {
	ClientID       string
	Capacity       int
	RefillInterval time.Duration
}

type RateLimiter struct {
	buckets map[string]*TokenBucket
	config  TokenBucketConfig
	mu      sync.RWMutex
	db      *sql.DB
}

func NewRateLimiter(db *sql.DB) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		db:      db,
	}

	// Загружаем настройки клиентов из БД при старте, если нет данных в бд то загружаем из дефолтного конфига
	if err := rl.loadClientConfig(); err != nil {
		err := rl.loadDefaultConfig()
		log.Print("loading default config")
		if err != nil {
			log.Fatal("load bucket config failed", err)
			return nil
		}
	}
	log.Printf("clientId: %s, capacity: %d, refill int: %d", rl.config.ClientID, rl.config.Capacity, rl.config.RefillInterval)
	return rl
}

func (rl *RateLimiter) loadDefaultConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	var cfg = TokenBucketConfig{}
	cfg.Capacity = viper.GetInt("rate_limiting.capacity")
	cfg.RefillInterval = viper.GetDuration("rate_limiting.refill_interval")
	rl.config = cfg
	return nil
}

func (rl *RateLimiter) loadClientConfig() error {
	rows, err := rl.db.Query("SELECT client_id, capacity, refill_interval FROM client_configs")
	if err != nil {
		log.Printf("Failed to load client configs: %v", err)
		return err
	}
	defer rows.Close()

	loaded := false
	for rows.Next() {
		loaded = true
		var cfg TokenBucketConfig
		var interval int
		err := rows.Scan(&cfg.ClientID, &cfg.Capacity, &interval)
		if err != nil {
			log.Printf("Failed to scan client config: %v", err)
			continue
		}
		cfg.RefillInterval = time.Duration(interval) * time.Second

		rl.config = cfg
		rl.buckets[cfg.ClientID] = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
	}

	if !loaded {
		return fmt.Errorf("no client configurations found in database")
	}
	return nil
}

func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		bucket = NewTokenBucket(rl.config.Capacity, rl.config.RefillInterval)
		rl.buckets[clientID] = bucket
		rl.mu.Unlock()
	}

	return bucket.Allow()
}
