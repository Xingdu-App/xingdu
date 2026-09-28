package config

import (
	"errors"
	"os"
)

type Config struct {
	HTTPAddr    string
	DatabaseURL string
}

func Load() (Config, error) {
	c := Config{HTTPAddr: os.Getenv("XINGDU_HTTP_ADDR"), DatabaseURL: os.Getenv("DATABASE_URL")}
	if c.HTTPAddr == "" {
		c.HTTPAddr = "127.0.0.1:18080"
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return c, nil
}
