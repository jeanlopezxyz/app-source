// Package config reads the API configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// DefaultPasswordFile is where the database password is mounted from a Secret.
const DefaultPasswordFile = "/etc/catalogo/db/password"

// Config holds every setting the API needs. The password is not stored here:
// it is read from PasswordFile on every new database connection.
type Config struct {
	ListenAddr   string
	DBHost       string
	DBPort       int
	DBName       string
	DBUser       string
	DBSSLMode    string
	PasswordFile string
}

// Load builds a Config from the environment, applying defaults.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return fallback
	}

	port, err := strconv.Atoi(value("DB_PORT", "5432"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("DB_PORT must be a valid TCP port, got %q", getenv("DB_PORT"))
	}

	cfg := Config{
		ListenAddr:   ":" + value("PORT", "8080"),
		DBHost:       value("DB_HOST", "localhost"),
		DBPort:       port,
		DBName:       value("DB_NAME", "catalogo"),
		DBUser:       value("DB_USER", "catalogo"),
		DBSSLMode:    value("DB_SSLMODE", "prefer"),
		PasswordFile: value("DB_PASSWORD_FILE", DefaultPasswordFile),
	}
	return cfg, nil
}
