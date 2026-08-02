package config

import "testing"

func TestPortUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Port
		wantErr bool
	}{
		{"valid port", "5432", 5432, false},
		{"lowest valid port", "1", 1, false},
		{"highest valid port", "65535", 65535, false},
		{"surrounding spaces are trimmed", "  5432  ", 5432, false},
		{"zero is out of range", "0", 0, true},
		{"above range", "65536", 0, true},
		{"negative", "-1", 0, true},
		{"not a number", "abc", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p Port
			err := p.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if p != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, p)
			}
		})
	}
}

func TestAddressUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Address
		wantErr bool
	}{
		{"host and port", "localhost:6379", "localhost:6379", false},
		{"ipv4 and port", "127.0.0.1:6379", "127.0.0.1:6379", false},
		{"ipv6 and port", "[::1]:6379", "[::1]:6379", false},
		{"empty host means all interfaces", ":8080", ":8080", false},
		{"surrounding spaces are trimmed", "  localhost:6379  ", "localhost:6379", false},
		{"missing port", "localhost", "", true},
		{"empty", "", "", true},
		{"port only, no colon", "6379", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Address
			err := a.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if a != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, a)
			}
		})
	}
}
