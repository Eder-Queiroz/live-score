package config

import "github.com/caarlos0/env/v11"

type Ingestor struct {
	App           App           `envPrefix:""`
	Kafka         Kafka         `envPrefix:"KAFKA_"`
	Producer      KafkaProducer `envPrefix:"KAFKA_PRODUCER_"`
	SnapshotTopic string        `env:"TOPIC_SNAPSHOT,required"`
}

func LoadIngestor() (Ingestor, error) {
	ingestor := Ingestor{}
	if err := env.Parse(&ingestor); err != nil {
		return Ingestor{}, err
	}
	return ingestor, nil
}
