package main

import (
	"log"
	"os"

	"nusantara/internal/app"
	"nusantara/internal/config"
)

func main() {
	logger := log.New(os.Stdout, "nusantarad ", log.LstdFlags|log.LUTC)

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	application := app.New(cfg, logger)
	if err := application.Run(); err != nil {
		logger.Fatalf("run: %v", err)
	}
}


