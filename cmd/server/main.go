package main

import (
	"log"

	"github.com/abrahamVado/Yamato-Go-Gin-API/internal/config"
	apphttp "github.com/abrahamVado/Yamato-Go-Gin-API/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	r := apphttp.NewRouter(cfg)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
