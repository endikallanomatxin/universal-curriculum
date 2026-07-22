package server

import (
	"strings"
	"testing"
)

func TestLoadConfigUsesEnvironmentDefaults(t *testing.T) {
	setDatabaseEnvironment(t)

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Env != "dev" || config.Port != "8080" || config.DBSSLMode != "disable" ||
		config.StorageBackend != "local" || config.UploadsFolder != "uploads" {
		t.Fatalf("unexpected development defaults: %+v", config)
	}

	t.Setenv("ENV", "prod")
	config, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.DBSSLMode != "require" || config.UploadsFolder != "/app/uploads" {
		t.Fatalf("unexpected production defaults: %+v", config)
	}
}

func TestLoadConfigRejectsUnsupportedStorageBackend(t *testing.T) {
	setDatabaseEnvironment(t)
	t.Setenv("STORAGE_BACKEND", "s3")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "unsupported STORAGE_BACKEND") {
		t.Fatalf("LoadConfig() error = %v", err)
	}
}

func TestLoadConfigRequiresCompleteBootstrapCredentials(t *testing.T) {
	setDatabaseEnvironment(t)
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("LoadConfig() error = %v", err)
	}
}

func TestLoadConfigReportsMissingDatabaseSettings(t *testing.T) {
	t.Setenv("ENV", "dev")
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME"} {
		t.Setenv(key, "")
	}

	_, err := LoadConfig()
	if err == nil || !strings.Contains(err.Error(), "missing database configuration") {
		t.Fatalf("LoadConfig() error = %v", err)
	}
}

func setDatabaseEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("STORAGE_BACKEND", "")
	t.Setenv("UPLOADS_FOLDER", "")
	t.Setenv("BOOTSTRAP_ADMIN_EMAIL", "")
	t.Setenv("BOOTSTRAP_ADMIN_PASSWORD", "")
	t.Setenv("BOOTSTRAP_ADMIN_FULL_NAME", "")
	t.Setenv("BOOTSTRAP_ADMIN_ALIAS", "")
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "universal_curriculum")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "universal_curriculum")
}
