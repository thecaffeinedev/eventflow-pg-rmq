package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

type API struct {
	PgListener   *pglistener.PgListener
	RmqPublisher *rabbitmq.RabbitMQPublisher
	DB           *pgxpool.Pool
}

func New(pgListener *pglistener.PgListener, rmqPublisher *rabbitmq.RabbitMQPublisher, db *pgxpool.Pool) *API {
	return &API{
		PgListener:   pgListener,
		RmqPublisher: rmqPublisher,
		DB:           db,
	}
}

func (a *API) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.HealthCheck)
	mux.HandleFunc("/users", a.HandleUsers)
	return mux
}

func (a *API) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (a *API) HandleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user models.User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Insert the user into the database
	_, err = a.DB.Exec(context.Background(), "INSERT INTO users (name, email) VALUES ($1, $2)", user.Name, user.Email)
	if err != nil {
		http.Error(w, "Failed to insert user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "User created successfully"})
}
