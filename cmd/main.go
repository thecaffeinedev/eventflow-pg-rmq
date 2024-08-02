package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/app"
)

func runMigrations(dbURL string) error {
	log.Println("Starting database migrations...")
	m, err := migrate.New(
		"file://./migrations",
		dbURL,
	)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	log.Println("Database migrations completed successfully")
	return nil
}

func main() {
	log.Println("Starting application...")

	pgConnString := os.Getenv("PG_CONNECTION_STRING")
	rabbitMQURL := os.Getenv("RABBITMQ_URL")

	// Run migrations
	log.Println("Running database migrations...")
	if err := runMigrations(pgConnString); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

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
