package config

import "time"

type HTTP struct {
	Addr              Address       `env:"ADDR" envDefault:":8080"`
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"5s"`
}
