#!/bin/bash
# prometheus_queries.sh
# This script demonstrates various ways to query Prometheus metrics for the distributed cache system
# Author: AI Assistant
# Created: 2025-05-11

# Set the Prometheus server URL
PROMETHEUS_URL="http://localhost:9090"

# Color definitions for better output formatting
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper function to send a query to Prometheus and format the output
query_prometheus() {
  local query="$1"
  local title="$2"
  local description="$3"
  
  echo -e "${GREEN}=== $title ===${NC}"
  echo -e "${BLUE}Description:${NC} $description"
  echo -e "${BLUE}Query:${NC} $query"
  echo -e "${YELLOW}Result:${NC}"
  
  # Execute the query and format the output
  curl -s -G --data-urlencode "query=$query" "$PROMETHEUS_URL/api/v1/query" | jq '.'
  
  echo ""
}

# Helper function for range queries
range_query_prometheus() {
  local query="$1"
  local title="$2"
  local description="$3"
  local start=$(date -d '1 hour ago' +%s)
  local end=$(date +%s)
  local step="15s"
  
  echo -e "${GREEN}=== $title ===${NC}"
  echo -e "${BLUE}Description:${NC} $description"
  echo -e "${BLUE}Query:${NC} $query"
  echo -e "${BLUE}Range:${NC} Last hour with 15s steps"
  echo -e "${YELLOW}Result (truncated):${NC}"
  
  # Execute the range query and format the output, showing only part of the result
  curl -s -G \
    --data-urlencode "query=$query" \
    --data-urlencode "start=$start" \
    --data-urlencode "end=$end" \
    --data-urlencode "step=$step" \
    "$PROMETHEUS_URL/api/v1/query_range" | jq '.data.result[] | {metric, values: (.values | .[0:3])}' 
  
  echo ""
}

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}= Prometheus Query Examples         =${NC}"
echo -e "${GREEN}= For Distributed Cache System      =${NC}"
echo -e "${GREEN}=====================================${NC}"
echo ""

#########################################################
# 1. Basic Queries for Current Values
#########################################################

# Current cache size
query_prometheus \
  "cache_size" \
  "Current Cache Size" \
  "Shows the current number of items stored in the cache."

# All cache operations counts
query_prometheus \
  "cache_operations_total" \
  "Cache Operations Counter" \
  "Shows the total count of all cache operations, broken down by operation type and status."

# Memory usage
query_prometheus \
  "cache_memory_bytes" \
  "Current Memory Usage" \
  "Shows the current memory usage of the cache in bytes."

# Server info
query_prometheus \
  "server_info" \
  "Server Information" \
  "Shows metadata about the server, including its type."

#########################################################
# 2. Range Queries for Time-Series Data
#########################################################

# Cache size over time
range_query_prometheus \
  "cache_size" \
  "Cache Size Over Time" \
  "Shows how the cache size has changed over the time period."

# Cache operations over time
range_query_prometheus \
  "sum by(operation)(cache_operations_total)" \
  "Operations Count Over Time" \
  "Shows how the total number of operations has changed, grouped by operation type."

#########################################################
# 3. Rate Calculations for Operations
#########################################################

# Operation rate per second (over 5m window)
query_prometheus \
  "rate(cache_operations_total[5m])" \
  "Operation Rate (per second)" \
  "Shows the rate of operations per second, calculated over a 5-minute window."

# Request duration 95th percentile
query_prometheus \
  "histogram_quantile(0.95, sum(rate(request_duration_seconds_bucket[5m])) by (le, handler))" \
  "95th Percentile Request Duration" \
  "Shows the 95th percentile of request durations, meaning 95% of requests are faster than this value."

#########################################################
# 4. Aggregations for Hit Rates
#########################################################

# Cache hit ratio calculation
query_prometheus \
  "sum(cache_operations_total{operation=\"get\",status=\"hit\"}) / sum(cache_operations_total{operation=\"get\"})" \
  "Cache Hit Ratio" \
  "Shows the overall cache hit ratio (proportion of GET operations that were hits)."

# Cache hit ratio as percentage
query_prometheus \
  "100 * sum(cache_operations_total{operation=\"get\",status=\"hit\"}) / sum(cache_operations_total{operation=\"get\"})" \
  "Cache Hit Percentage" \
  "Shows the overall cache hit ratio as a percentage."

# Operation distribution by type
query_prometheus \
  "sum by(operation) (cache_operations_total) / scalar(sum(cache_operations_total))" \
  "Operation Distribution" \
  "Shows the proportion of each operation type relative to all operations."

#########################################################
# 5. Complex Examples
#########################################################

# Average latency by operation type
query_prometheus \
  "sum by(handler, method) (rate(request_duration_seconds_sum[5m])) / sum by(handler, method) (rate(request_duration_seconds_count[5m]))" \
  "Average Latency by Handler and Method" \
  "Shows the average request latency broken down by endpoint and HTTP method."

# Slow requests percentage
query_prometheus \
  "sum(rate(request_duration_seconds_bucket{le=\"0.1\"}[5m])) by (handler) / sum(rate(request_duration_seconds_count[5m])) by (handler)" \
  "Percentage of Fast Requests (<100ms)" \
  "Shows the percentage of requests completing in less than 100ms, by handler."

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}= HOW TO USE THESE QUERIES          =${NC}"
echo -e "${GREEN}=====================================${NC}"
echo ""
echo -e "1. ${BLUE}Via Prometheus UI:${NC}"
echo "   Open http://localhost:9090 in your browser"
echo "   Enter any of the queries above in the query box"
echo "   Click 'Execute' to run the query"
echo "   Use the Graph tab to visualize time-series data"
echo ""
echo -e "2. ${BLUE}Via Grafana:${NC}"
echo "   Open http://localhost:4000 in your browser (default: admin/admin)"
echo "   Create a new dashboard and add panels"
echo "   Use the queries as the data source for each panel"
echo ""
echo -e "3. ${BLUE}Via API:${NC}"
echo "   Use curl to query the Prometheus API directly:"
echo '   curl -G --data-urlencode "query=cache_size" http://localhost:9090/api/v1/query'
echo ""
echo -e "4. ${BLUE}For Alerts:${NC}"
echo "   Add these queries to prometheus/alert_rules.yml with appropriate thresholds"
echo "   Example: cache_hit_ratio < 0.5 would alert when hit rate drops below 50%"
echo ""

echo "Script execution complete."

