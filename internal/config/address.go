package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

type Port int

func (p *Port) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return fmt.Errorf("port cannot be empty")
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("invalid port value: %q", text)
	}
	if parsed < 1 || parsed > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got: %d", parsed)
	}
	*p = Port(parsed)
	return nil
}

type Address string

func (a *Address) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return fmt.Errorf("address cannot be empty")
	}
	_, _, err := net.SplitHostPort(trimmed)
	if err != nil {
		return fmt.Errorf("invalid address: %q", text)
	}
	*a = Address(trimmed)
	return nil
}
