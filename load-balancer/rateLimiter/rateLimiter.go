package rateLimiter

import (
	"database/sql"
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net/url"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity       int
	tokens         int
	refillInterval time.Duration
	mu             sync.Mutex
}

func NewTokenBucket(capacity int, refillInterval time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:       capacity,
		tokens:         capacity,
		refillInterval: refillInterval,
	}
}

func (tb *TokenBucket) Refill() {
	ticker := time.NewTicker(tb.refillInterval)
	for {
		tb.mu.Lock()
		tb.tokens = tb.capacity
		tb.mu.Unlock()
		<-ticker.C
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

type TokenBucketConfig struct {
	ClientId       string
	Capacity       int
	RefillInterval time.Duration
}

type RateLimiter struct {
	config  map[string]*TokenBucketConfig
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
	db      *sql.DB
}

func NewRateLimiter(db *sql.DB) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*TokenBucket),
		db:      db,
	}
	// Загружаем настройки клиентов из БД при старте, если нет данных в бд то загружаем из дефолтного конфига
	err := rl.loadDefaultConfig()
	log.Print("loading default config")
	if err != nil {
		log.Fatal("load bucket config failed", err)
		return nil
	}
	if err := rl.loadClientConfig(); err != nil {
		log.Print("[ERROR] load bucket config from db failed", err)
	}

	return rl
}

func (rl *RateLimiter) AddBucket(bucket TokenBucketConfig) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.buckets[bucket.ClientId]; ok {
		rl.buckets[bucket.ClientId] = NewTokenBucket(bucket.Capacity, bucket.RefillInterval)
	} else {
		return fmt.Errorf("backend %s does not exists", bucket.ClientId)
	}
	log.Print("[DEBUG] add bucket", bucket.Capacity, bucket.RefillInterval)
	return nil
}

func (rl *RateLimiter) DeleteBucket(bucket string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.buckets[bucket]; ok {
		delete(rl.buckets, bucket)
	} else {
		return fmt.Errorf("backend %s does not exists", bucket)
	}
	for _, bucket := range rl.buckets {
		log.Print("[DEBUG] delete bucket", *bucket)
	}
	return nil
}

func (rl *RateLimiter) loadDefaultConfig() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	cfgMap := make(map[string]*TokenBucketConfig)
	var cfg TokenBucketConfig
	cfg.Capacity = viper.GetInt("rate_limiting.capacity")
	cfg.RefillInterval = viper.GetDuration("rate_limiting.refill_interval")

	temp := viper.GetStringSlice("backends")
	for _, clientId := range temp {
		clientURL, err := url.Parse(clientId)
		if err != nil {
			return fmt.Errorf("parse client id failed: %v", err)
		}
		cfgMap[clientURL.Host] = &TokenBucketConfig{
			Capacity:       cfg.Capacity,
			RefillInterval: cfg.RefillInterval}
	}
	rl.config = cfgMap
	return nil
}

func (rl *RateLimiter) loadClientConfig() error {
	rows, err := rl.db.Query("SELECT client_id, capacity, refill_interval FROM client_configs")
	if err != nil {
		log.Printf("Failed to load client configs: %v", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cfg TokenBucketConfig
		var interval int
		err := rows.Scan(&cfg.ClientId, &cfg.Capacity, &interval)
		if err != nil {
			log.Printf("Failed to scan client config: %v", err)
			continue
		}
		cfg.RefillInterval = time.Duration(interval) * time.Second

		rl.config[cfg.ClientId] = &cfg
		rl.buckets[cfg.ClientId] = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
	}
	err = rows.Err()
	if err != nil {
		return err
	}

	return nil
}

func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.RLock()
	bucket, bucketExists := rl.buckets[clientID]
	cfg, configExists := rl.config[clientID]
	rl.mu.RUnlock()

	if !bucketExists {
		if !configExists {
			log.Fatalf("config for client %s not exists", clientID)
		}
		rl.mu.Lock()
		bucket = NewTokenBucket(cfg.Capacity, cfg.RefillInterval)
		rl.buckets[clientID] = bucket
		go bucket.Refill()
		rl.mu.Unlock()
	}

	return bucket.Allow()
}
