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

// Refill каждые промежутки времени, указанные в конфиге обновляет количество токенов в бакете
func (tb *TokenBucket) Refill() {
	ticker := time.NewTicker(tb.refillInterval)
	for {
		tb.mu.Lock()
		tb.tokens = tb.capacity
		tb.mu.Unlock()
		<-ticker.C
	}
}

// Allow проверяет количество токенов в бакете
// если токенов хватает, то уменьшает их кол-во на 1 и возвращает true
// если не хватает то возвращает false
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
	ClientId       string        `json:"client_id"`
	Capacity       int           `json:"capacity"`
	RefillInterval time.Duration `json:"refill_interval"`
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
	// Загружаем настройки клиентов из стандартного конфига, потом если есть пользовательские настройки, одновляем их
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

// AddBucket добавляет бакет в map
func (rl *RateLimiter) AddBucket(bucket TokenBucketConfig) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Создаем/обновляем бакет независимо от его существования
	rl.buckets[bucket.ClientId] = NewTokenBucket(bucket.Capacity, bucket.RefillInterval)
	rl.config[bucket.ClientId] = &bucket

	log.Printf("[DEBUG] added/updated bucket %s: %+v", bucket.ClientId, bucket)
	return nil
}

// DeleteBucket удаляет бакет из map
func (rl *RateLimiter) DeleteBucket(bucket string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	_, bucketExists := rl.buckets[bucket]
	_, configExists := rl.config[bucket]

	if !bucketExists && !configExists {
		return fmt.Errorf("backend %s does not exists", bucket)
	}

	if bucketExists {
		delete(rl.buckets, bucket)
	}
	if configExists {
		delete(rl.config, bucket)
	}

	return nil
}

// GetBucketConfig возвращает конфиг переданного бакета
func (rl *RateLimiter) GetBucketConfig(bucket string) (*TokenBucketConfig, error) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	bucketConfig, ok := rl.config[bucket]
	if !ok {
		return nil, fmt.Errorf("backend %s does not exists", bucket)
	}
	return bucketConfig, nil
}

// GetAllBucketConfigs возвращает конфиги всех бакетов
func (rl *RateLimiter) GetAllBucketConfigs() []TokenBucketConfig {
	res := make([]TokenBucketConfig, 0)
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	for _, cfg := range rl.config {
		res = append(res, *cfg)
	}
	return res
}

// loadDefaultConfig загружает стандартный конфиг из файла
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
			RefillInterval: cfg.RefillInterval,
			ClientId:       clientURL.Host,
		}
	}
	rl.config = cfgMap
	return nil
}

// loadClientConfig загружает конфиг из бд
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

// Allow если бакет не создан, то создает его и вызывает TokenBucket.Allow
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
