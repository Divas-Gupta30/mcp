#!/bin/bash
# MCP Productivity Hub - Environment Configuration
# Source this file to set up your local development environment
#
# Usage:
#   cp config/environment.sample.sh config/environment.sh
#   # Edit config/environment.sh with your values
#   source config/environment.sh

#=================================================================
# Global Configuration
#=================================================================
export PROJECT_NAME="mcp-productivity-hub"
export REGISTRY="localhost:5000"
export VERSION="latest"

#=================================================================
# Service Ports (for local development)
#=================================================================
export MCP_SERVER_PORT=8080
export TASK_SERVICE_PORT=8081
export CALENDAR_SERVICE_PORT=8082
export WEATHER_SERVICE_PORT=8083

#=================================================================
# Database Configuration
#=================================================================
# PostgreSQL for Task Service
export POSTGRES_HOST="localhost"
export POSTGRES_PORT="5432"
export POSTGRES_DB="taskdb"
export POSTGRES_USER="taskuser"
export POSTGRES_PASSWORD="taskpass"
export DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable"

#=================================================================
# Redis Configuration
#=================================================================
# Redis for Weather Service caching
export REDIS_HOST="localhost"
export REDIS_PORT="6379"
export REDIS_PASSWORD=""
export REDIS_DB="0"
export REDIS_URL="${REDIS_HOST}:${REDIS_PORT}"

#=================================================================
# External API Credentials
#=================================================================

# Google Calendar API (OAuth2)
# Get these from: https://console.cloud.google.com/
# 1. Create a new project or select existing
# 2. Enable Google Calendar API
# 3. Create OAuth2 credentials (Web application)
# 4. Add http://localhost:8082/callback to authorized redirect URIs
export GOOGLE_CLIENT_ID="your-google-client-id-here"
export GOOGLE_CLIENT_SECRET="your-google-client-secret-here"
export GOOGLE_REDIRECT_URL="http://localhost:${CALENDAR_SERVICE_PORT}/callback"

# OpenWeatherMap API
# Get your API key from: https://openweathermap.org/api
# 1. Sign up for a free account
# 2. Generate an API key
# 3. Replace the value below
export OPENWEATHER_API_KEY="your-openweathermap-api-key-here"

#=================================================================
# Service URLs (for MCP Server)
#=================================================================
export TASK_SERVICE_URL="http://localhost:${TASK_SERVICE_PORT}"
export CALENDAR_SERVICE_URL="http://localhost:${CALENDAR_SERVICE_PORT}"
export WEATHER_SERVICE_URL="http://localhost:${WEATHER_SERVICE_PORT}"

#=================================================================
# Development Configuration
#=================================================================
# Set to 'development' for local dev, 'production' for deployment
export ENVIRONMENT="development"

# Log level (debug, info, warn, error)
export LOG_LEVEL="info"

# Enable debug mode
export DEBUG="false"

#=================================================================
# Helper Functions
#=================================================================

# Function to check if all required credentials are set
check_credentials() {
    echo "🔍 Checking API credentials..."

    if [[ "$GOOGLE_CLIENT_ID" == "your-google-client-id-here" ]]; then
        echo "⚠️  Google Client ID not configured"
    else
        echo "✅ Google Client ID configured"
    fi

    if [[ "$GOOGLE_CLIENT_SECRET" == "your-google-client-secret-here" ]]; then
        echo "⚠️  Google Client Secret not configured"
    else
        echo "✅ Google Client Secret configured"
    fi

    if [[ "$OPENWEATHER_API_KEY" == "your-openweathermap-api-key-here" ]]; then
        echo "⚠️  OpenWeatherMap API Key not configured"
    else
        echo "✅ OpenWeatherMap API Key configured"
    fi

    echo ""
    echo "📝 Note: Services will use mock data when credentials are not configured"
}

# Function to start local dependencies
start_dependencies() {
    echo "🚀 Starting local dependencies..."

    # Start PostgreSQL
    if ! docker ps | grep -q postgres; then
        echo "Starting PostgreSQL..."
        docker run -d \
            --name postgres \
            -e POSTGRES_DB="$POSTGRES_DB" \
            -e POSTGRES_USER="$POSTGRES_USER" \
            -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
            -p "$POSTGRES_PORT:5432" \
            postgres:15-alpine
    else
        echo "✅ PostgreSQL already running"
    fi

    # Start Redis
    if ! docker ps | grep -q redis; then
        echo "Starting Redis..."
        docker run -d \
            --name redis \
            -p "$REDIS_PORT:6379" \
            redis:7-alpine
    else
        echo "✅ Redis already running"
    fi

    echo "⏳ Waiting for services to be ready..."
    sleep 5
    echo "✅ Dependencies started"
}

# Function to stop local dependencies
stop_dependencies() {
    echo "🛑 Stopping local dependencies..."
    docker stop postgres redis 2>/dev/null || true
    docker rm postgres redis 2>/dev/null || true
    echo "✅ Dependencies stopped"
}

# Print configuration summary
echo "🔧 MCP Productivity Hub Environment Loaded"
echo "   Project: $PROJECT_NAME"
echo "   Environment: $ENVIRONMENT"
echo "   Registry: $REGISTRY"
echo ""
echo "📡 Service Ports:"
echo "   MCP Server: $MCP_SERVER_PORT"
echo "   Task Service: $TASK_SERVICE_PORT"
echo "   Calendar Service: $CALENDAR_SERVICE_PORT"
echo "   Weather Service: $WEATHER_SERVICE_PORT"
echo ""
echo "🔗 Useful commands:"
echo "   check_credentials    # Check API credentials"
echo "   start_dependencies   # Start PostgreSQL and Redis"
echo "   stop_dependencies    # Stop local dependencies"