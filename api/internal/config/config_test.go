package config

import "testing"

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(envFrom(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":8080" || cfg.DBPort != 5432 || cfg.DBName != "catalogo" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.PasswordFile != DefaultPasswordFile {
		t.Errorf("password file = %q, want %q", cfg.PasswordFile, DefaultPasswordFile)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"DB_HOST":          "catalogo-db-rw",
		"DB_PORT":          "6432",
		"DB_USER":          "app",
		"DB_PASSWORD_FILE": "/tmp/pw",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DBHost != "catalogo-db-rw" || cfg.DBPort != 6432 || cfg.DBUser != "app" || cfg.PasswordFile != "/tmp/pw" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "70000"} {
		if _, err := load(envFrom(map[string]string{"DB_PORT": port})); err == nil {
			t.Errorf("DB_PORT=%q: expected an error", port)
		}
	}
}
