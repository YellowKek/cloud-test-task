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

type ConfigHandler struct {
	s  service.ConfigService
	rl *rateLimiter.RateLimiter
}

func NewConfigHandler(s service.ConfigService, rl *rateLimiter.RateLimiter) *ConfigHandler {
	return &ConfigHandler{s: s, rl: rl}
}

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
		w.Write([]byte("fail to parse duration"))
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
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(fmt.Sprintf("%+v", bucket)))
}

func (h *ConfigHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	res, err := h.s.GetAll()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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

func (h *ConfigHandler) GetById(w http.ResponseWriter, r *http.Request) {
	clientId := r.URL.Path
	if clientId == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	res, err := h.s.GetByClientId(clientId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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

func (h *ConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var clientId string
	if err := json.NewDecoder(r.Body).Decode(&clientId); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request body"))
		return
	}

	err := h.rl.DeleteBucket(clientId)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	err = h.s.Delete(clientId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
