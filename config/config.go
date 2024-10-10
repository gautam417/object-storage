package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Port        int    `mapstructure:"PORT"`
	LogLevel    string `mapstructure:"LOG_LEVEL"`
	MinioConfig MinioConfig
}

type MinioConfig struct {
	Endpoint  string `mapstructure:"MINIO_ENDPOINT"`
	AccessKey string `mapstructure:"MINIO_ACCESS_KEY"`
	SecretKey string `mapstructure:"MINIO_SECRET_KEY"`
	UseSSL    bool   `mapstructure:"MINIO_USE_SSL"`
}

func Load() (*Config, error) {
	viper.SetDefault("PORT", 3000)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("MINIO_USE_SSL", false)

	viper.AutomaticEnv()

	var config Config
	err := viper.Unmarshal(&config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if config.MinioConfig.Endpoint == "" {
		return nil, fmt.Errorf("MINIO_ENDPOINT is not set")
	}
	if config.MinioConfig.AccessKey == "" {
		return nil, fmt.Errorf("MINIO_ACCESS_KEY is not set")
	}
	if config.MinioConfig.SecretKey == "" {
		return nil, fmt.Errorf("MINIO_SECRET_KEY is not set")
	}

	return &config, nil
}