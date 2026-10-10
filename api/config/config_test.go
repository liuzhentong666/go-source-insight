package config

import "testing"

func TestSourceAndJobConfiguration(t *testing.T) {
	for _, name := range []string{"SOURCE_ROOT", "JOB_WORKERS", "JOB_QUEUE_SIZE"} {
		t.Setenv(name, "")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceRoot != "sources" || cfg.JobWorkers != 2 || cfg.JobQueueSize != 16 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	for _, name := range []string{"JOB_WORKERS", "JOB_QUEUE_SIZE"} {
		for _, value := range []string{"0", "-1", "not-a-number", "100000"} {
			t.Setenv(name, value)
			if _, err := Load(); err == nil {
				t.Fatalf("%s accepted %q", name, value)
			}
		}
		t.Setenv(name, "")
	}
	t.Setenv("SOURCE_ROOT", "project-sources")
	t.Setenv("JOB_WORKERS", "3")
	t.Setenv("JOB_QUEUE_SIZE", "8")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceRoot != "project-sources" || cfg.JobWorkers != 3 || cfg.JobQueueSize != 8 {
		t.Fatal("explicit source/job configuration ignored")
	}
}

func TestConfiguredJWTAndLifecycleBounds(t *testing.T) {
	for key, value := range map[string]string{"PORT": "8080", "DB_HOST": "localhost", "DB_PORT": "5432", "DB_USER": "app", "DB_PASSWORD": "test-only", "DB_NAME": "app", "JWT_SECRET": "N7q4KP2rV8mZ1uX5dT9sB3cW6fH0jLQe", "JOB_TIMEOUT_SECONDS": "60", "SHUTDOWN_TIMEOUT_SECONDS": "10", "CORS_ORIGINS": "https://app.example"} {
		t.Setenv(key, value)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.JWT.SecretKey != "N7q4KP2rV8mZ1uX5dT9sB3cW6fH0jLQe" {
		t.Fatal("configured key ignored")
	}
	for _, key := range []string{"JOB_TIMEOUT_SECONDS", "SHUTDOWN_TIMEOUT_SECONDS"} {
		for _, value := range []string{"0", "-1", "bad", "100000"} {
			t.Setenv(key, value)
			if _, err := Load(); err == nil {
				t.Fatalf("invalid bound %s=%q", key, value)
			}
		}
		t.Setenv(key, "10")
	}
	for _, secret := range []string{"", "change-me-with-a-long-secret-value"} {
		cfg.JWT.SecretKey = secret
		if err := cfg.Validate(); err == nil {
			t.Fatal("weak key passed startup validation")
		}
	}
	cfg.JWT.SecretKey = "N7q4KP2rV8mZ1uX5dT9sB3cW6fH0jLQe"
	for _, origin := range []string{"*", "https://app.example/path", "http://", "https://app.example?token=value"} {
		cfg.CORSOrigins = []string{origin}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("invalid origin=%s", origin)
		}
	}
}
