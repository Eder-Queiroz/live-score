package config

import "testing"

func TestMaxConnectionsUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    MaxConnections
		wantErr bool
	}{
		{"typical value", "10", 10, false},
		{"minimum value", "1", 1, false},
		{"surrounding spaces are trimmed", "  10  ", 10, false},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"not a number", "ten", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m MaxConnections
			err := m.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if m != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, m)
			}
		})
	}
}
