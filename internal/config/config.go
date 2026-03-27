package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv               string
	HTTPPort             int
	DBHost               string
	DBPort               int
	DBUser               string
	DBPassword           string
	DBName               string
	DBSSLMode            string
	DBTimezone           string
	DBMaxIdleConns       int
	DBMaxOpenConns       int
	DBConnMaxLifetimeMin int
	DBConnMaxIdleTimeMin int
	LogLevel             string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	httpPort, err := getEnvInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	dbPort, err := getEnvInt("DB_PORT", 5432)
	if err != nil {
		return Config{}, err
	}

	dbMaxIdleConns, err := getEnvInt("DB_MAX_IDLE_CONNS", 10)
	if err != nil {
		return Config{}, err
	}

	dbMaxOpenConns, err := getEnvInt("DB_MAX_OPEN_CONNS", 50)
	if err != nil {
		return Config{}, err
	}

	dbConnMaxLifetimeMin, err := getEnvInt("DB_CONN_MAX_LIFETIME_MIN", 30)
	if err != nil {
		return Config{}, err
	}

	dbConnMaxIdleTimeMin, err := getEnvInt("DB_CONN_MAX_IDLE_TIME_MIN", 10)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:               strings.ToLower(getEnv("APP_ENV", "development")),
		HTTPPort:             httpPort,
		DBHost:               getEnv("DB_HOST", "localhost"),
		DBPort:               dbPort,
		DBUser:               getEnv("DB_USER", "postgres"),
		DBPassword:           getEnv("DB_PASSWORD", "postgres"),
		DBName:               getEnv("DB_NAME", "srtp"),
		DBSSLMode:            getEnv("DB_SSLMODE", "disable"),
		DBTimezone:           getEnv("DB_TIMEZONE", "Asia/Shanghai"),
		DBMaxIdleConns:       dbMaxIdleConns,
		DBMaxOpenConns:       dbMaxOpenConns,
		DBConnMaxLifetimeMin: dbConnMaxLifetimeMin,
		DBConnMaxIdleTimeMin: dbConnMaxIdleTimeMin,
		LogLevel:             strings.ToLower(getEnv("LOG_LEVEL", "info")),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.HTTPPort <= 0 {
		return fmt.Errorf("HTTP_PORT must be greater than 0")
	}
	if strings.TrimSpace(c.DBHost) == "" {
		return fmt.Errorf("DB_HOST is required")
	}
	if c.DBPort <= 0 {
		return fmt.Errorf("DB_PORT must be greater than 0")
	}
	if strings.TrimSpace(c.DBUser) == "" {
		return fmt.Errorf("DB_USER is required")
	}
	if strings.TrimSpace(c.DBPassword) == "" {
		return fmt.Errorf("DB_PASSWORD is required")
	}
	if strings.TrimSpace(c.DBName) == "" {
		return fmt.Errorf("DB_NAME is required")
	}
	if c.DBMaxIdleConns < 0 {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be greater than or equal to 0")
	}
	if c.DBMaxOpenConns <= 0 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be greater than 0")
	}
	if c.DBConnMaxLifetimeMin <= 0 {
		return fmt.Errorf("DB_CONN_MAX_LIFETIME_MIN must be greater than 0")
	}
	if c.DBConnMaxIdleTimeMin < 0 {
		return fmt.Errorf("DB_CONN_MAX_IDLE_TIME_MIN must be greater than or equal to 0")
	}
	if strings.TrimSpace(c.LogLevel) == "" {
		return fmt.Errorf("LOG_LEVEL is required")
	}

	return nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return parsed, nil
}
