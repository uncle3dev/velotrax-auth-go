package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppPort int    `mapstructure:"APP_PORT"`
	AppEnv  string `mapstructure:"APP_ENV"`

	JWTSecret        string        `mapstructure:"JWT_SECRET"`
	JWTExpiry        time.Duration `mapstructure:"JWT_EXPIRY"`
	JWTRefreshExpiry time.Duration `mapstructure:"JWT_REFRESH_EXPIRY"`

	LogLevel string `mapstructure:"LOG_LEVEL"`

	MongoURI string `mapstructure:"MONGO_URI"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.SetDefault("APP_PORT", 50051)
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("JWT_EXPIRY", 15*time.Minute)
	v.SetDefault("JWT_REFRESH_EXPIRY", 7*24*time.Hour)
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("JWT_SECRET", "change_me_to_a_strong_secret_at_least_32_chars")

	v.AutomaticEnv()
	for _, key := range []string{"APP_PORT", "APP_ENV", "JWT_SECRET", "JWT_EXPIRY", "JWT_REFRESH_EXPIRY", "LOG_LEVEL", "MONGO_URI"} {
		v.BindEnv(key)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.JWTSecret == "" || len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if cfg.MongoURI == "" {
		return fmt.Errorf("MONGO_URI is required")
	}
	return nil
}
