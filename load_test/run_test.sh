#!/bin/bash

# Default values
HOST=${HOST:-"http://localhost:8081"}
USERS=${USERS:-100}
SPAWN_RATE=${SPAWN_RATE:-10}
RUN_TIME=${RUN_TIME:-"5m"}
HEADLESS=${HEADLESS:-false}

# Help message
function show_help {
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  -h, --host       Target host URL (default: $HOST)"
    echo "  -u, --users      Number of concurrent users (default: $USERS)"
    echo "  -r, --rate       User spawn rate per second (default: $SPAWN_RATE)"
    echo "  -t, --time       Test duration in time format (default: $RUN_TIME)"
    echo "  -l, --headless   Run in headless mode (default: $HEADLESS)"
    echo "  --help           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -h http://localhost:8081 -u 200 -r 20 -t 10m"
    echo "  $0 --host http://localhost:8081 --users 200 --rate 20 --time 10m --headless"
    exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--host)
            HOST="$2"
            shift 2
            ;;
        -u|--users)
            USERS="$2"
            shift 2
            ;;
        -r|--rate)
            SPAWN_RATE="$2"
            shift 2
            ;;
        -t|--time)
            RUN_TIME="$2"
            shift 2
            ;;
        -l|--headless)
            HEADLESS=true
            shift
            ;;
        --help)
            show_help
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            ;;
    esac
done

# Check for Python and pip
if ! command -v python3 &> /dev/null; then
    echo "Python 3 is required but not found."
    exit 1
fi

if ! command -v pip3 &> /dev/null; then
    echo "pip3 is required but not found."
    exit 1
fi

# Install dependencies if not already installed
echo "Installing dependencies..."
pip3 install -r requirements.txt

# Print test configuration
echo "Starting load test with the following configuration:"
echo "  Host:       $HOST"
echo "  Users:      $USERS"
echo "  Spawn Rate: $SPAWN_RATE"
echo "  Duration:   $RUN_TIME"
echo "  Headless:   $HEADLESS"
echo ""

# Run the test
if [ "$HEADLESS" = true ]; then
    echo "Running in headless mode..."
    locust -f locustfile.py --host=$HOST --users=$USERS --spawn-rate=$SPAWN_RATE --run-time=$RUN_TIME --headless --csv=results_$(date +%Y%m%d_%H%M%S)
else
    echo "Starting Locust web interface..."
    locust -f locustfile.py --host=$HOST
fi

echo "Load test complete!"

