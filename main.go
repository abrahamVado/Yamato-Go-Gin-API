package main

import (
	"github.com/example/Yamato-Go-Gin-API/bootstrap"
	"go.uber.org/zap"
)

func main() {
	// 1.- Initialize the router and configuration using the bootstrap package.
	router, config := bootstrap.SetupRouter()

	// 2.- Ensure buffered log entries are flushed when the application exits.
	logger := zap.L()
	defer func() {
		_ = logger.Sync()
	}()

	// 3.- Start the HTTP server and log fatal errors if the server stops unexpectedly.
	logger.Info("starting HTTP server", zap.String("port", config.Port))
	if err := router.Run(":" + config.Port); err != nil {
		// 4.- Record the error to aid diagnostics in case startup fails.
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
