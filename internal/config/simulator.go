package config

import "github.com/caarlos0/env/v11"

type Simulator struct {
	App           App           `envPrefix:""`
	Kafka         Kafka         `envPrefix:"KAFKA_"`
	Producer      KafkaProducer `envPrefix:"KAFKA_PRODUCER_"`
	SnapshotTopic string        `env:"TOPIC_SNAPSHOT,required"`
}

func LoadSimulator() (Simulator, error) {
	simulator := Simulator{}
	if err := env.Parse(&simulator); err != nil {
		return Simulator{}, err
	}
	return simulator, nil
}
