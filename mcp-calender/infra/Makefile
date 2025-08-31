# MCP Productivity Hub - Infrastructure Makefile

# Configuration
REGISTRY = localhost:5000
PROJECT_NAME = mcp-productivity-hub
SERVICES = mcp-server task-service calendar-service weather-service
VERSION ?= latest

# Colors for output
RED = \033[0;31m
GREEN = \033[0;32m
YELLOW = \033[0;33m
BLUE = \033[0;34m
NC = \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "$(BLUE)MCP Productivity Hub - Infrastructure Commands$(NC)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

# Cluster Management
.PHONY: cluster-start
cluster-start: ## Start kind cluster with local registry
	@echo "$(YELLOW)Starting kind cluster with local registry...$(NC)"
	- kind delete cluster --name mcp-cluster >/dev/null 2>&1 || true
	- docker rm -f registry >/dev/null 2>&1 || true
	- docker network rm kind >/dev/null 2>&1 || true
	@kind create cluster --config kind-cluster.yaml --name mcp-cluster
	@if ! docker ps | grep -q registry; then \
		docker run -d --restart=always -p 5000:5000 --name registry registry:2; \
		docker network connect kind registry; \
		echo "$(GREEN)Local registry started and connected to kind$(NC)"; \
	else \
		echo "$(GREEN)Registry already running$(NC)"; \
	fi
	@kubectl config use-context kind-mcp-cluster
	@echo "$(GREEN)Cluster is ready!$(NC)"

.PHONY: cluster-stop
cluster-stop: ## Stop kind cluster and registry
	@echo "$(YELLOW)Stopping kind cluster...$(NC)"
	- kind delete cluster --name mcp-cluster >/dev/null 2>&1 || echo "$(YELLOW)Cluster not found$(NC)"
	- docker stop registry >/dev/null 2>&1 || echo "$(YELLOW)Registry not running$(NC)"
	- docker rm registry >/dev/null 2>&1 || echo "$(YELLOW)Registry not found$(NC)"
	@echo "$(GREEN)Cluster stopped$(NC)"

.PHONY: cluster-info
cluster-info: ## Show cluster information
	@echo "$(BLUE)Cluster Information:$(NC)"
	@kubectl cluster-info || echo "Cluster not available"
	@echo ""
	@echo "$(BLUE)Nodes:$(NC)"
	@kubectl get nodes || echo "No nodes found"
	@echo ""
	@echo "$(BLUE)Registry Status:$(NC)"
	@curl -s http://localhost:5000/v2/_catalog | jq . || echo "Registry not accessible"
