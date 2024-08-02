package rabbitmq

import (
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
)

type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(url string) (*RabbitMQPublisher, error) {
	log.Printf("Connecting to RabbitMQ at %s", url)
	conn, err := amqp.Dial(url)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %v", err)
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %v", err)
	}
	log.Println("Successfully connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %v", err)
		return nil, fmt.Errorf("failed to open a channel: %v", err)
	}
	log.Println("Successfully opened a channel")

	log.Println("Declaring exchange 'cdc_events'")
	err = ch.ExchangeDeclare(
		"cdc_events", // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Printf("Failed to declare an exchange: %v", err)
		return nil, fmt.Errorf("failed to declare an exchange: %v", err)
	}
	log.Println("Successfully declared exchange 'cdc_events'")

	return &RabbitMQPublisher{
		conn:    conn,
		channel: ch,
	}, nil
}

func (r *RabbitMQPublisher) Publish(event models.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	log.Printf("Publishing event: %s", string(body))

	err = r.channel.Publish(
		"cdc_events",    // exchange
		event.TableName, // routing key
		false,           // mandatory
		false,           // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
	if err != nil {
		log.Printf("Failed to publish a message: %v", err)
		return fmt.Errorf("failed to publish a message: %v", err)
	}
	log.Println("Successfully published event")

	return nil
}

func (r *RabbitMQPublisher) Close() error {
	log.Println("Closing RabbitMQ channel and connection")
	if err := r.channel.Close(); err != nil {
		log.Printf("Failed to close channel: %v", err)
		return err
	}
	if err := r.conn.Close(); err != nil {
		log.Printf("Failed to close connection: %v", err)
		return err
	}
	log.Println("Successfully closed RabbitMQ channel and connection")
	return nil
}
