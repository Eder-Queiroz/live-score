package config

import "github.com/caarlos0/env/v11"

type ScoreProjector struct {
	App         App           `envPrefix:""`
	Kafka       Kafka         `envPrefix:"KAFKA_"`
	Consumer    KafkaConsumer `envPrefix:"KAFKA_CONSUMER_"`
	Postgres    Postgres      `envPrefix:"POSTGRES_"`
	Redis       Redis         `envPrefix:"REDIS_"`
	EventsTopic string        `env:"TOPIC_EVENTS,required"`
}

func LoadScoreProjector() (ScoreProjector, error) {
	var cfg ScoreProjector
	if err := env.Parse(&cfg); err != nil {
		return ScoreProjector{}, err
	}
	return cfg, nil
}
