package main

import (
	"aws-resource-watcher/internal/config"
	"aws-resource-watcher/internal/watcher"
	"context"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func main() {
	// Setup logging
	log.SetFormatter(&log.TextFormatter{})
	log.SetLevel(log.InfoLevel)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create and start the watcher
	w, err := watcher.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Start the watcher in a goroutine
	go func() {
		if err := w.Start(ctx); err != nil {
			log.Errorf("Watcher stopped with error: %v", err)
			cancel()
		}
	}()

	log.Info("AWS Resource Watcher started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutdown signal received, stopping watcher...")

	cancel()
	w.Stop()

	log.Info("AWS Resource Watcher stopped")
}
