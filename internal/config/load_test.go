package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// unsetEnv removes the given variables for the duration of the test and
// restores whatever was there before. t.Setenv can only set a value, and an
// empty string still counts as "set" for the required option — so the missing
// variable cases need a real unset.
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if old, ok := os.LookupEnv(key); ok {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("could not unset %s: %v", key, err)
			}
			t.Cleanup(func() {
				if err := os.Setenv(key, old); err != nil {
					t.Errorf("could not restore %s: %v", key, err)
				}
			})
		}
	}
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	for key, value := range vars {
		t.Setenv(key, value)
	}
}

var scoreProjectorEnv = map[string]string{
	"KAFKA_BROKERS":           "localhost:9092",
	"KAFKA_CLIENT_ID":         "score-projector",
	"KAFKA_CONSUMER_GROUP_ID": "score-projector",
	"POSTGRES_HOST":           "localhost",
	"POSTGRES_PORT":           "5432",
	"POSTGRES_USER":           "postgres",
	"POSTGRES_PASSWORD":       "test-only-placeholder",
	"POSTGRES_DB":             "live_score",
	"REDIS_ADDR":              "localhost:6379",
	"REDIS_PASSWORD":          "test-only-placeholder",
	"REDIS_DB":                "0",
	"TOPIC_EVENTS":            "match.events.v1",
}

func TestLoadIngestorFailsWhenRequiredVarsAreMissing(t *testing.T) {
	required := []string{"KAFKA_BROKERS", "KAFKA_CLIENT_ID", "TOPIC_SNAPSHOT"}
	unsetEnv(t, required...)

	_, err := LoadIngestor()
	if err == nil {
		t.Fatal("expected an error when required variables are missing, got none")
	}

	// Every missing variable must be named, not just the first one: fixing
	// configuration one round trip at a time is what this loader avoids.
	for _, key := range required {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error message does not name %s: %v", key, err)
		}
	}
}

func TestLoadIngestorSucceedsWithRequiredVars(t *testing.T) {
	setEnv(t, map[string]string{
		"KAFKA_BROKERS":   "a:9092,b:9092",
		"KAFKA_CLIENT_ID": "ingestor",
		"TOPIC_SNAPSHOT":  "match.snapshots.v1",
	})

	cfg, err := LoadIngestor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Kafka.Brokers) != 2 {
		t.Errorf("expected 2 brokers, got %d: %q", len(cfg.Kafka.Brokers), cfg.Kafka.Brokers)
	}
	if cfg.Kafka.ClientID != "ingestor" {
		t.Errorf("expected client id %q, got %q", "ingestor", cfg.Kafka.ClientID)
	}
	if cfg.SnapshotTopic != "match.snapshots.v1" {
		t.Errorf("expected topic %q, got %q", "match.snapshots.v1", cfg.SnapshotTopic)
	}
}

func TestLoadScoreProjectorReportsEveryMissingVar(t *testing.T) {
	keys := make([]string, 0, len(scoreProjectorEnv))
	for key := range scoreProjectorEnv {
		keys = append(keys, key)
	}
	unsetEnv(t, keys...)

	_, err := LoadScoreProjector()
	if err == nil {
		t.Fatal("expected an error when required variables are missing, got none")
	}

	for _, key := range keys {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error message does not name %s", key)
		}
	}
}

func TestLoadScoreProjectorSucceedsWithFullEnv(t *testing.T) {
	setEnv(t, scoreProjectorEnv)

	cfg, err := LoadScoreProjector()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Postgres.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Postgres.Port)
	}
	if cfg.Postgres.Password.Reveal() != "test-only-placeholder" {
		t.Error("postgres password was not parsed")
	}
	if cfg.Consumer.AutoOffsetReset != AutoOffsetResetEarliest {
		t.Errorf("expected default auto offset reset %q, got %q", AutoOffsetResetEarliest, cfg.Consumer.AutoOffsetReset)
	}
}

func TestLoadScoreProjectorRejectsInvalidValue(t *testing.T) {
	setEnv(t, scoreProjectorEnv)
	t.Setenv("POSTGRES_PORT", "70000")

	_, err := LoadScoreProjector()
	if err == nil {
		t.Fatal("expected an error for an out-of-range port, got none")
	}
	if !strings.Contains(err.Error(), "65535") {
		t.Errorf("error message does not explain the valid range: %v", err)
	}
}

func TestAppDefaultsAreApplied(t *testing.T) {
	unsetEnv(t, "APP_ENV", "LOG_LEVEL", "SHUTDOWN_TIMEOUT")
	setEnv(t, map[string]string{
		"KAFKA_BROKERS":   "localhost:9092",
		"KAFKA_CLIENT_ID": "ingestor",
		"TOPIC_SNAPSHOT":  "match.snapshots.v1",
	})

	cfg, err := LoadIngestor()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.App.Env != EnvDevelopment {
		t.Errorf("expected default env %q, got %q", EnvDevelopment, cfg.App.Env)
	}
	if cfg.App.LogLevel != LogLevelInfo {
		t.Errorf("expected default log level %q, got %q", LogLevelInfo, cfg.App.LogLevel)
	}
	if cfg.App.ShutdownTimeout != 15*time.Second {
		t.Errorf("expected default shutdown timeout 15s, got %s", cfg.App.ShutdownTimeout)
	}
}

func TestLoadedConfigDoesNotLeakSecrets(t *testing.T) {
	setEnv(t, scoreProjectorEnv)

	cfg, err := LoadScoreProjector()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// This is the line every service prints at boot.
	printed := fmt.Sprintf("%+v", cfg)
	if strings.Contains(printed, "test-only-placeholder") {
		t.Errorf("secret leaked when printing the whole config: %s", printed)
	}
}
