#!/bin/bash

# Distributed Cache System Management Script
# This script provides easy commands for managing the distributed cache system

# Ensure script can be executed with bash
if [ ! -n "$BASH" ]; then
    echo "Please run this script with bash"
    exit 1
fi

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Constants
DOCKER_COMPOSE="docker-compose"
SYSTEM_PORT=8081
PROMETHEUS_PORT=9090
GRAFANA_PORT=4000
AUX_BASE_PORT=3000

# Help message
function show_help {
    echo -e "${BLUE}Distributed Cache System Management Script${NC}"
    echo
    echo "Usage: $0 [command]"
    echo
    echo "Commands:"
    echo "  start             Start the system"
    echo "  stop              Stop the system"
    echo "  restart           Restart the system"
    echo "  status            Check the status of all services"
    echo "  logs [service]    View logs (optionally for a specific service)"
    echo "  health            Check system health"
    echo "  load-test         Run load tests"
    echo "  metrics           View system metrics"
    echo "  backup            Trigger a manual backup"
    echo "  restore [id]      Restore from a backup (requires backup ID)"
    echo "  scale-aux [n]     Scale auxiliary nodes to n instances"
    echo "  help              Show this help message"
    echo
    echo "Examples:"
    echo "  $0 start          # Start the system"
    echo "  $0 logs master-1  # View logs for master-1"
    echo "  $0 scale-aux 5    # Scale to 5 auxiliary nodes"
    echo
}

# Check if Docker and Docker Compose are installed
function check_requirements {
    if ! command -v docker &> /dev/null; then
        echo -e "${RED}Docker is required but not found.${NC}"
        echo "Please install Docker first: https://docs.docker.com/get-docker/"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        echo -e "${RED}Docker Compose is required but not found.${NC}"
        echo "Please install Docker Compose first: https://docs.docker.com/compose/install/"
        exit 1
    fi
}

# Start the system
function start_system {
    echo -e "${BLUE}Starting the Distributed Cache System...${NC}"
    $DOCKER_COMPOSE up -d
    
    echo -e "${GREEN}System started successfully!${NC}"
    echo -e "Access the system at: ${YELLOW}http://localhost:${SYSTEM_PORT}${NC}"
    echo -e "Prometheus metrics at: ${YELLOW}http://localhost:${PROMETHEUS_PORT}${NC}"
    echo -e "Grafana dashboards at: ${YELLOW}http://localhost:${GRAFANA_PORT}${NC} (login: admin/admin)"
}

# Stop the system
function stop_system {
    echo -e "${BLUE}Stopping the Distributed Cache System...${NC}"
    $DOCKER_COMPOSE down
    echo -e "${GREEN}System stopped successfully!${NC}"
}

# Restart the system
function restart_system {
    echo -e "${BLUE}Restarting the Distributed Cache System...${NC}"
    $DOCKER_COMPOSE restart
    echo -e "${GREEN}System restarted successfully!${NC}"
}

# Check system status
function check_status {
    echo -e "${BLUE}Checking status of all services...${NC}"
    $DOCKER_COMPOSE ps
}

# View logs
function view_logs {
    if [ -z "$1" ]; then
        echo -e "${BLUE}Viewing logs for all services...${NC}"
        $DOCKER_COMPOSE logs
    else
        echo -e "${BLUE}Viewing logs for $1...${NC}"
        $DOCKER_COMPOSE logs "$1"
    fi
}

# Check system health
function check_health {
    echo -e "${BLUE}Checking system health...${NC}"
    
    # Check if the system is running
    local count=$($DOCKER_COMPOSE ps --services --filter "status=running" | wc -l)
    if [ "$count" -eq 0 ]; then
        echo -e "${RED}The system is not running. Start it first with: $0 start${NC}"
        return 1
    fi
    
    # Check master health
    echo -e "\n${YELLOW}Checking master servers health:${NC}"
    for i in {1..3}; do
        local status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${SYSTEM_PORT}/health)
        if [ "$status" -eq 200 ]; then
            echo -e "Master-$i: ${GREEN}HEALTHY${NC}"
        else
            echo -e "Master-$i: ${RED}UNHEALTHY${NC} (Status code: $status)"
        fi
    done
    
    # Check auxiliary nodes health
    echo -e "\n${YELLOW}Checking auxiliary nodes health:${NC}"
    local aux_count=$($DOCKER_COMPOSE ps --services | grep -c "aux")
    
    for i in $(seq 1 $aux_count); do
        local container="aux$i"
        local status=$($DOCKER_COMPOSE ps --services --filter "status=running" | grep -c "$container")
        
        if [ "$status" -eq 1 ]; then
            echo -e "Auxiliary-$i: ${GREEN}RUNNING${NC}"
        else
            echo -e "Auxiliary-$i: ${RED}NOT RUNNING${NC}"
        fi
    done
    
    # Check Prometheus
    echo -e "\n${YELLOW}Checking monitoring systems:${NC}"
    local prom_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${PROMETHEUS_PORT}/-/healthy)
    if [ "$prom_status" -eq 200 ]; then
        echo -e "Prometheus: ${GREEN}HEALTHY${NC}"
    else
        echo -e "Prometheus: ${RED}UNHEALTHY${NC} (Status code: $prom_status)"
    fi
    
    # Check Grafana (just check if it's responding)
    local grafana_status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${GRAFANA_PORT}/api/health)
    if [ "$grafana_status" -eq 200 ]; then
        echo -e "Grafana: ${GREEN}HEALTHY${NC}"
    else
        echo -e "Grafana: ${RED}UNHEALTHY${NC} (Status code: $grafana_status)"
    fi
}

# Run load tests
function run_load_tests {
    echo -e "${BLUE}Running load tests...${NC}"
    
    # Check if the system is running
    local count=$($DOCKER_COMPOSE ps --services --filter "status=running" | wc -l)
    if [ "$count" -eq 0 ]; then
        echo -e "${RED}The system is not running. Start it first with: $0 start${NC}"
        return 1
    fi
    
    # Run load tests
    if [ -f "./load_test/run_test.sh" ]; then
        cd load_test && ./run_test.sh
    else
        echo -e "${RED}Load test script not found.${NC}"
        echo "Please ensure the load_test directory contains run_test.sh"
        return 1
    fi
}

# View system metrics
function view_metrics {
    echo -e "${BLUE}Opening system metrics...${NC}"
    
    # Check if the system is running
    local count=$($DOCKER_COMPOSE ps --services --filter "status=running" | wc -l)
    if [ "$count" -eq 0 ]; then
        echo -e "${RED}The system is not running. Start it first with: $0 start${NC}"
        return 1
    fi
    
    echo -e "${YELLOW}Prometheus URL: http://localhost:${PROMETHEUS_PORT}${NC}"
    echo -e "${YELLOW}Grafana URL: http://localhost:${GRAFANA_PORT} (login: admin/admin)${NC}"
    
    # Try to open metrics in a browser
    if command -v xdg-open &> /dev/null; then
        xdg-open "http://localhost:${GRAFANA_PORT}" &> /dev/null &
    elif command -v open &> /dev/null; then
        open "http://localhost:${GRAFANA_PORT}" &> /dev/null &
    else
        echo -e "${YELLOW}Please open the metrics manually in your browser.${NC}"
    fi
}

# Trigger a manual backup
function trigger_backup {
    echo -e "${BLUE}Triggering manual backup...${NC}"
    
    # Check if the system is running
    local count=$($DOCKER_COMPOSE ps --services --filter "status=running" | wc -l)
    if [ "$count" -eq 0 ]; then
        echo -e "${RED}The system is not running. Start it first with: $0 start${NC}"
        return 1
    fi
    
    # Trigger backup on all auxiliary nodes
    local backup_id="manual-$(date +%Y%m%d-%H%M%S)"
    local aux_count=$($DOCKER_COMPOSE ps --services | grep -c "aux")
    
    for i in $(seq 1 $aux_count); do
        echo -e "${YELLOW}Backing up auxiliary node $i...${NC}"
        curl -s -X POST -H "Content-Type: application/json" -d "{\"backupID\":\"${backup_id}\"}" http://localhost:${SYSTEM_PORT}/backup
    done
    
    echo -e "${GREEN}Backup triggered with ID: ${backup_id}${NC}"
}

# Restore from a backup
function restore_from_backup {
    # Check if backup ID is provided
    if [ -z "$1" ]; then
        echo -e "${RED}Backup ID is required.${NC}"
        echo "Usage: $0 restore <backup-id>"
        return 1
    fi
    
    local backup_id="$1"
    echo -e "${BLUE}Restoring from backup: ${backup_id}...${NC}"
    
    # Check if the system is running
    local count=$($DOCKER_COMPOSE ps --services --filter "status=running" | wc -l)
    if [ "$count" -eq 0 ]; then
        echo -e "${RED}The system is not running. Start it first with: $0 start${NC}"
        return 1
    fi
    
    # Trigger restore on all auxiliary nodes
    curl -s -X POST -H "Content-Type: application/json" -d "{\"backupID\":\"${backup_id}\"}" http://localhost:${SYSTEM_PORT}/restore
    
    echo -e "${GREEN}Restore from backup ${backup_id} initiated.${NC}"
}

# Scale auxiliary nodes
function scale_auxiliary {
    # Check if number of instances is provided
    if [ -z "$1" ] || ! [[ "$1" =~ ^[0-9]+$ ]]; then
        echo -e "${RED}Number of instances is required.${NC}"
        echo "Usage: $0 scale-aux <number-of-instances>"
        return 1
    fi
    
    local instances="$1"
    echo -e "${BLUE}Scaling to ${instances} auxiliary nodes...${NC}"
    
    # Scale using docker-compose
    $DOCKER_COMPOSE up -d --scale aux=$instances
    
    echo -e "${GREEN}System scaled to ${instances} auxiliary nodes.${NC}"
    echo -e "${YELLOW}Note: You may need to update Prometheus configuration for monitoring.${NC}"
}

# Main function
function main {
    # Check requirements
    check_requirements
    
    # Parse arguments
    case "$1" in
        start)
            start_system
            ;;
        stop)
            stop_system
            ;;
        restart)
            restart_system
            ;;
        status)
            check_status
            ;;
        logs)
            view_logs "$2"
            ;;
        health)
            check_health
            ;;
        load-test)
            run_load_tests
            ;;
        metrics)
            view_metrics
            ;;
        backup)
            trigger_backup
            ;;
        restore)
            restore_from_backup "$2"
            ;;
        scale-aux)
            scale_auxiliary "$2"
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            echo -e "${RED}Unknown command: $1${NC}"
            show_help
            exit 1
            ;;
    esac
}

# Execute main with all arguments
main "$@"

