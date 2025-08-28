package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Task represents a task in the system
type Task struct {
	ID          int       `json:"id" db:"id"`
	Title       string    `json:"title" db:"title"`
	Description string    `json:"description" db:"description"`
	Priority    string    `json:"priority" db:"priority"`
	Status      string    `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// CreateTaskRequest represents the request payload for creating a task
type CreateTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
}

// UpdateTaskRequest represents the request payload for updating a task
type UpdateTaskRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Priority    *string `json:"priority,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// Database connection
var db *sql.DB

// Prometheus metrics
var (
	taskRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "task_requests_total",
			Help: "Total number of task API requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	taskRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "task_request_duration_seconds",
			Help: "Duration of task API requests",
		},
		[]string{"method", "endpoint"},
	)
	tasksInDB = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "tasks_in_database_total",
			Help: "Total number of tasks in database",
		},
	)
)

func init() {
	prometheus.MustRegister(taskRequestsTotal)
	prometheus.MustRegister(taskRequestDuration)
	prometheus.MustRegister(tasksInDB)
}

func main() {
	// Initialize database
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create tables
	if err := createTables(); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	router := mux.NewRouter()

	// Task endpoints
	router.HandleFunc("/tasks", handleGetTasks).Methods("GET")
	router.HandleFunc("/tasks", handleCreateTask).Methods("POST")
	router.HandleFunc("/tasks/{id}", handleUpdateTask).Methods("PATCH")
	router.HandleFunc("/tasks/{id}", handleDeleteTask).Methods("DELETE")
	router.HandleFunc("/health", handleHealth).Methods("GET")

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	// Start metrics updater
	go updateMetrics()

	port := getEnv("PORT", "8081")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Task Service starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

func initDB() error {
	dbURL := getEnv("DATABASE_URL", "postgres://taskuser:taskpass@postgres:5432/taskdb?sslmode=disable")

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		return err
	}

	// Test connection
	if err = db.Ping(); err != nil {
		return err
	}

	log.Println("Connected to PostgreSQL database")
	return nil
}

func createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		title VARCHAR(255) NOT NULL,
		description TEXT,
		priority VARCHAR(20) DEFAULT 'medium',
		status VARCHAR(20) DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE OR REPLACE FUNCTION update_updated_at_column()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = CURRENT_TIMESTAMP;
		RETURN NEW;
	END;
	$$ language 'plpgsql';
	
	DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
	CREATE TRIGGER update_tasks_updated_at
		BEFORE UPDATE ON tasks
		FOR EACH ROW
		EXECUTE FUNCTION update_updated_at_column();
	`

	_, err := db.Exec(query)
	if err != nil {
		return err
	}

	log.Println("Database tables created successfully")
	return nil
}

func handleGetTasks(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskRequestDuration.WithLabelValues("GET", "/tasks").Observe(time.Since(start).Seconds())
	}()

	rows, err := db.Query(`
		SELECT id, title, description, priority, status, created_at, updated_at 
		FROM tasks 
		ORDER BY created_at DESC
	`)
	if err != nil {
		taskRequestsTotal.WithLabelValues("GET", "/tasks", "error").Inc()
		http.Error(w, "Failed to query tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID, &task.Title, &task.Description,
			&task.Priority, &task.Status, &task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			taskRequestsTotal.WithLabelValues("GET", "/tasks", "error").Inc()
			http.Error(w, "Failed to scan task", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	taskRequestsTotal.WithLabelValues("GET", "/tasks", "success").Inc()
	writeJSONResponse(w, map[string]interface{}{"tasks": tasks})
}

func handleCreateTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskRequestDuration.WithLabelValues("POST", "/tasks").Observe(time.Since(start).Seconds())
	}()

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		taskRequestsTotal.WithLabelValues("POST", "/tasks", "error").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		taskRequestsTotal.WithLabelValues("POST", "/tasks", "error").Inc()
		http.Error(w, "Title is required", http.StatusBadRequest)
		return
	}

	if req.Priority == "" {
		req.Priority = "medium"
	}

	var task Task
	err := db.QueryRow(`
		INSERT INTO tasks (title, description, priority, status) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, title, description, priority, status, created_at, updated_at
	`, req.Title, req.Description, req.Priority, "pending").Scan(
		&task.ID, &task.Title, &task.Description,
		&task.Priority, &task.Status, &task.CreatedAt, &task.UpdatedAt,
	)

	if err != nil {
		taskRequestsTotal.WithLabelValues("POST", "/tasks", "error").Inc()
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	taskRequestsTotal.WithLabelValues("POST", "/tasks", "success").Inc()
	w.WriteHeader(http.StatusCreated)
	writeJSONResponse(w, task)
}

func handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskRequestDuration.WithLabelValues("PATCH", "/tasks/:id").Observe(time.Since(start).Seconds())
	}()

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build dynamic update query
	setParts := []string{}
	args := []interface{}{}
	argIndex := 1

	if req.Title != nil {
		setParts = append(setParts, "title = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Title)
		argIndex++
	}
	if req.Description != nil {
		setParts = append(setParts, "description = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Description)
		argIndex++
	}
	if req.Priority != nil {
		setParts = append(setParts, "priority = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Priority)
		argIndex++
	}
	if req.Status != nil {
		setParts = append(setParts, "status = $"+strconv.Itoa(argIndex))
		args = append(args, *req.Status)
		argIndex++
	}

	if len(setParts) == 0 {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	query := "UPDATE tasks SET " + strings.Join(setParts, ", ") + " WHERE id = $" + strconv.Itoa(argIndex)
	args = append(args, id)

	result, err := db.Exec(query, args...)
	if err != nil {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "Failed to update task", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Get updated task
	var task Task
	err = db.QueryRow(`
		SELECT id, title, description, priority, status, created_at, updated_at 
		FROM tasks WHERE id = $1
	`, id).Scan(
		&task.ID, &task.Title, &task.Description,
		&task.Priority, &task.Status, &task.CreatedAt, &task.UpdatedAt,
	)

	if err != nil {
		taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "error").Inc()
		http.Error(w, "Failed to retrieve updated task", http.StatusInternalServerError)
		return
	}

	taskRequestsTotal.WithLabelValues("PATCH", "/tasks/:id", "success").Inc()
	writeJSONResponse(w, task)
}

func handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		taskRequestDuration.WithLabelValues("DELETE", "/tasks/:id").Observe(time.Since(start).Seconds())
	}()

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		taskRequestsTotal.WithLabelValues("DELETE", "/tasks/:id", "error").Inc()
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		taskRequestsTotal.WithLabelValues("DELETE", "/tasks/:id", "error").Inc()
		http.Error(w, "Failed to delete task", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		taskRequestsTotal.WithLabelValues("DELETE", "/tasks/:id", "error").Inc()
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	taskRequestsTotal.WithLabelValues("DELETE", "/tasks/:id", "success").Inc()
	w.WriteHeader(http.StatusNoContent)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := db.Ping(); err != nil {
		http.Error(w, "Database connection failed", http.StatusServiceUnavailable)
		return
	}

	writeJSONResponse(w, map[string]string{"status": "healthy"})
}

func updateMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count)
		if err == nil {
			tasksInDB.Set(float64(count))
		}
	}
}

func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
