package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgListener, err := pglistener.New("postgresql://new_user:new_password@postgres:5432/mydatabase?sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL listener: %v", err)
	}
	defer pgListener.Close(ctx)

	rmqPublisher, err := rabbitmq.New("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalf("Failed to create RabbitMQ publisher: %v", err)
	}
	defer rmqPublisher.Close()

	// Start listening for changes
	changes, err := pgListener.Listen(ctx)
	if err != nil {
		log.Fatalf("Failed to start listening: %v", err)
	}

	// Process changes and publish to RabbitMQ
	go func() {
		for change := range changes {
			if err := rmqPublisher.Publish(change); err != nil {
				log.Printf("Failed to publish change: %v", err)
			}
		}
	}()

	// Wait for interrupt signal to gracefully shut down
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down...")
}
