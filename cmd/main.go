package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/app"
)

func main() {
	log.Println("Starting application...")

	pgConnString := "postgresql://new_user:new_password@postgres:5432/mydatabase?sslmode=disable"
	rabbitMQURL := "amqp://guest:guest@rabbitmq:5672/"

	application, err := app.New(pgConnString, rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}
	defer application.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := application.Run(ctx); err != nil {
			log.Printf("Application error: %v", err)
			cancel()
		}
	}()

	log.Println("Application is now running. Press Ctrl+C to shut down.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down...")
	cancel()
	log.Println("Shutdown complete.")
}
