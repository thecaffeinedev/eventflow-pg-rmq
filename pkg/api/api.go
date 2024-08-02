package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/models"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/pglistener"
	"github.com/thecaffeinedev/eventflow-pg-rmq/pkg/rabbitmq"
)

type API struct {
	PgListener   *pglistener.PgListener
	RmqPublisher *rabbitmq.RabbitMQPublisher
	DB           *pgxpool.Pool
	eventChan    chan models.Event
	clients      map[chan string]bool
	clientsMu    sync.Mutex
}

func New(pgListener *pglistener.PgListener, rmqPublisher *rabbitmq.RabbitMQPublisher, db *pgxpool.Pool) *API {
	return &API{
		PgListener:   pgListener,
		RmqPublisher: rmqPublisher,
		DB:           db,
		eventChan:    make(chan models.Event),
		clients:      make(map[chan string]bool),
	}
}

func (a *API) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.serveHTML)
	mux.HandleFunc("/events", a.streamEvents)
	mux.HandleFunc("/health", a.HealthCheck)
	mux.HandleFunc("/users", a.HandleUsers)
	go a.broadcastEvents()
	return mux
}

func (a *API) serveHTML(w http.ResponseWriter, r *http.Request) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Error getting current working directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	indexPath := filepath.Join(cwd, "static", "index.html")

	_, err = os.Stat(indexPath)
	if os.IsNotExist(err) {
		log.Printf("index.html not found at %s", indexPath)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	tmpl, err := template.ParseFiles(indexPath)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (a *API) streamEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan string)
	a.clientsMu.Lock()
	a.clients[clientChan] = true
	a.clientsMu.Unlock()

	defer func() {
		a.clientsMu.Lock()
		delete(a.clients, clientChan)
		a.clientsMu.Unlock()
		close(clientChan)
	}()

	for {
		select {
		case msg := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (a *API) broadcastEvents() {
	for event := range a.eventChan {
		eventJSON, err := json.Marshal(event)
		if err != nil {
			continue
		}
		a.clientsMu.Lock()
		for clientChan := range a.clients {
			select {
			case clientChan <- string(eventJSON):
			default:
			}
		}
		a.clientsMu.Unlock()
	}
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

	// Parse the form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")

	// Validate user data
	if name == "" || email == "" {
		http.Error(w, "Name and email are required", http.StatusBadRequest)
		return
	}

	// Insert the user into the database
	var id int
	err = a.DB.QueryRow(context.Background(),
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		name, email).Scan(&id)
	if err != nil {
		log.Printf("Failed to insert user: %v", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	// Create the event
	event := models.Event{
		Action:    "I",
		TableName: "public.users",
		New: map[string]interface{}{
			"id":    id,
			"name":  name,
			"email": email,
		},
		ID: id,
		Diff: map[string]interface{}{
			"id":    id,
			"name":  name,
			"email": email,
		},
	}

	// Broadcast the event
	select {
	case a.eventChan <- event:
		log.Println("Event broadcasted successfully")
	default:
		log.Println("Failed to broadcast event: channel full")
	}

	// Prepare the response
	response := map[string]interface{}{
		"status": "User created successfully",
		"id":     id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
