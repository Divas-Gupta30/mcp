# MCP Productivity Hub

A local-first Golang project that implements an MCP (Model Context Protocol) server connecting to productivity microservices. This system provides AI models (Claude, GPT) with tools to manage tasks, calendar events, and weather information through a unified MCP interface.

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   AI Models     │    │   MCP Server    │    │  Microservices  │
│                 │    │                 │    │                 │
│ • Claude        │◄──►│ • Tool Routing  │◄──►│ • Task Service  │
│ • GPT           │    │ • MCP Protocol  │    │ • Calendar Svc  │
│ • Other LLMs    │    │ • HTTP Proxy    │    │ • Weather Svc   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Components

- **MCP Server**: Central hub implementing MCP protocol for AI model connectivity
- **Task Service**: Task management with PostgreSQL persistence  
- **Calendar Service**: Google Calendar integration with OAuth2
- **Weather Service**: Weather data with Redis caching
- **Infrastructure**: PostgreSQL database, Redis cache, Kubernetes deployment

## 🚀 Services Overview

### MCP Server (Port 8080)
- **Protocol**: Implements Model Context Protocol (MCP) for AI model integration
- **Tools Exposed**: 
  - `get_tasks` - Retrieve all tasks
  - `add_task` - Create new tasks
  - `get_calendar_events` - Fetch calendar events
  - `get_weather` - Get weather for a city
- **Routing**: Proxies requests to appropriate microservices
- **Monitoring**: Prometheus metrics for request counts, duration, errors

### Task Service (Port 8081)
- **Database**: PostgreSQL for persistent task storage
- **REST APIs**: 
  - `GET /tasks` - List all tasks
  - `POST /tasks` - Create new task
  - `PATCH /tasks/:id` - Update existing task
  - `DELETE /tasks/:id` - Delete task
- **Features**: Task priorities, status tracking, timestamps
- **Health Check**: `/health` endpoint with database connectivity check

### Calendar Service (Port 8082)
- **Integration**: Google Calendar API with OAuth2 authentication
- **REST APIs**:
  - `GET /events` - List calendar events (with date filtering)
  - `POST /events` - Create calendar events
  - `GET /auth` - OAuth2 authorization URL
  - `GET /callback` - OAuth2 callback handler
- **Features**: Date range filtering, mock data fallback
- **Authentication**: Secure credential management via Kubernetes secrets

### Weather Service (Port 8083)
- **External API**: OpenWeatherMap integration
- **Caching**: Redis with 10-minute TTL for performance
- **REST API**: `GET /weather?city=CityName`
- **Features**: Automatic cache management, mock data fallback
- **Resilience**: Graceful fallback when Redis unavailable

## 🛠️ Quick Start

### Prerequisites

```bash
# Required tools
go version    # Go 1.21+
docker --version
kubectl version --client
kind version  # or minikube version

# Optional but recommended
helm version
jq --version  # For JSON processing
```

### One-Command Setup

```bash
# Navigate to infrastructure directory
cd infra

# Complete setup (cluster + build + deploy)
make dev-setup
```

This command will:
1. Create kind cluster with local registry
2. Build all Docker images 
3. Push images to local registry
4. Deploy all services to Kubernetes
5. Set up port forwarding

### Manual Setup

1. **Create Kubernetes cluster**:
   ```bash
   cd infra
   make cluster-start
   ```

2. **Build and push images**:
   ```bash
   make build
   make push
   ```

3. **Deploy to Kubernetes**:
   ```bash
   make deploy
   ```

4. **Access services** (in separate terminal):
   ```bash
   make port-forward
   ```

### Verification

Check that all services are running:
```bash
make status
kubectl get pods
make health-check
```

Access endpoints:
- **MCP Server**: http://localhost:8080/tools/list
- **Task Service**: http://localhost:8081/health  
- **Calendar Service**: http://localhost:8082/health
- **Weather Service**: http://localhost:8083/health

## 🔧 Configuration

### API Credentials

Update Google Calendar and OpenWeatherMap credentials:

```bash
# Interactive credential update
make update-secrets

# Or manually edit secrets
kubectl edit secret google-credentials
kubectl edit secret weather-credentials
```

Required credentials:
- **Google OAuth2**: Client ID and Client Secret from Google Cloud Console
- **OpenWeatherMap**: API key from openweathermap.org

### Environment Variables

Each service can be configured via environment variables:

**MCP Server**:
- `PORT`: Server port (default: 8080)
- `TASK_SERVICE_URL`: Task service endpoint
- `CALENDAR_SERVICE_URL`: Calendar service endpoint  
- `WEATHER_SERVICE_URL`: Weather service endpoint

**Task Service**:
- `PORT`: Server port (default: 8081)
- `DATABASE_URL`: PostgreSQL connection string

**Calendar Service**:
- `PORT`: Server port (default: 8082)
- `GOOGLE_CLIENT_ID`: OAuth2 client ID
- `GOOGLE_CLIENT_SECRET`: OAuth2 client secret
- `GOOGLE_REDIRECT_URL`: OAuth2 redirect URL

**Weather Service**:
- `PORT`: Server port (default: 8083)
- `REDIS_URL`: Redis connection string
- `OPENWEATHER_API_KEY`: OpenWeatherMap API key

## 📊 Monitoring & Observability

### Prometheus Metrics

All services expose metrics on `/metrics`:

```bash
# View metrics endpoints
make metrics

# Access metrics directly
curl http://localhost:8080/metrics  # MCP Server
curl http://localhost:8081/metrics  # Task Service  
curl http://localhost:8082/metrics  # Calendar Service
curl http://localhost:8083/metrics  # Weather Service
```

**Key Metrics**:
- Request counts by method/endpoint/status
- Request duration histograms
- Cache hit/miss ratios (Weather Service)
- Database connection health (Task Service)
- External API call counts

### Health Checks

```bash
# Check all service health
make health-check

# Individual health checks
curl http://localhost:8080/health
curl http://localhost:8081/health
curl http://localhost:8082/health  
curl http://localhost:8083/health
```

### Logs

```bash
# View recent logs from all services
make logs

# Follow logs in real-time
make logs-follow

# Service-specific logs
kubectl logs -l app=mcp-server -f
kubectl logs -l app=task-service -f
```

## 🎯 MCP Integration

### Using with Claude Desktop

1. **Configure Claude Desktop** by adding to `claude_desktop_config.json`:
   ```json
   {
     "mcpServers": {
       "productivity-hub": {
         "command": "curl",
         "args": ["-X", "POST", "http://localhost:8080/mcp"]
       }
     }
   }
   ```

2. **Test MCP Tools**:
   ```bash
   # List available tools
   curl -X POST http://localhost:8080/mcp \
     -H "Content-Type: application/json" \
     -d '{"id":"1","method":"tools/list"}'

   # Call a tool
   curl -X POST http://localhost:8080/mcp \
     -H "Content-Type: application/json" \
     -d '{
       "id":"2",
       "method":"tools/call",
       "params":{
         "name":"get_weather",
         "arguments":{"city":"London"}
       }
     }'
   ```

### Available MCP Tools

1. **get_tasks**: Retrieve all tasks
   ```json
   {"name": "get_tasks", "arguments": {}}
   ```

2. **add_task**: Create a new task
   ```json
   {
     "name": "add_task",
     "arguments": {
       "title": "Complete documentation",
       "description": "Finish writing the README",
       "priority": "high"
     }
   }
   ```

3. **get_calendar_events**: Fetch calendar events
   ```json
   {
     "name": "get_calendar_events", 
     "arguments": {
       "start_date": "2024-01-01",
       "end_date": "2024-01-31"
     }
   }
   ```

4. **get_weather**: Get weather information
   ```json
   {
     "name": "get_weather",
     "arguments": {"city": "San Francisco"}
   }
   ```

## 🚢 Deployment Options

### Option 1: Raw Kubernetes Manifests

```bash
# Deploy infrastructure
kubectl apply -f deployments/base/postgres.yaml
kubectl apply -f deployments/base/redis.yaml

# Deploy services  
kubectl apply -f deployments/base/task-service.yaml
kubectl apply -f deployments/base/calendar-service.yaml
kubectl apply -f deployments/base/weather-service.yaml
kubectl apply -f deployments/base/mcp-server.yaml
```

### Option 2: Helm Chart

```bash
# Add dependencies
cd deployments/helm/mcp-productivity-hub
helm dependency update

# Install with custom values
helm install mcp-hub . -f values.yaml

# Upgrade
helm upgrade mcp-hub . -f values.yaml

# Uninstall
helm uninstall mcp-hub
```

### Option 3: Make Commands (Recommended)

```bash
cd infra

# Complete development setup
make dev-setup

# Rebuild and redeploy
make dev-rebuild

# Reset everything
make reset
```

## 🛠️ Development

### Local Development

1. **Run services locally**:
   ```bash
   # Terminal 1: Start PostgreSQL and Redis
   docker-compose up postgres redis

   # Terminal 2: Run Task Service
   cd services/task-service
   go run main.go

   # Terminal 3: Run Calendar Service  
   cd services/calendar-service
   export GOOGLE_CLIENT_ID="your-client-id"
   export GOOGLE_CLIENT_SECRET="your-client-secret"
   go run main.go

   # Terminal 4: Run Weather Service
   cd services/weather-service
   export OPENWEATHER_API_KEY="your-api-key"
   go run main.go

   # Terminal 5: Run MCP Server
   cd services/mcp-server
   go run main.go
   ```

2. **Testing individual services**:
   ```bash
   # Test Task Service
   curl -X POST http://localhost:8081/tasks \
     -H "Content-Type: application/json" \
     -d '{"title":"Test task","priority":"medium"}'

   # Test Calendar Service
   curl http://localhost:8082/events

   # Test Weather Service  
   curl http://localhost:8083/weather?city=London
   ```

### Code Structure

```
mcp-productivity-hub/
├── services/
│   ├── mcp-server/           # MCP protocol implementation
│   │   ├── main.go          # HTTP server & tool routing
│   │   ├── go.mod           # Go dependencies
│   │   └── Dockerfile       # Container image
│   ├── task-service/        # Task management service
│   │   ├── main.go          # REST API & PostgreSQL
│   │   ├── go.mod
│   │   └── Dockerfile
│   ├── calendar-service/    # Google Calendar integration
│   │   ├── main.go          # OAuth2 & Calendar API
│   │   ├── go.mod
│   │   └── Dockerfile
│   └── weather-service/     # Weather data service
│       ├── main.go          # OpenWeatherMap & Redis
│       ├── go.mod
│       └── Dockerfile
├── deployments/
│   ├── base/                # Raw Kubernetes manifests
│   │   ├── postgres.yaml
│   │   ├── redis.yaml
│   │   ├── task-service.yaml
│   │   ├── calendar-service.yaml
│   │   ├── weather-service.yaml
│   │   └── mcp-server.yaml
│   └── helm/                # Helm chart
│       └── mcp-productivity-hub/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── infra/
│   ├── Makefile            # Automation commands
│   └── kind-cluster.yaml   # Kubernetes cluster config
└── README.md
```

## 🔍 Troubleshooting

### Common Issues

1. **Cluster Won't Start**:
   ```bash
   # Check Docker is running
   docker ps
   
   # Clean up and retry
   make reset
   make cluster-start
   ```

2. **Images Won't Build**:
   ```bash
   # Check Go modules
   cd services/mcp-server
   go mod tidy
   
   # Build individually
   docker build -t localhost:5000/mcp-server:latest .
   ```

3. **Services Won't Connect**:
   ```bash
   # Check service discovery
   kubectl get services
   kubectl get endpoints
   
   # Check logs for connection errors
   make logs
   ```

4. **Database Connection Fails**:
   ```bash
   # Check PostgreSQL pod
   kubectl get pods -l app=postgres
   kubectl logs -l app=postgres
   
   # Test connection
   kubectl exec -it deployment/postgres -- psql -U taskuser -d taskdb
   ```

5. **Redis Connection Fails**:
   ```bash
   # Check Redis pod  
   kubectl get pods -l app=redis
   kubectl logs -l app=redis
   
   # Test connection
   kubectl exec -it deployment/redis -- redis-cli ping
   ```

### Debug Commands

```bash
# Get detailed status
kubectl describe pod <pod-name>

# Shell into a service
make shell SERVICE=mcp-server

# Port forward for debugging
kubectl port-forward pod/<pod-name> 8080:8080

# Check resource usage
kubectl top pods
kubectl top nodes
```

## 🧹 Cleanup

```bash
# Remove all deployments
make undeploy

# Clean up images  
make clean

# Complete reset (cluster + images)
make reset
```

## 📚 API Documentation

### Task Service API

**GET /tasks**
- Returns list of all tasks
- Response: `{"tasks": [...]}`

**POST /tasks**  
- Creates new task
- Body: `{"title": "string", "description": "string", "priority": "low|medium|high"}`
- Response: Created task object

**PATCH /tasks/:id**
- Updates existing task
- Body: Partial task object
- Response: Updated task object

**DELETE /tasks/:id**
- Deletes task
- Response: 204 No Content

### Calendar Service API

**GET /events**
- Query params: `start_date`, `end_date` (YYYY-MM-DD)
- Returns calendar events

**POST /events**
- Body: `{"summary": "string", "start": "RFC3339", "end": "RFC3339", "location": "string"}`
- Creates calendar event

**GET /auth**
- Returns Google OAuth2 authorization URL

### Weather Service API  

**GET /weather**
- Query param: `city` (required)
- Returns weather data with caching

## 📝 License

This project is provided as-is for educational and development purposes.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test thoroughly  
4. Submit a pull request

## 📧 Support

For issues and questions:
- Check the troubleshooting section
- Review logs with `make logs`
- Create an issue in the repository
