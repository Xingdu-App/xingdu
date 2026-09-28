package config

import (
	"errors"
	"net/url"
	"os"
)

type Config struct {
	HTTPAddr      string
	PublicOrigin  string
	SecureCookies bool
	DatabaseURL   string
}

func Load() (Config, error) {
	c := Config{HTTPAddr: os.Getenv("XINGDU_HTTP_ADDR"), DatabaseURL: os.Getenv("DATABASE_URL")}
	if c.HTTPAddr == "" {
		c.HTTPAddr = "127.0.0.1:18080"
	}
	c.PublicOrigin = os.Getenv("XINGDU_PUBLIC_ORIGIN")
	if c.PublicOrigin == "" {
		c.PublicOrigin = "http://127.0.0.1:15173"
	}
	u, err := url.Parse(c.PublicOrigin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Config{}, errors.New("XINGDU_PUBLIC_ORIGIN must be an origin without a path")
	}
	if u.Scheme == "http" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return Config{}, errors.New("non-local origins require HTTPS")
	}
	c.SecureCookies = u.Scheme == "https"
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return c, nil
}
