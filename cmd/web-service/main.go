package main

import (
	"log"

	"github.com/Eder-Queiroz/live-score/internal/config"
)

func main() {
	cfg, err := config.LoadWebService()
	if err != nil {
		log.Fatal(err)

	}
	log.Println(cfg)
}
