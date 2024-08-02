# EventFlow: PostgreSQL to RabbitMQ

EventFlow is a simple Change Data Capture (CDC) tool that watches for changes in a PostgreSQL database and sends those changes to RabbitMQ. It captures database changes and broadcasts them in real-time, facilitating event-driven architectures and data synchronization across distributed systems.

## Motivation

I've been fascinated by the concept of Change Data Capture for a long time. As systems grow more complex and data-driven, the need for real-time data synchronization becomes crucial. I created EventFlow-PG-RMQ to deepen my understanding of CDC processes and to explore how they can be implemented in a practical, scalable manner.

## What is Change Data Capture (CDC)? 🤔

Change Data Capture is a set of software design patterns used to determine and track the data that has changed so that action can be taken using the changed data.

## Features

- Real-time Change Data Capture from PostgreSQL
- Integration with RabbitMQ for event broadcasting
- HTMX-powered web interface for live demonstration and testing
- Docker containerization for easy deployment and scalability

## Architecture

1. **PostgreSQL Database**: Configured with triggers to notify on data changes.
2. **Go Application**: 
   - Listens for PostgreSQL notifications
   - Processes and formats change events
   - Publishes events to RabbitMQ
3. **RabbitMQ**: Acts as the message broker for distributing change events.
4. **Web Interface**: Provides a real-time visualization of the CDC process and allows for manual data manipulation.

## Getting Started

### Prerequisites

- Docker
- Docker Compose

### Installation and Setup

1. Clone the repository:
   ```
   git clone https://github.com/thecaffeinedev/eventflow-pg-rmq.git
   ```

2. Navigate to the project directory:
   ```
   cd eventflow-pg-rmq
   ```

3. Start the application using Docker Compose:
   ```
   docker-compose up --build
   ```

4. Access the web interface at `http://localhost:8080`

## Usage

The web interface provides two main functionalities:

1. **User Management**: Add new users to the PostgreSQL database.
2. **Event Stream**: View real-time CDC events as they occur in the database.

To test the system:
1. Use the form on the left panel to add a new user.
2. Observe the corresponding CDC event appear in the right panel's event stream.

## Configuration

Key configuration options are available through environment variables:

- `PG_CONNECTION_STRING`: PostgreSQL connection string
- `RABBITMQ_URL`: RabbitMQ connection URL

Refer to the `docker-compose.yml` file for default values and additional configuration options.

## Development

This project is structured as follows:

```
.
├── cmd
│   └── main.go
├── pkg
│   ├── api
│   ├── app
│   ├── health
│   ├── models
│   ├── pglistener
│   └── rabbitmq
├── migrations
├── static
│   └── index.html
├── Dockerfile
└── docker-compose.yml
└── .env
```

## Future Enhancements
As I continue to explore CDC concepts, I plan to enhance EventFlow-PG-RMQ with the following features:

- Support for additional database operations.
- UI Improvements
- Implementation of more advanced CDC patterns like snapshot and incremental snapshot
- Integration with cloud-based message queues and event streaming platforms
- Performance optimizations for handling high-volume data changes

## References and Further Reading 📚

- [Change Data Capture: What it is and How it Works](https://www.striim.com/blog/change-data-capture-cdc-what-it-is-and-how-it-works/)
- [Debezium Documentation](https://debezium.io/documentation/reference/1.5/index.html)
- [PostgreSQL Logical Replication](https://www.postgresql.org/docs/current/logical-replication.html)
- [RabbitMQ Tutorials](https://www.rabbitmq.com/getstarted.html)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
