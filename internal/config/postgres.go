package config

import (
	"fmt"
	"strconv"
	"strings"
)

type MaxConnections int

func (m *MaxConnections) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return fmt.Errorf("max connections cannot be empty")
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("invalid max connections value: %q", text)
	}
	if parsed < 1 {
		return fmt.Errorf("max connections must be greater than 0")
	}
	*m = MaxConnections(parsed)
	return nil
}

type Postgres struct {
	Host           string         `env:"HOST,required"`
	Port           Port           `env:"PORT,required"`
	User           string         `env:"USER,required"`
	Password       Secret         `env:"PASSWORD,required"`
	DB             string         `env:"DB,required"`
	MaxConnections MaxConnections `env:"MAX_CONNECTIONS" envDefault:"10"`
}
