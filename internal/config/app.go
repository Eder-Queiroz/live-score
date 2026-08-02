package config

import (
	"fmt"
	"time"
)

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

func (l *LogLevel) UnmarshalText(text []byte) error {
	switch LogLevel(text) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
		*l = LogLevel(text)
		return nil
	default:
		return fmt.Errorf("must be one of debug, info, warn, error; got %q", text)
	}
}

type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
	EnvStaging     Env = "staging"
	EnvTest        Env = "test"
)

func (e *Env) UnmarshalText(text []byte) error {
	switch Env(text) {
	case EnvDevelopment, EnvProduction, EnvStaging, EnvTest:
		*e = Env(text)
		return nil
	default:
		return fmt.Errorf("must be one of development, production, staging, test; got %q", text)
	}
}

type App struct {
	Env             Env           `env:"APP_ENV" envDefault:"development"`
	LogLevel        LogLevel      `env:"LOG_LEVEL" envDefault:"info"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}
