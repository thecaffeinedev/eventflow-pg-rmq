package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

func healthCheck(pgConnString, rabbitMQURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check PostgreSQL connection
	pgConn, err := pgx.Connect(ctx, pgConnString)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}
	defer pgConn.Close(ctx)

	err = pgConn.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %v", err)
	}

	// Check RabbitMQ connection
	rabbitConn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()

	return nil
}

func main() {
	log.Println("Starting application...")

	pgConnString := "postgresql://new_user:new_password@postgres:5432/mydatabase?sslmode=disable"
	rabbitMQURL := "amqp://guest:guest@rabbitmq:5672/"

	for {
		err := healthCheck(pgConnString, rabbitMQURL)
		if err == nil {
			log.Println("Health check passed. Continuing with application startup.")
			break
		}
		log.Printf("Health check failed: %v. Retrying in 5 seconds...\n", err)
		time.Sleep(5 * time.Second)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgListener, err := pglistener.New(pgConnString)
	if err != nil {
		log.Fatalf("Failed to create PostgreSQL listener: %v", err)
	}
	defer pgListener.Close()

	var rmqPublisher *rabbitmq.RabbitMQPublisher
	for i := 0; i < 5; i++ {
		rmqPublisher, err = rabbitmq.New(rabbitMQURL)
		if err == nil {
			break
		}
		log.Printf("Failed to create RabbitMQ publisher (attempt %d/5): %v. Retrying in 5 seconds...\n", i+1, err)
		time.Sleep(5 * time.Second)
	}
	if rmqPublisher == nil {
		log.Fatalf("Failed to create RabbitMQ publisher after 5 attempts")
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
			} else {
				log.Printf("Successfully published change: %v", change)
			}
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
