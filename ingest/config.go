package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration values for the ingest service
type Config struct {
	// WebSocket configuration
	TurboStreamURL string

	// Elasticsearch configuration
	ElasticsearchURL string

	// Worker configuration
	WebSocketWorkers     int
	ElasticsearchWorkers int
	WorkerTimeout        time.Duration

	// Logging configuration
	LoggingEnabled bool

	// Server configuration
	Port string
}

// LoadConfig loads configuration from environment variables with defaults
func LoadConfig() *Config {
	return &Config{
		TurboStreamURL:       getEnv("TURBOSTREAM_URL", "wss://graze.social/turbostream"),
		ElasticsearchURL:     getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		WebSocketWorkers:     getEnvInt("WEBSOCKET_WORKERS", 3),
		ElasticsearchWorkers: getEnvInt("ELASTICSEARCH_WORKERS", 5),
		WorkerTimeout:        getEnvDuration("WORKER_TIMEOUT", 30*time.Second),
		LoggingEnabled:       getEnvBool("LOGGING_ENABLED", true),
		Port:                 getEnv("PORT", "8080"),
	}
}

// getEnv returns the value of an environment variable or a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns the integer value of an environment variable or a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool returns the boolean value of an environment variable or a default value
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvDuration returns the duration value of an environment variable or a default value
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}