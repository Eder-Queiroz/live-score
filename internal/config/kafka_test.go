package config

import (
	"slices"
	"testing"
)

func TestBrokersUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Brokers
		wantErr bool
	}{
		{"single broker", "localhost:9092", Brokers{"localhost:9092"}, false},
		{"multiple brokers", "a:9092,b:9092", Brokers{"a:9092", "b:9092"}, false},
		{"ipv6 broker", "[::1]:9092", Brokers{"[::1]:9092"}, false},
		{"spaces around each broker are trimmed", " a:9092 , b:9092 ", Brokers{"a:9092", "b:9092"}, false},
		{"empty", "", nil, true},
		{"only spaces", "   ", nil, true},
		{"empty item between commas", "a:9092,,b:9092", nil, true},
		{"broker without port", "localhost", nil, true},
		{"one broker without port", "a:9092,localhost", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Brokers
			err := b.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if !slices.Equal(b, tt.want) {
				t.Fatalf("expected %q, got %q", tt.want, b)
			}
		})
	}
}

func TestClientIDUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ClientID
		wantErr bool
	}{
		{"plain id", "ingestor", "ingestor", false},
		{"surrounding spaces are trimmed", "  ingestor  ", "ingestor", false},
		{"empty", "", "", true},
		{"only spaces", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c ClientID
			err := c.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if c != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, c)
			}
		})
	}
}

func TestGroupIDUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    GroupID
		wantErr bool
	}{
		{"plain id", "score-projector", "score-projector", false},
		{"surrounding spaces are trimmed", "  score-projector  ", "score-projector", false},
		{"empty", "", "", true},
		{"only spaces", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var g GroupID
			err := g.UnmarshalText([]byte(tt.input))

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if g != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, g)
			}
		})
	}
}

func TestAcksUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Acks
		wantErr bool
	}{
		{"all", "all", AcksAll, false},
		{"leader only", "1", AcksOne, false},
		{"none", "0", AcksNone, false},
		{"unsupported value", "2", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Acks
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

func TestAutoOffsetResetUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    AutoOffsetReset
		wantErr bool
	}{
		{"earliest", "earliest", AutoOffsetResetEarliest, false},
		{"latest", "latest", AutoOffsetResetLatest, false},
		{"unsupported value", "none", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a AutoOffsetReset
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
