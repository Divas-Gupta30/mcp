package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// WeatherData represents weather information
type WeatherData struct {
	City        string  `json:"city"`
	Country     string  `json:"country"`
	Temperature float64 `json:"temperature"`
	Description string  `json:"description"`
	Humidity    int     `json:"humidity"`
	WindSpeed   float64 `json:"wind_speed"`
	Timestamp   int64   `json:"timestamp"`
	Source      string  `json:"source"` // "api" or "cache"
}

// OpenWeatherMap API response structure
type OpenWeatherResponse struct {
	Name string `json:"name"`
	Sys  struct {
		Country string `json:"country"`
	} `json:"sys"`
	Main struct {
		Temp     float64 `json:"temp"`
		Humidity int     `json:"humidity"`
	} `json:"main"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
}

// Redis client
var redisClient *redis.Client

// Prometheus metrics
var (
	weatherRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "weather_requests_total",
			Help: "Total number of weather API requests",
		},
		[]string{"method", "endpoint", "status"},
	)
	weatherRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "weather_request_duration_seconds",
			Help: "Duration of weather API requests",
		},
		[]string{"method", "endpoint"},
	)
	cacheHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		},
	)
	cacheMissesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		},
	)
	externalAPICallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "external_api_calls_total",
			Help: "Total number of external API calls",
		},
		[]string{"provider", "status"},
	)
)

func init() {
	prometheus.MustRegister(weatherRequestsTotal)
	prometheus.MustRegister(weatherRequestDuration)
	prometheus.MustRegister(cacheHitsTotal)
	prometheus.MustRegister(cacheMissesTotal)
	prometheus.MustRegister(externalAPICallsTotal)
}

func main() {
	// Initialize Redis
	initRedis()
	defer redisClient.Close()

	router := mux.NewRouter()

	// Weather endpoints
	router.HandleFunc("/weather", handleGetWeather).Methods("GET")
	router.HandleFunc("/health", handleHealth).Methods("GET")

	// Metrics endpoint
	router.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "8083")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		log.Printf("Weather Service starting on port %s", port)
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

func initRedis() {
	redisURL := getEnv("REDIS_URL", "redis:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := 0

	if dbStr := getEnv("REDIS_DB", "0"); dbStr != "" {
		if db, err := strconv.Atoi(dbStr); err == nil {
			redisDB = db
		}
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: redisPassword,
		DB:       redisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
		log.Println("Weather service will work without caching")
	} else {
		log.Println("Connected to Redis cache")
	}
}

func handleGetWeather(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		weatherRequestDuration.WithLabelValues("GET", "/weather").Observe(time.Since(start).Seconds())
	}()

	city := r.URL.Query().Get("city")
	if city == "" {
		weatherRequestsTotal.WithLabelValues("GET", "/weather", "error").Inc()
		http.Error(w, "City parameter is required", http.StatusBadRequest)
		return
	}

	// Check cache first
	weatherData, err := getWeatherFromCache(city)
	if err == nil {
		cacheHitsTotal.Inc()
		weatherRequestsTotal.WithLabelValues("GET", "/weather", "success").Inc()
		writeJSONResponse(w, weatherData)
		return
	}

	cacheMissesTotal.Inc()

	// Get from OpenWeatherMap API
	weatherData, err = getWeatherFromAPI(city)
	if err != nil {
		weatherRequestsTotal.WithLabelValues("GET", "/weather", "error").Inc()
		externalAPICallsTotal.WithLabelValues("openweathermap", "error").Inc()
		http.Error(w, fmt.Sprintf("Failed to get weather data: %v", err), http.StatusInternalServerError)
		return
	}

	// Cache the result
	if err := cacheWeatherData(city, weatherData); err != nil {
		log.Printf("Warning: Failed to cache weather data: %v", err)
	}

	weatherRequestsTotal.WithLabelValues("GET", "/weather", "success").Inc()
	externalAPICallsTotal.WithLabelValues("openweathermap", "success").Inc()
	log.Printf("api weather data: %v", weatherData)
	writeJSONResponse(w, weatherData)
}

func getWeatherFromCache(city string) (*WeatherData, error) {
	if redisClient == nil {
		return nil, fmt.Errorf("redis not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf("weather:%s", city)
	data, err := redisClient.Get(ctx, cacheKey).Result()
	if err != nil {
		return nil, err
	}

	var weatherData WeatherData
	if err := json.Unmarshal([]byte(data), &weatherData); err != nil {
		return nil, err
	}

	weatherData.Source = "cache"
	return &weatherData, nil
}

func cacheWeatherData(city string, data *WeatherData) error {
	if redisClient == nil {
		return nil // No error if Redis is not available
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cacheKey := fmt.Sprintf("weather:%s", city)
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Cache for 10 minutes
	ttl := 10 * time.Minute
	return redisClient.Set(ctx, cacheKey, dataBytes, ttl).Err()
}

func getWeatherFromAPI(city string) (*WeatherData, error) {
	apiKey := getEnv("OPENWEATHER_API_KEY", "")
	if apiKey == "" {
		// Return mock data if no API key is configured
		log.Println("Warning: OPENWEATHER_API_KEY not configured, returning mock data")
		return getMockWeatherData(city), nil
	}

	url := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", city, apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	var owmResp OpenWeatherResponse
	if err := json.NewDecoder(resp.Body).Decode(&owmResp); err != nil {
		return nil, err
	}

	description := "Clear"
	if len(owmResp.Weather) > 0 {
		description = owmResp.Weather[0].Description
	}

	return &WeatherData{
		City:        owmResp.Name,
		Country:     owmResp.Sys.Country,
		Temperature: owmResp.Main.Temp,
		Description: description,
		Humidity:    owmResp.Main.Humidity,
		WindSpeed:   owmResp.Wind.Speed,
		Timestamp:   time.Now().Unix(),
		Source:      "api",
	}, nil
}

func getMockWeatherData(city string) *WeatherData {
	// Generate some mock weather data
	temps := map[string]float64{
		"london":   12.5,
		"paris":    15.2,
		"tokyo":    18.7,
		"new york": 8.3,
		"sydney":   22.1,
	}

	descriptions := []string{"Sunny", "Cloudy", "Rainy", "Partly cloudy", "Clear"}

	temp := 20.0
	if t, ok := temps[city]; ok {
		temp = t
	}

	return &WeatherData{
		City:        city,
		Country:     "XX",
		Temperature: temp,
		Description: descriptions[int(time.Now().Unix())%len(descriptions)],
		Humidity:    65,
		WindSpeed:   5.2,
		Timestamp:   time.Now().Unix(),
		Source:      "mock",
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"redis":  "disconnected",
	}

	// Check Redis connection
	if redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if _, err := redisClient.Ping(ctx).Result(); err == nil {
			health["redis"] = "connected"
		}
	}

	writeJSONResponse(w, health)
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
