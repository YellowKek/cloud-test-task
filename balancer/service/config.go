package service

import (
	"cloud-test-task/rateLimiter"
	"cloud-test-task/repository"
)

type ConfigService interface {
	Create(cfg rateLimiter.TokenBucketConfig) error
	GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error)
	Update(cfg rateLimiter.TokenBucketConfig) error
	Delete(clientId string) error
	GetAll() ([]rateLimiter.TokenBucketConfig, error)
}

type ConfigServiceImpl struct {
	r repository.ConfigRepository
}

func NewConfigService(r repository.ConfigRepository) ConfigService {
	return &ConfigServiceImpl{r: r}
}

func (s *ConfigServiceImpl) Create(cfg rateLimiter.TokenBucketConfig) error {
	return s.r.Create(cfg)
}

func (s *ConfigServiceImpl) Update(cfg rateLimiter.TokenBucketConfig) error {
	return s.r.Update(cfg)
}

func (s *ConfigServiceImpl) GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error) {
	return s.r.GetByClientId(clientId)
}

func (s *ConfigServiceImpl) Delete(clientId string) error {
	return s.r.Delete(clientId)
}

func (s *ConfigServiceImpl) GetAll() ([]rateLimiter.TokenBucketConfig, error) {
	return s.r.GetAll()
}
