package app

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/api"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/health"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

type App struct {
	PgListener   *pglistener.PgListener
	RmqPublisher *rabbitmq.RabbitMQPublisher
	API          *api.API
	DB           *pgxpool.Pool
}

func New(pgConnString, rabbitMQURL string) (*App, error) {
	for {
		err := health.Check(pgConnString, rabbitMQURL)
		if err == nil {
			log.Println("Health check passed. Continuing with application startup.")
			break
		}
		log.Printf("Health check failed: %v. Retrying in 5 seconds...\n", err)
		time.Sleep(5 * time.Second)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, pgConnString)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(ctx); err != nil {
		return nil, err
	}

	pgListener, err := pglistener.New(pgConnString)
	if err != nil {
		return nil, err
	}

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
		return nil, err
	}

	apiHandler := api.New(pgListener, rmqPublisher, db)

	return &App{
		PgListener:   pgListener,
		RmqPublisher: rmqPublisher,
		API:          apiHandler,
		DB:           db,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	go func() {
		changes, err := a.PgListener.Listen(ctx)
		if err != nil {
			log.Printf("Failed to start listening: %v", err)
			return
		}

		for change := range changes {
			if err := a.RmqPublisher.Publish(change); err != nil {
				log.Printf("Failed to publish change: %v", err)
			} else {
				log.Printf("Successfully published change: %v", change)
			}
		}
	}()

	server := &http.Server{
		Addr:    ":8080",
		Handler: a.API.SetupRoutes(),
	}

	go func() {
		log.Println("Starting API server on :8080")
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	return server.Shutdown(context.Background())
}

func (a *App) Close() {
	if a.PgListener != nil {
		a.PgListener.Close()
	}
	if a.RmqPublisher != nil {
		a.RmqPublisher.Close()
	}
	if a.DB != nil {
		a.DB.Close()
	}
}
