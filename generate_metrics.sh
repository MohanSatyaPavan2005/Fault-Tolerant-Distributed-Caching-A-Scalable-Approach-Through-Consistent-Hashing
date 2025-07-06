#!/bin/bash

# Enhanced script to generate comprehensive test metrics for the distributed cache system
# This will create rich data visualizations in the Grafana dashboard

# Colors for better output readability
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
MASTER_NODES=("master-1" "master-2" "master-3")
MASTER_PORTS=(8000 8000 8000)
AUX_NODES=("aux1" "aux2" "aux3")
AUX_PORTS=(3001 3002 3003)
TOTAL_KEYS=100
ERROR_RATE=10  # 1 in X operations will simulate an error
DELAY_RATE=5   # 1 in X operations will simulate high latency
RUN_TIME=300   # Run time in seconds (5 minutes)

# Function to send PUT request to store a key-value pair
put_cache() {
    node_type=$1  # "master" or "aux"
    node_index=$2 # Index in the array
    key=$3
    value=$4
    size=${#value}
    
    if [ "$node_type" == "master" ]; then
        node=${MASTER_NODES[$node_index]}
        port=${MASTER_PORTS[$node_index]}
    else
        node=${AUX_NODES[$node_index]}
        port=${AUX_PORTS[$node_index]}
    fi
    
    # Simulate occasional high latency
    if [ $((RANDOM % DELAY_RATE)) -eq 0 ]; then
        sleep 0.2
        echo -e "${YELLOW}⏱ Delayed PUT operation for key: $key on $node:$port${NC}"
    fi
    
    # Simulate occasional errors
    if [ $((RANDOM % ERROR_RATE)) -eq 0 ]; then
        echo -e "${RED}✗ Error storing key: $key on $node:$port${NC}"
        return 1
    fi
    
    echo -e "${GREEN}✓ Storing key: $key with value: $value (size: ${size}B) on $node:$port${NC}"
    docker exec distributed-cache-system-$node-1 curl -s -X PUT \
        -H "Content-Type: application/json" \
        -d "{\"key\":\"$key\",\"value\":\"$value\"}" \
        http://localhost:$port/data > /dev/null
    
    return 0
}

# Function to get value from cache
get_cache() {
    node_type=$1  # "master" or "aux"
    node_index=$2 # Index in the array
    key=$3
    
    if [ "$node_type" == "master" ]; then
        node=${MASTER_NODES[$node_index]}
        port=${MASTER_PORTS[$node_index]}
    else
        node=${AUX_NODES[$node_index]}
        port=${AUX_PORTS[$node_index]}
    fi
    
    # Simulate occasional high latency
    if [ $((RANDOM % DELAY_RATE)) -eq 0 ]; then
        sleep 0.2
        echo -e "${YELLOW}⏱ Delayed GET operation for key: $key on $node:$port${NC}"
    fi
    
    # Simulate occasional errors
    if [ $((RANDOM % ERROR_RATE)) -eq 0 ]; then
        echo -e "${RED}✗ Error retrieving key: $key on $node:$port${NC}"
        return 1
    fi
    
    result=$(docker exec distributed-cache-system-$node-1 curl -s http://localhost:$port/data/$key)
    
    if [[ $result == *"\"value\""* ]]; then
        echo -e "${GREEN}✓ Cache HIT for key: $key on $node:$port${NC}"
        return 0
    else
        echo -e "${BLUE}? Cache MISS for key: $key on $node:$port${NC}"
        return 2
    fi
}

# Function to delete a key from cache
delete_cache() {
    node_type=$1  # "master" or "aux"
    node_index=$2 # Index in the array
    key=$3
    
    if [ "$node_type" == "master" ]; then
        node=${MASTER_NODES[$node_index]}
        port=${MASTER_PORTS[$node_index]}
    else
        node=${AUX_NODES[$node_index]}
        port=${AUX_PORTS[$node_index]}
    fi
    
    echo -e "${BLUE}- Deleting key: $key from $node:$port${NC}"
    docker exec distributed-cache-system-$node-1 curl -s -X DELETE \
        http://localhost:$port/data/$key > /dev/null
    
    return 0
}

# Print banner
echo -e "${GREEN}==========================================================${NC}"
echo -e "${GREEN}  Generating comprehensive metrics for Distributed Cache  ${NC}"
echo -e "${GREEN}==========================================================${NC}"
echo "This script will generate diverse metrics for your Grafana dashboard"
echo "Running time: $RUN_TIME seconds"
echo ""

# Initialize counters
total_ops=0
hits=0
misses=0
errors=0
puts=0
gets=0
deletes=0

# Set start time
start_time=$(date +%s)
current_time=$start_time
end_time=$((start_time + RUN_TIME))

# Preload some keys to have initial data
echo -e "${YELLOW}Preloading initial cache data...${NC}"
for i in $(seq 1 $((TOTAL_KEYS / 2))); do
    node_type="aux"
    node_index=$((RANDOM % ${#AUX_NODES[@]}))
    key="init_key_$i"
    value=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $((50 + RANDOM % 200)) | head -n 1)
    
    put_cache $node_type $node_index $key "$value"
    puts=$((puts + 1))
    total_ops=$((total_ops + 1))
    
    # Don't overload system during initialization
    sleep 0.1
done

echo ""
echo -e "${YELLOW}Starting continuous operation simulation...${NC}"

# Main loop - run until time expires
while [ $current_time -lt $end_time ]; do
    # Random operation type: 0=GET, 1=PUT, 2=DELETE
    op_type=$((RANDOM % 10))
    
    # Simulate more GETs than PUTs or DELETEs
    if [ $op_type -lt 7 ]; then
        # GET operation
        node_type=$([ $((RANDOM % 2)) -eq 0 ] && echo "master" || echo "aux")
        node_index=0
        
        if [ "$node_type" == "master" ]; then
            node_index=$((RANDOM % ${#MASTER_NODES[@]}))
        else
            node_index=$((RANDOM % ${#AUX_NODES[@]}))
        fi
        
        # Use existing key with 90% probability, random key otherwise
        if [ $((RANDOM % 10)) -lt 9 ]; then
            key_id=$((RANDOM % TOTAL_KEYS + 1))
            if [ $((RANDOM % 2)) -eq 0 ]; then
                key="init_key_$key_id"
            else
                key="test_key_$key_id"
            fi
        else
            key="nonexistent_key_$((RANDOM % 1000))"
        fi
        
        get_cache $node_type $node_index $key
        result=$?
        gets=$((gets + 1))
        
        if [ $result -eq 0 ]; then
            hits=$((hits + 1))
        elif [ $result -eq 2 ]; then
            misses=$((misses + 1))
        else
            errors=$((errors + 1))
        fi
    elif [ $op_type -lt 9 ]; then
        # PUT operation
        node_type=$([ $((RANDOM % 2)) -eq 0 ] && echo "master" || echo "aux")
        node_index=0
        
        if [ "$node_type" == "master" ]; then
            node_index=$((RANDOM % ${#MASTER_NODES[@]}))
        else
            node_index=$((RANDOM % ${#AUX_NODES[@]}))
        fi
        
        key="test_key_$((RANDOM % TOTAL_KEYS + 1))"
        # Generate random values of different sizes
        value=$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w $((50 + RANDOM % 450)) | head -n 1)
        
        put_cache $node_type $node_index $key "$value"
        result=$?
        puts=$((puts + 1))
        
        if [ $result -ne 0 ]; then
            errors=$((errors + 1))
        fi
    else
        # DELETE operation
        node_type=$([ $((RANDOM % 2)) -eq 0 ] && echo "master" || echo "aux")
        node_index=0
        
        if [ "$node_type" == "master" ]; then
            node_index=$((RANDOM % ${#MASTER_NODES[@]}))
        else
            node_index=$((RANDOM % ${#AUX_NODES[@]}))
        fi
        
        if [ $((RANDOM % 2)) -eq 0 ]; then
            key_id=$((RANDOM % (TOTAL_KEYS / 2) + 1))
            key="init_key_$key_id"
        else
            key_id=$((RANDOM % TOTAL_KEYS + 1))
            key="test_key_$key_id"
        fi
        
        delete_cache $node_type $node_index $key
        deletes=$((deletes + 1))
    fi
    
    total_ops=$((total_ops + 1))
    
    # Update current time
    current_time=$(date +%s)
    
    # Print status every 50 operations
    if [ $((total_ops % 50)) -eq 0 ]; then
        elapsed=$((current_time - start_time))
        remaining=$((end_time - current_time))
        echo ""
        echo -e "${YELLOW}Status Update:${NC}"
        echo "→ Operations: $total_ops (PUT: $puts, GET: $gets, DELETE: $deletes)"
        echo "→ Cache: $hits hits, $misses misses, $errors errors"
        echo "→ Hit rate: $(awk "BEGIN {printf \"%.1f%%\", ($hits/($hits+$misses))*100}")"
        echo "→ Time elapsed: ${elapsed}s, remaining: ${remaining}s"
        echo ""
    fi
    
    # Variable delay between operations
    sleep 0.$(( RANDOM % 5 + 1 ))
done

echo ""
echo -e "${GREEN}==========================================================${NC}"
echo -e "${GREEN}  Test metrics generation complete!                       ${NC}"
echo -e "${GREEN}==========================================================${NC}"
echo ""
echo "Final statistics:"
echo "→ Total operations: $total_ops (PUT: $puts, GET: $gets, DELETE: $deletes)"
echo "→ Cache statistics: $hits hits, $misses misses, $errors errors"
echo "→ Hit rate: $(awk "BEGIN {printf \"%.1f%%\", ($hits/($hits+$misses))*100}")"
echo ""
echo "To view your dashboard in Grafana:"
echo "1. Open http://localhost:4000 in your browser"
echo "2. Log in with admin/admin (change password if prompted)"
echo "3. Click on 'Dashboards' in the left sidebar"
echo "4. Select the 'Cache System Dashboard'"
echo ""
echo "Your dashboard should be automatically updated with the metrics generated."
