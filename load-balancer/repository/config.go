package repository

import (
	"cloud-test-task/rateLimiter"
	"database/sql"
	"log"
)

type ConfigRepository interface {
	Create(cfg rateLimiter.TokenBucketConfig) error
	GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error)
	Update(newCfg rateLimiter.TokenBucketConfig) error
	Delete(clientId string) error
	GetAll() ([]rateLimiter.TokenBucketConfig, error)
}

type ConfigRepositoryImpl struct {
	db *sql.DB
}

func NewConfigRepository(db *sql.DB) ConfigRepository {
	return &ConfigRepositoryImpl{db: db}
}

func (c *ConfigRepositoryImpl) Create(cfg rateLimiter.TokenBucketConfig) error {
	query := `INSERT INTO client_configs (client_id, capacity, refill_interval) VALUES ($1, $2, $3)`
	_, err := c.db.Exec(query, cfg.ClientId, cfg.Capacity, cfg.RefillInterval)
	if err != nil {
		log.Print("[ERROR] insert config err: ", err)
		return err
	}
	return nil
}

func (c *ConfigRepositoryImpl) GetByClientId(clientId string) (rateLimiter.TokenBucketConfig, error) {
	query := `SELECT client_id, capacity, refill_interval FROM client_configs WHERE client_id = $1`
	var res rateLimiter.TokenBucketConfig
	err := c.db.QueryRow(query, clientId).Scan(&res.ClientId, &res.Capacity, &res.RefillInterval)
	if err != nil {
		log.Print("[ERROR] get config err: ", err)
		return res, err
	}
	return res, nil
}

func (c *ConfigRepositoryImpl) Update(newCfg rateLimiter.TokenBucketConfig) error {
	cfg, err := c.GetByClientId(newCfg.ClientId)
	if err != nil {
		return err
	}

	query := `UPDATE client_configs SET capacity = $1, refill_interval = $2 WHERE client_id = $3`
	_, err = c.db.Exec(query, cfg.Capacity, cfg.RefillInterval, newCfg.ClientId)
	if err != nil {
		log.Print("[ERROR] update config err: ", err)
		return err
	}
	return nil
}

func (c *ConfigRepositoryImpl) Delete(clientId string) error {
	query := `DELETE FROM client_configs WHERE client_id = $1`
	_, err := c.db.Exec(query, clientId)
	if err != nil {
		log.Print("[ERROR] delete config err: ", err)
		return err
	}
	return nil
}

func (c *ConfigRepositoryImpl) GetAll() ([]rateLimiter.TokenBucketConfig, error) {
	query := `SELECT client_id, capacity, refill_interval FROM client_configs`
	rows, err := c.db.Query(query)
	if err != nil {
		log.Print("[ERROR] get all config err: ", err)
		return nil, err
	}
	defer rows.Close()
	var cfgs []rateLimiter.TokenBucketConfig
	for rows.Next() {
		var cfg rateLimiter.TokenBucketConfig
		err = rows.Scan(&cfg.ClientId, &cfg.Capacity, &cfg.RefillInterval)
		if err != nil {
			log.Print("[ERROR] get all config err: ", err)
			return nil, err
		}
		cfgs = append(cfgs, cfg)
	}
	return cfgs, nil
}
