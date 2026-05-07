package config

import (
	"os"
	"time"
)

type Config struct {
	Port              string
	TickInterval      time.Duration
	DefaultVolatility float64
	CORSOrigin        string
}

func Load() Config {
	cfg := Config{
		Port:              getEnv("PORT", "8080"),
		TickInterval:      parseDuration(getEnv("TICK_INTERVAL", "10s")),
		DefaultVolatility: 0.03,
		CORSOrigin:        getEnv("CORS_ORIGIN", "*"),
	}
	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 10 * time.Second
	}
	return d
}
