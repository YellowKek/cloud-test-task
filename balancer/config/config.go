package config

import "github.com/spf13/viper"

type Config struct {
	Port     string
	Backends []string
	Db       string
}

// LoadConfig функция для загрузки конфига из файла config.yaml
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	return &Config{
		Port:     viper.GetString("port"),
		Backends: viper.GetStringSlice("backends"),
		Db:       viper.GetString("db"),
	}, nil
}
