package handler

import (
	"cloud-test-task/dto"
	"cloud-test-task/rateLimiter"
	"cloud-test-task/service"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ConfigHandler хэндлер для crud операций
type ConfigHandler struct {
	s  service.ConfigService
	rl *rateLimiter.RateLimiter
}

func NewConfigHandler(s service.ConfigService, rl *rateLimiter.RateLimiter) *ConfigHandler {
	return &ConfigHandler{s: s, rl: rl}
}

// Create создание бакета по переданному конфигу
func (h *ConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var dto dto.ConfigDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	duration, err := time.ParseDuration(dto.RefillInterval)
	if err != nil {
		log.Print("[ERROR] fail to parse duration: ", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail to parse duration: " + err.Error()))
		return
	}

	bucket := rateLimiter.TokenBucketConfig{Capacity: dto.Capacity, ClientId: dto.ClientId, RefillInterval: duration}
	err = h.s.Create(bucket)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = h.rl.AddBucket(bucket)
	err = h.rl.AddBucket(bucket)
	if err != nil {
		// Откатываем создание в БД если не удалось добавить в память
		_ = h.s.Delete(bucket.ClientId)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("%+v", bucket)))
}

// GetAll получение всех конфигов бакетов
func (h *ConfigHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	res := h.rl.GetAllBucketConfigs()
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

// GetById получение бакета по clientId
func (h *ConfigHandler) GetById(w http.ResponseWriter, r *http.Request) {
	clientId := r.URL.Path
	if clientId == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	res, err := h.rl.GetBucketConfig(clientId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

// Update обновление конфигурации бакета по clientId
func (h *ConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	var dto dto.ConfigDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	duration, err := time.ParseDuration(dto.RefillInterval)
	if err != nil {
		log.Print("[ERROR] fail to parse duration: ", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("fail to parse duration"))
		return
	}

	bucket := rateLimiter.TokenBucketConfig{Capacity: dto.Capacity, ClientId: dto.ClientId, RefillInterval: duration}
	err = h.s.Update(bucket)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	err = h.rl.AddBucket(bucket)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(bucket)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
}

// Delete удаление конфигурации и бакета по clientId
func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientId string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	err := h.rl.DeleteBucket(req.ClientId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	err = h.s.Delete(req.ClientId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
