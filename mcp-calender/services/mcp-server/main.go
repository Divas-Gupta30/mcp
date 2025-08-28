package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MCP Protocol structures
type MCPRequest struct {
	ID     string                 `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Service endpoints configuration
var serviceEndpoints = map[string]string{
	"task-service":     getEnv("TASK_SERVICE_URL", "http://task-service:8081"),
	"calendar-service": getEnv("CALENDAR_SERVICE_URL", "http://calendar-service:8082"),
	"weather-service":  getEnv("WEATHER_SERVICE_URL", "http://weather-service:8083"),
}

// Prometheus metrics
var (
	mcpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_requests_total",
			Help: "Total number of MCP requests",
		},
		[]string{"method", "status"},
	)
	mcpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "mcp_request_duration_seconds",
			Help: "Duration of MCP requests",
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(mcpRequestsTotal)
	prometheus.MustRegister(mcpRequestDuration)
}

func main() {
	router := mux.NewRouter()

	// MCP endpoints
	router.HandleFunc("/mcp", handleMCP).Methods("POST")
	router.HandleFunc("/tools/list", handleToolsList).Methods("GET")
	router.HandleFunc("/health", handleHealth).Methods("GET")

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "8080")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("MCP Server starting on port %s", port)
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

func handleMCP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErrorResponse(w, req.ID, -32700, "Parse error")
		mcpRequestsTotal.WithLabelValues(req.Method, "error").Inc()
		return
	}

	defer func() {
		mcpRequestDuration.WithLabelValues(req.Method).Observe(time.Since(start).Seconds())
	}()

	var response MCPResponse
	response.ID = req.ID

	switch req.Method {
	case "tools/call":
		response = handleToolCall(req)
	case "tools/list":
		response = handleToolsListMCP(req)
	default:
		response = MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		mcpRequestsTotal.WithLabelValues(req.Method, "error").Inc()
		writeJSONResponse(w, response)
		return
	}

	status := "success"
	if response.Error != nil {
		status = "error"
	}
	mcpRequestsTotal.WithLabelValues(req.Method, status).Inc()

	writeJSONResponse(w, response)
}

func handleToolCall(req MCPRequest) MCPResponse {
	toolName, ok := req.Params["name"].(string)
	if !ok {
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid tool name",
			},
		}
	}

	arguments, _ := req.Params["arguments"].(map[string]interface{})

	switch toolName {
	case "get_tasks":
		return callTaskService("GET", "/tasks", nil)
	case "add_task":
		return callTaskService("POST", "/tasks", arguments)
	case "get_calendar_events":
		return callCalendarService("GET", "/events", arguments)
	case "get_weather":
		city, _ := arguments["city"].(string)
		return callWeatherService("GET", fmt.Sprintf("/weather?city=%s", city), nil)
	default:
		return MCPResponse{
			ID: req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Tool not found",
			},
		}
	}
}

func handleToolsListMCP(req MCPRequest) MCPResponse {
	tools := getAvailableTools()
	return MCPResponse{
		ID:     req.ID,
		Result: map[string]interface{}{"tools": tools},
	}
}

func handleToolsList(w http.ResponseWriter, r *http.Request) {
	tools := getAvailableTools()
	writeJSONResponse(w, map[string]interface{}{"tools": tools})
}

func getAvailableTools() []Tool {
	return []Tool{
		{
			Name:        "get_tasks",
			Description: "Retrieve all tasks",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "add_task",
			Description: "Add a new task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"title": map[string]interface{}{
						"type":        "string",
						"description": "Task title",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Task description",
					},
					"priority": map[string]interface{}{
						"type":        "string",
						"description": "Task priority (low, medium, high)",
					},
				},
				"required": []string{"title"},
			},
		},
		{
			Name:        "get_calendar_events",
			Description: "Get calendar events",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"start_date": map[string]interface{}{
						"type":        "string",
						"description": "Start date (YYYY-MM-DD)",
					},
					"end_date": map[string]interface{}{
						"type":        "string",
						"description": "End date (YYYY-MM-DD)",
					},
				},
			},
		},
		{
			Name:        "get_weather",
			Description: "Get weather information for a city",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"city": map[string]interface{}{
						"type":        "string",
						"description": "City name",
					},
				},
				"required": []string{"city"},
			},
		},
	}
}

func callTaskService(method, path string, body interface{}) MCPResponse {
	return callService("task-service", method, path, body)
}

func callCalendarService(method, path string, body interface{}) MCPResponse {
	return callService("calendar-service", method, path, body)
}

func callWeatherService(method, path string, body interface{}) MCPResponse {
	return callService("weather-service", method, path, body)
}

func callService(serviceName, method, path string, body interface{}) MCPResponse {
	baseURL, exists := serviceEndpoints[serviceName]
	if !exists {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32001,
				Message: fmt.Sprintf("Service %s not configured", serviceName),
			},
		}
	}

	// Prepare request body
	var reqBody io.Reader
	if body != nil && (method == "POST" || method == "PATCH") {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return MCPResponse{
				Error: &MCPError{
					Code:    -32002,
					Message: fmt.Sprintf("Failed to marshal request body: %v", err),
				},
			}
		}
		reqBody = bytes.NewBuffer(bodyBytes)
	}

	// Create HTTP request
	url := baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32003,
				Message: fmt.Sprintf("Failed to create request: %v", err),
			},
		}
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Make HTTP request with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32004,
				Message: fmt.Sprintf("Service request failed: %v", err),
			},
		}
	}
	defer resp.Body.Close()

	// Read response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32005,
				Message: fmt.Sprintf("Failed to read response: %v", err),
			},
		}
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return MCPResponse{
			Error: &MCPError{
				Code:    -32006,
				Message: fmt.Sprintf("Service returned error %d: %s", resp.StatusCode, string(responseBody)),
			},
		}
	}

	// Parse JSON response
	var result interface{}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &result); err != nil {
			// If JSON parsing fails, return raw response
			result = map[string]interface{}{
				"raw_response": string(responseBody),
				"content_type": resp.Header.Get("Content-Type"),
			}
		}
	}

	return MCPResponse{
		Result: result,
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, map[string]string{"status": "healthy"})
}

func writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeErrorResponse(w http.ResponseWriter, id string, code int, message string) {
	response := MCPResponse{
		ID: id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	writeJSONResponse(w, response)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
