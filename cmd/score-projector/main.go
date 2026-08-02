package main

import (
	"log"

	"github.com/Eder-Queiroz/live-score/internal/config"
)

func main() {
	cfg, err := config.LoadScoreProjector()
	if err != nil {
		log.Fatal(cfg)

	}
	log.Println(cfg)
}
