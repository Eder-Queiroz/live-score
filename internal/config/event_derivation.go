package config

import "github.com/caarlos0/env/v11"

type EventDerivation struct {
	App           App           `envPrefix:""`
	Kafka         Kafka         `envPrefix:"KAFKA_"`
	Producer      KafkaProducer `envPrefix:"KAFKA_PRODUCER_"`
	Consumer      KafkaConsumer `envPrefix:"KAFKA_CONSUMER_"`
	EventsTopic   string        `env:"TOPIC_EVENTS,required"`
	SnapshotTopic string        `env:"TOPIC_SNAPSHOT,required"`
}

func LoadEventDerivation() (EventDerivation, error) {
	eventDerivation := EventDerivation{}
	if err := env.Parse(&eventDerivation); err != nil {
		return EventDerivation{}, err
	}
	return eventDerivation, nil
}
