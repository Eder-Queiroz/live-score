package config

import "testing"

func TestLogLevelUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    LogLevel
		wantErr bool
	}{
		{"debug", "debug", LogLevelDebug, false},
		{"info", "info", LogLevelInfo, false},
		{"warn", "warn", LogLevelWarn, false},
		{"error", "error", LogLevelError, false},
		{"unknown level", "verbose", "", true},
		{"wrong case", "INFO", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l LogLevel
			err := l.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if l != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, l)
			}
		})
	}
}

func TestEnvUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Env
		wantErr bool
	}{
		{"development", "development", EnvDevelopment, false},
		{"production", "production", EnvProduction, false},
		{"staging", "staging", EnvStaging, false},
		{"test", "test", EnvTest, false},
		{"unknown environment", "prod", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e Env
			err := e.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if e != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, e)
			}
		})
	}
}
