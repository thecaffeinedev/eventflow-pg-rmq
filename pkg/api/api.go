package api

import (
	"encoding/json"
	"net/http"

	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

type API struct {
	PgListener   *pglistener.PgListener
	RmqPublisher *rabbitmq.RabbitMQPublisher
}

func New(pgListener *pglistener.PgListener, rmqPublisher *rabbitmq.RabbitMQPublisher) *API {
	return &API{
		PgListener:   pgListener,
		RmqPublisher: rmqPublisher,
	}
}

func (a *API) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", a.HealthCheck)
	mux.HandleFunc("/trigger", a.TriggerEvent)
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

func (a *API) TriggerEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event models.Event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = a.RmqPublisher.Publish(event)
	if err != nil {
		http.Error(w, "Failed to publish event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "Event published successfully"})
}
