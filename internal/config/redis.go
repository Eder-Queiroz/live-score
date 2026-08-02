package config

type Redis struct {
	Addr     Address `env:"ADDR,required"`
	Password Secret  `env:"PASSWORD,required"`
	DB       int     `env:"DB,required"`
}
