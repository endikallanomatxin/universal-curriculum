package server

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env            string
	Port           string
	StorageBackend string
	UploadsFolder  string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBSSLMode      string
}

func LoadConfig() (Config, error) {
	env := getEnv("ENV", "dev")
	if env != "dev" && env != "prod" {
		return Config{}, fmt.Errorf("invalid ENV %q", env)
	}

	config := Config{
		Env:            env,
		Port:           getEnv("PORT", "8080"),
		StorageBackend: getEnv("STORAGE_BACKEND", "local"),
		UploadsFolder:  getUploadsFolder(env),
		DBHost:         os.Getenv("DB_HOST"),
		DBPort:         os.Getenv("DB_PORT"),
		DBUser:         os.Getenv("DB_USER"),
		DBPassword:     os.Getenv("DB_PASSWORD"),
		DBName:         os.Getenv("DB_NAME"),
		DBSSLMode:      getDBSSLMode(env),
	}
	if config.StorageBackend != "local" {
		return Config{}, fmt.Errorf("unsupported STORAGE_BACKEND %q: only local is supported for now", config.StorageBackend)
	}

	missing := make([]string, 0, 5)
	for key, value := range map[string]string{
		"DB_HOST": config.DBHost, "DB_PORT": config.DBPort, "DB_USER": config.DBUser,
		"DB_PASSWORD": config.DBPassword, "DB_NAME": config.DBName,
	} {
		if value == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing database configuration: %s", strings.Join(missing, ", "))
	}
	return config, nil
}

func getUploadsFolder(env string) string {
	fallback := "uploads"
	if env == "prod" {
		fallback = "/app/uploads"
	}
	return getEnv("UPLOADS_FOLDER", fallback)
}

func (config Config) ServerAddress() string {
	return ":" + config.Port
}

func (config Config) PostgresConnString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword,
		config.DBName, config.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDBSSLMode(env string) string {
	fallback := "disable"
	if env == "prod" {
		fallback = "require"
	}
	return getEnv("DB_SSLMODE", fallback)
}
