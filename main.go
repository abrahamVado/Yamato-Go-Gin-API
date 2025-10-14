package main

import (
	"log"

	"github.com/example/Yamato-Go-Gin-API/bootstrap"
)

func main() {
	// 1.- Initialize the router and configuration using the bootstrap package.
	router, config := bootstrap.SetupRouter()

	// 2.- Start the HTTP server and log fatal errors if the server stops unexpectedly.
	if err := router.Run(":" + config.Port); err != nil {
		// 3.- Record the error to aid diagnostics in case startup fails.
		log.Fatalf("failed to start server: %v", err)
	}
}
