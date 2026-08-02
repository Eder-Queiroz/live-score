package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Brokers []string

func (b *Brokers) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		return fmt.Errorf("brokers must not be empty")
	}
	brokers := make([]string, 0, len(strings.Split(s, ",")))
	for broker := range strings.SplitSeq(s, ",") {
		broker = strings.TrimSpace(broker)
		if broker == "" {
			return fmt.Errorf("brokers must not contain empty strings")
		}
		_, _, err := net.SplitHostPort(broker)
		if err != nil {
			return fmt.Errorf("invalid broker address: %q", broker)
		}
		brokers = append(brokers, strings.TrimSpace(broker))
	}
	*b = Brokers(brokers)
	return nil
}

type ClientID string

func (c *ClientID) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		return fmt.Errorf("client ID must not be empty")
	}
	*c = ClientID(s)
	return nil
}

type Kafka struct {
	Brokers  Brokers  `env:"BROKERS,required"`
	ClientID ClientID `env:"CLIENT_ID,required"`
}

type Acks string

const (
	AcksAll  Acks = "all"
	AcksOne  Acks = "1"
	AcksNone Acks = "0"
)

func (a *Acks) UnmarshalText(text []byte) error {
	switch Acks(text) {
	case AcksAll, AcksOne, AcksNone:
		*a = Acks(text)
		return nil
	default:
		return fmt.Errorf("invalid Acks value: %q", text)
	}
}

type KafkaProducer struct {
	Acks         Acks          `env:"ACKS" envDefault:"all"`
	Idempotent   bool          `env:"IDEMPOTENT" envDefault:"true"`
	FlushTimeout time.Duration `env:"FLUSH_TIMEOUT" envDefault:"10s"`
}

type GroupID string

func (g *GroupID) UnmarshalText(text []byte) error {
	trimmed := strings.TrimSpace(string(text))
	if trimmed == "" {
		return fmt.Errorf("group ID cannot be empty")
	}
	*g = GroupID(trimmed)
	return nil
}

type AutoOffsetReset string

const (
	AutoOffsetResetEarliest AutoOffsetReset = "earliest"
	AutoOffsetResetLatest   AutoOffsetReset = "latest"
)

func (a *AutoOffsetReset) UnmarshalText(text []byte) error {
	switch AutoOffsetReset(text) {
	case AutoOffsetResetEarliest, AutoOffsetResetLatest:
		*a = AutoOffsetReset(text)
	default:
		return fmt.Errorf("invalid AutoOffsetReset value: %q", text)
	}
	return nil
}

type KafkaConsumer struct {
	GroupID         GroupID         `env:"GROUP_ID,required"`
	AutoOffsetReset AutoOffsetReset `env:"AUTO_OFFSET_RESET" envDefault:"earliest"`
}
