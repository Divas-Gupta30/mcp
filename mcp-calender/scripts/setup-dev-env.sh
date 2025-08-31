#!/bin/bash
# MCP Productivity Hub - Development Environment Setup Script
# This script helps you set up your local development environment

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="$PROJECT_ROOT/config"

echo -e "${BLUE}🚀 MCP Productivity Hub - Development Environment Setup${NC}"
echo "Project root: $PROJECT_ROOT"
echo ""

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo -e "${YELLOW}📋 Checking prerequisites...${NC}"

# Check Go
if command_exists go; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "✅ Go: $GO_VERSION"
else
    echo -e "${RED}❌ Go is not installed. Please install Go 1.21+ from https://golang.org/${NC}"
    exit 1
fi

# Check Docker
if command_exists docker; then
    DOCKER_VERSION=$(docker --version | awk '{print $3}' | sed 's/,//')
    echo -e "✅ Docker: $DOCKER_VERSION"
else
    echo -e "${RED}❌ Docker is not installed. Please install Docker from https://docker.com/${NC}"
    exit 1
fi

# Check kubectl (optional)
if command_exists kubectl; then
    KUBECTL_VERSION=$(kubectl version --client --short 2>/dev/null | awk '{print $3}')
    echo -e "✅ kubectl: $KUBECTL_VERSION"
else
    echo -e "${YELLOW}⚠️  kubectl not found (optional for local development)${NC}"
fi

# Check kind (optional)
if command_exists kind; then
    KIND_VERSION=$(kind version | awk '{print $2}')
    echo -e "✅ kind: $KIND_VERSION"
else
    echo -e "${YELLOW}⚠️  kind not found (optional for local development)${NC}"
fi

echo ""

# Setup configuration files
echo -e "${YELLOW}⚙️  Setting up configuration files...${NC}"

# Copy environment sample files if they don't exist
copy_config_file() {
    local sample_file="$1"
    local target_file="${sample_file%.sample}"

    if [[ -f "$target_file" ]]; then
        echo -e "✅ $target_file already exists"
    else
        cp "$sample_file" "$target_file"
        echo -e "📝 Created $target_file from sample"
    fi
}

# Main environment file
if [[ -f "$CONFIG_DIR/environment.sample.sh" ]]; then
    copy_config_file "$CONFIG_DIR/environment.sample.sh"
fi

# Service-specific files
for file in "$CONFIG_DIR"/*.env.sample; do
    if [[ -f "$file" ]]; then
        copy_config_file "$file"
    fi
done

echo ""

# API Credentials setup
echo -e "${YELLOW}🔑 API Credentials Setup${NC}"
echo "You need to configure the following API credentials:"
echo ""

echo -e "${BLUE}1. Google Calendar API:${NC}"
echo "   • Go to: https://console.cloud.google.com/"
echo "   • Create/select a project"
echo "   • Enable Google Calendar API"
echo "   • Create OAuth2 credentials (Web application)"
echo "   • Add http://localhost:8082/callback to authorized redirect URIs"
echo "   • Update GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET in config files"
echo ""

echo -e "${BLUE}2. OpenWeatherMap API:${NC}"
echo "   • Go to: https://openweathermap.org/api"
echo "   • Sign up for a free account"
echo "   • Generate an API key"
echo "   • Update OPENWEATHER_API_KEY in config files"
echo ""

# Prompt for credential setup
read -p "Would you like to configure API credentials now? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo -e "${YELLOW}🔧 Configuring API credentials...${NC}"

    # Google credentials
    echo -e "${BLUE}Google Calendar API:${NC}"
    read -p "Enter Google Client ID: " GOOGLE_CLIENT_ID
    read -p "Enter Google Client Secret: " GOOGLE_CLIENT_SECRET

    # OpenWeatherMap credentials
    echo -e "${BLUE}OpenWeatherMap API:${NC}"
    read -p "Enter OpenWeatherMap API Key: " OPENWEATHER_API_KEY

    # Update the main environment file
    ENV_FILE="$CONFIG_DIR/environment.sh"
    if [[ -f "$ENV_FILE" ]]; then
        sed -i.bak "s/your-google-client-id-here/$GOOGLE_CLIENT_ID/g" "$ENV_FILE"
        sed -i.bak "s/your-google-client-secret-here/$GOOGLE_CLIENT_SECRET/g" "$ENV_FILE"
        sed -i.bak "s/your-openweathermap-api-key-here/$OPENWEATHER_API_KEY/g" "$ENV_FILE"
        rm "$ENV_FILE.bak"
        echo -e "✅ Updated $ENV_FILE with your credentials"
    fi

    # Update service-specific files
    for service in mcp-server task-service calendar-service weather-service; do
        SERVICE_ENV="$CONFIG_DIR/$service.env"
        if [[ -f "$SERVICE_ENV" ]]; then
            case "$service" in
                "calendar-service")
                    sed -i.bak "s/your-google-client-id-here/$GOOGLE_CLIENT_ID/g" "$SERVICE_ENV"
                    sed -i.bak "s/your-google-client-secret-here/$GOOGLE_CLIENT_SECRET/g" "$SERVICE_ENV"
                    ;;
                "weather-service")
                    sed -i.bak "s/your-openweathermap-api-key-here/$OPENWEATHER_API_KEY/g" "$SERVICE_ENV"
                    ;;
            esac
            [[ -f "$SERVICE_ENV.bak" ]] && rm "$SERVICE_ENV.bak"
        fi
    done

    echo -e "✅ API credentials configured in all service files"
fi

echo ""

# Development dependencies
echo -e "${YELLOW}🐳 Setting up development dependencies...${NC}"

read -p "Would you like to start PostgreSQL and Redis containers? (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo ""
    echo "Starting PostgreSQL..."
    if ! docker ps | grep -q postgres; then
        docker run -d \
            --name postgres \
            -e POSTGRES_DB=taskdb \
            -e POSTGRES_USER=taskuser \
            -e POSTGRES_PASSWORD=taskpass \
            -p 5432:5432 \
            postgres:15-alpine
        echo -e "✅ PostgreSQL started on port 5432"
    else
        echo -e "✅ PostgreSQL already running"
    fi

    echo "Starting Redis..."
    if ! docker ps | grep -q redis; then
        docker run -d \
            --name redis \
            -p 6379:6379 \
            redis:7-alpine
        echo -e "✅ Redis started on port 6379"
    else
        echo -e "✅ Redis already running"
    fi

    echo ""
    echo -e "${GREEN}⏳ Waiting for services to be ready...${NC}"
    sleep 5
fi

echo ""

# Next steps
echo -e "${GREEN}🎉 Development environment setup complete!${NC}"
echo ""
echo -e "${BLUE}📝 Next steps:${NC}"
echo ""
echo "1. Source the environment file:"
echo "   source config/environment.sh"
echo ""
echo "2. Run individual services:"
echo "   # Terminal 1: Task Service"
echo "   source config/task-service.env && cd services/task-service && go run main.go"
echo ""
echo "   # Terminal 2: Calendar Service"
echo "   source config/calendar-service.env && cd services/calendar-service && go run main.go"
echo ""
echo "   # Terminal 3: Weather Service"
echo "   source config/weather-service.env && cd services/weather-service && go run main.go"
echo ""
echo "   # Terminal 4: MCP Server"
echo "   source config/mcp-server.env && cd services/mcp-server && go run main.go"
echo ""
echo "3. Or use Kubernetes deployment:"
echo "   cd infra && make dev-setup"
echo ""
echo "4. Test the services:"
echo "   curl http://localhost:8080/tools/list"
echo "   curl http://localhost:8081/health"
echo "   curl http://localhost:8082/health"
echo "   curl http://localhost:8083/health"
echo ""
echo -e "${BLUE}📚 Documentation:${NC}"
echo "   • README.md - Complete project documentation"
echo "   • config/ - Environment configuration files"
echo "   • infra/Makefile - Kubernetes deployment commands"
echo ""
echo -e "${GREEN}🚀 Happy coding!${NC}"