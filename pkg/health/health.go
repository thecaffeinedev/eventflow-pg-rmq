package health

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"
)

func Check(pgConnString, rabbitMQURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pgConn, err := pgx.Connect(ctx, pgConnString)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}
	defer pgConn.Close(ctx)

	err = pgConn.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %v", err)
	}

	rabbitConn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitConn.Close()

	return nil
}
