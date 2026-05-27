// Package config loads all service configuration from environment variables (12-factor).
package config

import (
	"os"
	"strconv"
)

// Config is the top-level configuration container.
type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	Mongo    MongoConfig
	MinIO    MinIOConfig
	LLM      LLMConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	// MaxOpenConns and MaxIdleConns control the connection pool.
	MaxOpenConns int
	MaxIdleConns int
}

type MongoConfig struct {
	URI string
}

type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	// UseSSL should be false for local/Kind deployments.
	UseSSL bool
}

type LLMConfig struct {
	// Provider is either "openai" or "ollama".
	Provider     string
	OpenAIAPIKey string
	OpenAIModel  string
	// OllamaBaseURL is only used when Provider == "ollama".
	OllamaBaseURL string
	OllamaModel   string
}

type AuthConfig struct {
	APIKey string
}

// Load reads all config from environment variables, applying sensible defaults.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Postgres: PostgresConfig{
			Host:         getEnv("POSTGRES_HOST", "localhost"),
			Port:         getEnv("POSTGRES_PORT", "5432"),
			User:         getEnv("POSTGRES_USER", "admin"),
			Password:     getEnv("POSTGRES_PASSWORD", "secretpass"),
			DBName:       getEnv("POSTGRES_DB", "master_registry"),
			SSLMode:      getEnv("POSTGRES_SSLMODE", "disable"),
			MaxOpenConns: getEnvInt("POSTGRES_MAX_OPEN_CONNS", 25),
			MaxIdleConns: getEnvInt("POSTGRES_MAX_IDLE_CONNS", 5),
		},
		Mongo: MongoConfig{
			URI: getEnv("MONGO_URI", "mongodb://localhost:27017"),
		},
		MinIO: MinIOConfig{
			Endpoint:        getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
			UseSSL:          getEnvBool("MINIO_USE_SSL", false),
		},
		LLM: LLMConfig{
			Provider:      getEnv("LLM_PROVIDER", "openai"),
			OpenAIAPIKey:  getEnv("OPENAI_API_KEY", ""),
			OpenAIModel:   getEnv("OPENAI_MODEL", "gpt-4o-mini"),
			OllamaBaseURL: getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
			OllamaModel:   getEnv("OLLAMA_MODEL", "llama3"),
		},
		Auth: AuthConfig{
			APIKey: getEnv("API_KEY", "changeme"),
		},
	}
}

// DSN returns a PostgreSQL connection string for use with sqlx/pq.
func (p PostgresConfig) DSN() string {
	return "host=" + p.Host +
		" port=" + p.Port +
		" user=" + p.User +
		" password=" + p.Password +
		" dbname=" + p.DBName +
		" sslmode=" + p.SSLMode
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}
