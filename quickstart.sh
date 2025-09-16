#!/bin/bash

# Replicator Local Development Quickstart Script
# This script sets up a complete local development environment for testing replication scenarios

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}" && pwd)"

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    # Check if ports are available
    local ports=(27017 27018 3306 3307 5432 5433 9200 9092 6379 8080 9090 9091 3000)
    for port in "${ports[@]}"; do
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            log_warning "Port $port is already in use. This may cause conflicts."
        fi
    done
    
    log_success "Prerequisites check completed."
}

build_replicator() {
    log_info "Building Replicator application..."
    
    if [ ! -f "$PROJECT_ROOT/Dockerfile" ]; then
        log_error "Dockerfile not found. Please ensure you're running this from the project root."
        exit 1
    fi
    
    docker build -t replicator:latest . || {
        log_error "Failed to build Replicator Docker image."
        exit 1
    }
    
    log_success "Replicator application built successfully."
}

start_infrastructure() {
    log_info "Starting infrastructure services (databases, Kafka, etc.)..."
    
    # Start infrastructure services first (excluding replicator app)
    docker-compose up -d \
        mongodb-source mongodb-target \
        mysql-source mysql-target \
        postgresql-source postgresql-target \
        elasticsearch \
        zookeeper kafka redis \
        prometheus grafana
    
    log_info "Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    check_service_health
    
    log_success "Infrastructure services started successfully."
}

check_service_health() {
    log_info "Checking service health..."
    
    # MongoDB
    until docker exec replicator-mongodb-source mongosh --eval "db.runCommand('ping')" >/dev/null 2>&1; do
        log_info "Waiting for MongoDB source..."
        sleep 2
    done
    
    until docker exec replicator-mongodb-target mongosh --eval "db.runCommand('ping')" >/dev/null 2>&1; do
        log_info "Waiting for MongoDB target..."
        sleep 2
    done
    
    # MySQL
    until docker exec replicator-mysql-source mysql -u root -prootpassword -e "SELECT 1" >/dev/null 2>&1; do
        log_info "Waiting for MySQL source..."
        sleep 2
    done
    
    until docker exec replicator-mysql-target mysql -u root -prootpassword -e "SELECT 1" >/dev/null 2>&1; do
        log_info "Waiting for MySQL target..."
        sleep 2
    done
    
    # PostgreSQL
    until docker exec replicator-postgresql-source pg_isready -U replicator >/dev/null 2>&1; do
        log_info "Waiting for PostgreSQL source..."
        sleep 2
    done
    
    until docker exec replicator-postgresql-target pg_isready -U replicator >/dev/null 2>&1; do
        log_info "Waiting for PostgreSQL target..."
        sleep 2
    done
    
    # Elasticsearch
    until curl -s http://localhost:9200/_cluster/health >/dev/null 2>&1; do
        log_info "Waiting for Elasticsearch..."
        sleep 2
    done
    
    # Kafka
    until docker exec replicator-kafka kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; do
        log_info "Waiting for Kafka..."
        sleep 2
    done
    
    log_success "All services are healthy."
}

run_scenario() {
    local scenario=$1
    local config_file="examples/configs/${scenario}-new.yaml"
    
    # Fallback to original filename if new one doesn't exist
    if [ ! -f "$config_file" ]; then
        config_file="examples/configs/${scenario}.yaml"
    fi
    
    if [ ! -f "$config_file" ]; then
        log_error "Configuration file not found: $config_file"
        log_info "Available configurations:"
        ls -la examples/configs/*.yaml
        return 1
    fi
    
    log_info "Running scenario: $scenario"
    log_info "Configuration: $config_file"
    
    # Copy the config file to the container's config directory
    cp "$config_file" "examples/configs/config.yaml"
    
    # Start replicator with the selected configuration
    docker-compose up -d replicator
    
    log_info "Replicator started with configuration: $scenario"
    log_info "Monitor logs with: docker-compose logs -f replicator"
    log_info "Check metrics at: http://localhost:9090/metrics"
    log_info "Check health at: http://localhost:8080/health"
}

stop_scenario() {
    log_info "Stopping current replication scenario..."
    docker-compose stop replicator
    log_success "Replication scenario stopped."
}

show_services() {
    log_info "Service endpoints:"
    echo ""
    echo "🔧 Application Services:"
    echo "  Replicator API:      http://localhost:8080"
    echo "  Replicator Metrics:  http://localhost:9090/metrics"
    echo "  Replicator Health:   http://localhost:8080/health"
    echo ""
    echo "📊 Monitoring:"
    echo "  Prometheus:          http://localhost:9091"
    echo "  Grafana:             http://localhost:3000 (admin/admin123)"
    echo ""
    echo "💾 Source Databases:"
    echo "  MongoDB:             localhost:27017 (admin/password123)"
    echo "  MySQL:               localhost:3306 (replicator/password123)"
    echo "  PostgreSQL:          localhost:5432 (replicator/password123)"
    echo ""
    echo "🎯 Target Systems:"
    echo "  MongoDB Target:      localhost:27018 (admin/password123)"
    echo "  MySQL Target:        localhost:3307 (replicator/password123)"
    echo "  PostgreSQL Target:   localhost:5433 (replicator/password123)"
    echo "  Elasticsearch:       http://localhost:9200"
    echo ""
    echo "📡 Messaging:"
    echo "  Kafka:               localhost:9092"
    echo "  Redis:               localhost:6379 (password: password123)"
    echo ""
}

show_scenarios() {
    log_info "Available replication scenarios:"
    echo ""
    echo "1. mongodb-to-mongodb     - Basic MongoDB to MongoDB replication"
    echo "2. mysql-to-elasticsearch - MySQL to Elasticsearch indexing"
    echo "3. postgresql-to-kafka    - PostgreSQL change streams to Kafka"
    echo "4. multi-source-aggregation - Multi-source data aggregation"
    echo ""
    echo "Usage: $0 run <scenario-name>"
}

test_scenario() {
    local scenario=$1
    
    case $scenario in
        "mongodb-to-mongodb")
            test_mongodb_scenario
            ;;
        "mysql-to-elasticsearch")
            test_mysql_scenario
            ;;
        "postgresql-to-kafka")
            test_postgresql_scenario
            ;;
        "multi-source-aggregation")
            test_multi_source_scenario
            ;;
        *)
            log_error "Unknown scenario: $scenario"
            return 1
            ;;
    esac
}

test_mongodb_scenario() {
    log_info "Testing MongoDB to MongoDB replication..."
    
    # Insert test data
    docker exec replicator-mongodb-source mongosh --eval "
        db = db.getSiblingDB('source_db');
        db.users.insertOne({
            name: 'Test User $(date +%s)',
            email: 'test$(date +%s)@example.com',
            department: 'Testing',
            created_at: new Date()
        });
    "
    
    sleep 5
    
    # Check if data was replicated
    local count=$(docker exec replicator-mongodb-target mongosh --quiet --eval "
        db = db.getSiblingDB('target_db');
        db.users.countDocuments({department: 'Testing'});
    ")
    
    if [ "$count" -gt 0 ]; then
        log_success "MongoDB replication test passed! Found $count test records."
    else
        log_warning "MongoDB replication test may have issues. Check logs."
    fi
}

test_mysql_scenario() {
    log_info "Testing MySQL to Elasticsearch replication..."
    
    # Insert test data
    docker exec replicator-mysql-source mysql -u replicator -ppassword123 -D source_db -e "
        INSERT INTO products (name, description, price, category_id, brand) 
        VALUES ('Test Product $(date +%s)', 'Test description', 99.99, 999, 'TestBrand');
    "
    
    sleep 10
    
    # Check Elasticsearch
    local response=$(curl -s "http://localhost:9200/products/_search?q=TestBrand" | grep -o '"total":{"value":[0-9]*' | grep -o '[0-9]*$')
    
    if [ "$response" -gt 0 ]; then
        log_success "MySQL to Elasticsearch test passed! Found $response indexed products."
    else
        log_warning "MySQL to Elasticsearch test may have issues. Check logs."
    fi
}

test_postgresql_scenario() {
    log_info "Testing PostgreSQL to Kafka replication..."
    
    # Insert test data
    docker exec replicator-postgresql-source psql -U replicator -d source_db -c "
        INSERT INTO orders (customer_id, total_amount, status, order_date) 
        VALUES (999, 123.45, 'test', CURRENT_DATE);
    "
    
    sleep 10
    
    # Check Kafka topic
    local messages=$(docker exec replicator-kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic orders-stream --timeout-ms 5000 --from-beginning 2>/dev/null | wc -l)
    
    if [ "$messages" -gt 0 ]; then
        log_success "PostgreSQL to Kafka test passed! Found $messages messages."
    else
        log_warning "PostgreSQL to Kafka test may have issues. Check logs."
    fi
}

test_multi_source_scenario() {
    log_info "Testing multi-source aggregation..."
    
    # Insert data in all sources
    docker exec replicator-mongodb-source mongosh --eval "
        db = db.getSiblingDB('source_db');
        db.users.insertOne({name: 'Multi Test $(date +%s)', email: 'multi$(date +%s)@example.com'});
    "
    
    docker exec replicator-mysql-source mysql -u replicator -ppassword123 -D source_db -e "
        INSERT INTO orders (customer_id, total_amount, status, order_date) VALUES (888, 999.99, 'multi-test', CURRENT_DATE);
    "
    
    docker exec replicator-postgresql-source psql -U replicator -d source_db -c "
        INSERT INTO products (name, description, price, category_id) VALUES ('Multi Product', 'Multi test', 77.77, 888);
    "
    
    sleep 15
    
    # Check aggregated data in Elasticsearch
    local user_docs=$(curl -s "http://localhost:9200/customer-analytics/_search?q=document_type:user_profile" | grep -o '"total":{"value":[0-9]*' | grep -o '[0-9]*$')
    local order_docs=$(curl -s "http://localhost:9200/customer-analytics/_search?q=document_type:order_event" | grep -o '"total":{"value":[0-9]*' | grep -o '[0-9]*$')
    local product_docs=$(curl -s "http://localhost:9200/customer-analytics/_search?q=document_type:product_info" | grep -o '"total":{"value":[0-9]*' | grep -o '[0-9]*$')
    
    log_info "Aggregated documents - Users: $user_docs, Orders: $order_docs, Products: $product_docs"
    
    if [ "$user_docs" -gt 0 ] && [ "$order_docs" -gt 0 ] && [ "$product_docs" -gt 0 ]; then
        log_success "Multi-source aggregation test passed!"
    else
        log_warning "Multi-source aggregation test may have issues. Check logs."
    fi
}

cleanup() {
    log_info "Stopping all services..."
    docker-compose down
    log_info "Removing volumes (this will delete all data)..."
    docker-compose down -v
    log_success "Cleanup completed."
}

show_logs() {
    local service=${1:-replicator}
    docker-compose logs -f "$service"
}

# Main command handling
case "${1:-help}" in
    "prereq"|"prerequisites")
        check_prerequisites
        ;;
    "build")
        check_prerequisites
        build_replicator
        ;;
    "start")
        check_prerequisites
        build_replicator
        start_infrastructure
        show_services
        ;;
    "run")
        if [ -z "$2" ]; then
            show_scenarios
            exit 1
        fi
        run_scenario "$2"
        ;;
    "stop")
        stop_scenario
        ;;
    "test")
        if [ -z "$2" ]; then
            log_error "Please specify a scenario to test."
            show_scenarios
            exit 1
        fi
        test_scenario "$2"
        ;;
    "scenarios"|"list")
        show_scenarios
        ;;
    "services"|"endpoints")
        show_services
        ;;
    "logs")
        show_logs "$2"
        ;;
    "cleanup"|"clean")
        cleanup
        ;;
    "help"|*)
        echo "Replicator Local Development Quickstart"
        echo ""
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  prereq              Check prerequisites"
        echo "  build               Build Replicator Docker image"
        echo "  start               Start all infrastructure services"
        echo "  run <scenario>      Run a specific replication scenario"
        echo "  stop                Stop current replication scenario"
        echo "  test <scenario>     Test a specific scenario with sample data"
        echo "  scenarios           List available scenarios"
        echo "  services            Show service endpoints"
        echo "  logs [service]      Show logs (default: replicator)"
        echo "  cleanup             Stop all services and remove volumes"
        echo "  help                Show this help message"
        echo ""
        echo "Example workflow:"
        echo "  $0 prereq          # Check prerequisites"
        echo "  $0 start           # Start infrastructure"
        echo "  $0 run mongodb-to-mongodb  # Run MongoDB scenario"
        echo "  $0 test mongodb-to-mongodb # Test the scenario"
        echo "  $0 logs            # Monitor logs"
        echo "  $0 cleanup         # Clean up when done"
        ;;
esac