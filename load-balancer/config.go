package main

import "github.com/spf13/viper"

type Config struct {
	Port     string
	Backends []string
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	return &Config{
		Port:     viper.GetString("port"),
		Backends: viper.GetStringSlice("backends"),
	}, nil
}
