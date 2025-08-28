package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Event represents a calendar event
type Event struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Location    string    `json:"location"`
}

// CreateEventRequest represents the request payload for creating an event
type CreateEventRequest struct {
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Start       string `json:"start"` // RFC3339 format
	End         string `json:"end"`   // RFC3339 format
	Location    string `json:"location"`
}

// OAuth2 configuration
var oauth2Config *oauth2.Config

// Prometheus metrics
var (
	calendarRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "calendar_requests_total",
			Help: "Total number of calendar API requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	calendarRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "calendar_request_duration_seconds",
			Help: "Duration of calendar API requests",
		},
		[]string{"method", "endpoint"},
	)
	googleAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "google_api_calls_total",
			Help: "Total number of Google API calls",
		},
		[]string{"operation", "status"},
	)
)

func init() {
	prometheus.MustRegister(calendarRequestsTotal)
	prometheus.MustRegister(calendarRequestDuration)
	prometheus.MustRegister(googleAPICallsTotal)
}

func main() {
	// Initialize OAuth2 configuration
	initOAuth2Config()

	router := mux.NewRouter()

	// Calendar endpoints
	router.HandleFunc("/events", handleGetEvents).Methods("GET")
	router.HandleFunc("/events", handleCreateEvent).Methods("POST")
	router.HandleFunc("/auth", handleAuth).Methods("GET")
	router.HandleFunc("/callback", handleCallback).Methods("GET")
	router.HandleFunc("/health", handleHealth).Methods("GET")

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "8082")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Calendar Service starting on port %s", port)
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

func initOAuth2Config() {
	clientID := getEnv("GOOGLE_CLIENT_ID", "")
	clientSecret := getEnv("GOOGLE_CLIENT_SECRET", "")
	redirectURL := getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8082/callback")

	if clientID == "" || clientSecret == "" {
		log.Println("Warning: Google OAuth2 credentials not configured")
		log.Println("Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables")
	}

	oauth2Config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{calendar.CalendarScope},
		Endpoint:     google.Endpoint,
	}
}

func handleGetEvents(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		calendarRequestDuration.WithLabelValues("GET", "/events").Observe(time.Since(start).Seconds())
	}()

	// Get query parameters
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	// For demo purposes, return mock data if no OAuth token is available
	accessToken := getAccessToken(r)
	if accessToken == "" {
		calendarRequestsTotal.WithLabelValues("GET", "/events", "mock").Inc()
		events := getMockEvents(startDate, endDate)
		writeJSONResponse(w, map[string]interface{}{"events": events})
		return
	}

	// Get real events from Google Calendar
	events, err := getGoogleCalendarEvents(accessToken, startDate, endDate)
	if err != nil {
		calendarRequestsTotal.WithLabelValues("GET", "/events", "error").Inc()
		googleAPICallsTotal.WithLabelValues("list_events", "error").Inc()
		http.Error(w, fmt.Sprintf("Failed to get events: %v", err), http.StatusInternalServerError)
		return
	}

	calendarRequestsTotal.WithLabelValues("GET", "/events", "success").Inc()
	googleAPICallsTotal.WithLabelValues("list_events", "success").Inc()
	writeJSONResponse(w, map[string]interface{}{"events": events})
}

func handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		calendarRequestDuration.WithLabelValues("POST", "/events").Observe(time.Since(start).Seconds())
	}()

	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		calendarRequestsTotal.WithLabelValues("POST", "/events", "error").Inc()
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Summary == "" || req.Start == "" || req.End == "" {
		calendarRequestsTotal.WithLabelValues("POST", "/events", "error").Inc()
		http.Error(w, "Summary, start, and end are required", http.StatusBadRequest)
		return
	}

	// For demo purposes, return mock data if no OAuth token is available
	accessToken := getAccessToken(r)
	if accessToken == "" {
		calendarRequestsTotal.WithLabelValues("POST", "/events", "mock").Inc()
		event := createMockEvent(req)
		w.WriteHeader(http.StatusCreated)
		writeJSONResponse(w, event)
		return
	}

	// Create real event in Google Calendar
	event, err := createGoogleCalendarEvent(accessToken, req)
	if err != nil {
		calendarRequestsTotal.WithLabelValues("POST", "/events", "error").Inc()
		googleAPICallsTotal.WithLabelValues("create_event", "error").Inc()
		http.Error(w, fmt.Sprintf("Failed to create event: %v", err), http.StatusInternalServerError)
		return
	}

	calendarRequestsTotal.WithLabelValues("POST", "/events", "success").Inc()
	googleAPICallsTotal.WithLabelValues("create_event", "success").Inc()
	w.WriteHeader(http.StatusCreated)
	writeJSONResponse(w, event)
}

func handleAuth(w http.ResponseWriter, r *http.Request) {
	url := oauth2Config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	writeJSONResponse(w, map[string]string{"auth_url": url})
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Authorization code not provided", http.StatusBadRequest)
		return
	}

	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange code: %v", err), http.StatusInternalServerError)
		return
	}

	// In a real application, you would store this token securely
	// For demo purposes, we'll just return it
	writeJSONResponse(w, map[string]interface{}{
		"access_token": token.AccessToken,
		"token_type":   token.TokenType,
		"expires_in":   token.Expiry.Unix(),
	})
}

func getGoogleCalendarEvents(accessToken, startDate, endDate string) ([]Event, error) {
	ctx := context.Background()

	// Create OAuth2 token
	token := &oauth2.Token{AccessToken: accessToken}
	client := oauth2Config.Client(ctx, token)

	// Create Calendar service
	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	// Build the events list call
	call := service.Events.List("primary").SingleEvents(true).OrderBy("startTime")

	if startDate != "" {
		call = call.TimeMin(startDate)
	}
	if endDate != "" {
		call = call.TimeMax(endDate)
	}

	// Execute the call
	events, err := call.Do()
	if err != nil {
		return nil, err
	}

	// Convert to our Event format
	var result []Event
	for _, item := range events.Items {
		start, _ := time.Parse(time.RFC3339, item.Start.DateTime)
		if item.Start.DateTime == "" {
			start, _ = time.Parse("2006-01-02", item.Start.Date)
		}

		end, _ := time.Parse(time.RFC3339, item.End.DateTime)
		if item.End.DateTime == "" {
			end, _ = time.Parse("2006-01-02", item.End.Date)
		}

		result = append(result, Event{
			ID:          item.Id,
			Summary:     item.Summary,
			Description: item.Description,
			Start:       start,
			End:         end,
			Location:    item.Location,
		})
	}

	return result, nil
}

func createGoogleCalendarEvent(accessToken string, req CreateEventRequest) (*Event, error) {
	ctx := context.Background()

	// Create OAuth2 token
	token := &oauth2.Token{AccessToken: accessToken}
	client := oauth2Config.Client(ctx, token)

	// Create Calendar service
	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	// Create the event
	event := &calendar.Event{
		Summary:     req.Summary,
		Description: req.Description,
		Location:    req.Location,
		Start: &calendar.EventDateTime{
			DateTime: req.Start,
		},
		End: &calendar.EventDateTime{
			DateTime: req.End,
		},
	}

	// Insert the event
	createdEvent, err := service.Events.Insert("primary", event).Do()
	if err != nil {
		return nil, err
	}

	// Convert to our Event format
	start, _ := time.Parse(time.RFC3339, createdEvent.Start.DateTime)
	end, _ := time.Parse(time.RFC3339, createdEvent.End.DateTime)

	return &Event{
		ID:          createdEvent.Id,
		Summary:     createdEvent.Summary,
		Description: createdEvent.Description,
		Start:       start,
		End:         end,
		Location:    createdEvent.Location,
	}, nil
}

func getMockEvents(startDate, endDate string) []Event {
	now := time.Now()
	return []Event{
		{
			ID:          "mock-1",
			Summary:     "Team Meeting",
			Description: "Weekly team synchronization",
			Start:       now.Add(1 * time.Hour),
			End:         now.Add(2 * time.Hour),
			Location:    "Conference Room A",
		},
		{
			ID:          "mock-2",
			Summary:     "Project Review",
			Description: "Q4 project review meeting",
			Start:       now.Add(25 * time.Hour),
			End:         now.Add(26 * time.Hour),
			Location:    "Online",
		},
	}
}

func createMockEvent(req CreateEventRequest) Event {
	start, _ := time.Parse(time.RFC3339, req.Start)
	end, _ := time.Parse(time.RFC3339, req.End)

	return Event{
		ID:          fmt.Sprintf("mock-%d", time.Now().Unix()),
		Summary:     req.Summary,
		Description: req.Description,
		Start:       start,
		End:         end,
		Location:    req.Location,
	}
}

func getAccessToken(r *http.Request) string {
	// Try to get token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	// Try to get token from query parameter
	return r.URL.Query().Get("access_token")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, map[string]string{"status": "healthy"})
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
