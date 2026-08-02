package config

import (
	"encoding/json"
	"log/slog"
)

type Secret string

func (s Secret) String() string {
	return "[REDACTED]"
}

func (s Secret) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s Secret) Reveal() string {
	return string(s)
}

func (s Secret) LogValue() slog.Value {
	return slog.StringValue("[REDACTED]")
}
