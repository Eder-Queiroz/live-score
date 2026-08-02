package config

import "github.com/caarlos0/env/v11"

type WebService struct {
	App      App      `envPrefix:""`
	Postgres Postgres `envPrefix:"POSTGRES_"`
	Redis    Redis    `envPrefix:"REDIS_"`
	HTTP     HTTP     `envPrefix:"HTTP_"`
}

func LoadWebService() (WebService, error) {
	ws := WebService{}
	if err := env.Parse(&ws); err != nil {
		return WebService{}, err
	}
	return ws, nil
}
