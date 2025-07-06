<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Distributed-Cache-System](#distributed-cache-system)
  - [Features](#features)
  - [Architecture](#architecture)
  - [Data flow](#data-flow)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Usage](#usage)
    - [API Endpoints](#api-endpoints)
    - [Example Requests](#example-requests)
  - [Monitoring](#monitoring)
    - [Prometheus Metrics](#prometheus-metrics)
    - [Grafana Dashboards](#grafana-dashboards)
  - [Load Testing](#load-testing)
  - [Recovery and Backup](#recovery-and-backup)
  - [Development](#development)
  - [TODO](#todo)
  - [Contributing](#contributing)
  - [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Distributed-Cache-System
This implementation of the distributed cache system is an attempt to make a high-performant, scalable and fault-tolerant caching solution to improve performance and efficiency of distributed systems. It utilizes master-auxiliary architecture, where master servers select which auxiliary server to choose when getting or putting key-vals. 

![Architecture of Distributed Cache System](distributed-cache.png)
## Features

- **High Performance**: Scalable distributed cache system with efficient data access
- **Consistent Hashing**: Intelligent key distribution across multiple nodes
- **LRU Cache**: Least Recently Used caching algorithm with Doubly Linked List and Hashmap
- **High Availability**: Redundant master servers with load balancing
- **Fault Tolerance**: Automatic rebalancing when nodes are added/removed
- **Backup & Recovery**: Periodic backups and restore functionality for data persistence
- **Real-time Monitoring**: Comprehensive metrics collection with Prometheus
- **Visualization**: Grafana dashboards for system performance monitoring
- **Easy Deployment**: Docker containerization for seamless setup and scaling
- **Load Testing**: Built-in tools for performance and stress testing
- **Horizontal Scaling**: Add more nodes to increase system capacity
- **RESTful API**: Simple HTTP interface for cache operations

## Architecture

The Distributed Cache System consists of the following components:

- **Master Server**: The master node acts as the central coordinator and is responsible for handling client requests. It receives key-value pairs from clients and determines the appropriate auxiliary server to store the data using a consistent hashing algorithm. The master node also handles the retrieval of data from auxiliary servers and forwards read requests accordingly.

- **Auxiliary (Aux) Servers**: The auxiliary servers store the cached data. They are replicated instances deployed in a consistent hash ring to ensure efficient distribution and load balancing. Each auxiliary server is responsible for maintaining a local LRU (Least Recently Used) cache, implemented using a combination of a hashtable and a doubly linked list (DLL). This cache allows for fast access and eviction of less frequently used data.

- **Load Balancing**: The load balancer, typically implemented using nginx, acts as an intermediary between the clients and the master node. It distributes incoming client requests across multiple instances of the master node, enabling horizontal scaling and improved availability.

- **Metrics Monitoring**: Prometheus is integrated into both master and auxliliary server to collect count and response time of GET and POST requests.

- **Visualization**: Grafana is used to visualize the collected metrics from Prometheus, providing insightful dashboards and graphs for monitoring the cache system's performance.
  
- **Docker**: The project utilizes Docker to containerize and deploy the master and auxiliary servers, making it easy to scale and manage the system.

-  **Load Test**: To ensure the system's reliability and performance, a Python script for load testing was developed. The script utilizes Locust, a popular load testing framework.The script defines a set of tasks for load testing the cache system. The put task sends a POST request to the /data endpoint, randomly selecting a payload from a predefined list. The get task sends a GET request to the /data/{key} endpoint, randomly selecting a key from a predefined list. By utilizing this load testing script, it can simulate multiple concurrent users and measure the performance and scalability of the distributed cache system.



## Data flow

1. Client Interaction: Clients interact to distributed cache system via Load Balancer.
   
2. Load Balancer Routing: The load balancer receives the client requests and distributes them among the available instances of the master node using Round-Robin policy. This ensures a balanced workload and improved performance. The configuration can be tweaked to increase the connection pool or increase the number of works processed by each of the worker.
   
3. Master Node Processing: The master node receives the client requests and performs the necessary operations. For write requests (storing key-value pairs), the master node applies a consistent hashing algorithm to determine the appropriate auxiliary server for data storage. It then forwards the data to the selected auxiliary server for caching. For read requests, the master node identifies the auxiliary server holding the requested data and retrieves it from there.
   
4. Auxiliary Server Caching: The auxiliary servers receive data from the master node and store it in their local LRU cache. This cache allows for efficient data access and eviction based on usage patterns.
   
5. Response to Clients: Once the master node receives a response from the auxiliary server (in the case of read requests) or completes the necessary operations (in the case of write requests), it sends the response back to the client through the load balancer. Clients can then utilize the retrieved data or receive confirmation of a successful operation.

## Recovery
- Health of auxiliary servers are monitored by master servers in a regular interval. If any of the auliliary servers go down or respawned, the master knows about it and rebalance the key-val mappings using consistent hashing.
  
- In case if one or more auxiliary nodes are shutdown, the key-vals mappings are sent to the master node which rebalance them using consistent hashing. 
  
- Each auxiliary server backs up data in their container volume every 10 sec, incase a catastrophic failure occurs. These backups are then used when the server is respawned.
  
- In case if one or more auxiliary nodes are respawned, the key-vals mappings from the corresponding nodes in the hash ring are rebalanced using consistent hashing.
  
- When redistributing/remapping the key-vals, a copy is backed up in the shared volume of master containers incase if the whole system goes down and has to be quickly respawned. The backups can be used to salvage as much data as possible. When respawning the backup is rebalanced to corresponding auxiliary server.


## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/cruzelx/Distributed-Cache-System.git
   cd Distributed-Cache-System
   ```

2. Build and run the Docker containers:
   ```bash
   docker-compose up --build
   ```

3. Verify that all services are running:
   ```bash
   docker-compose ps
   ```

4. Access the system at http://localhost:8081

## Configuration

The distributed cache system offers extensive configuration options through environment variables in the `docker-compose.yml` file:

### Master Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| PORT | HTTP server port | 8000 |
| REPLICATION_FACTOR | Number of virtual nodes per physical node in consistent hashing | 10 |
| HEALTH_CHECK_INTERVAL | Interval in seconds between health checks | 10 |
| BACKUP_INTERVAL | Interval in seconds between backups | 120 |

### Auxiliary Server Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| PORT | HTTP server port | 3001-3003 |
| MASTER_ADDRESS | Address of master server/load balancer | nginx:80 |
| CACHE_SIZE | Maximum number of items in the LRU cache | 1000 |
| BACKUP_DIR | Directory where backups are stored | /app/backup |
| BACKUP_INTERVAL | Interval in seconds between automatic backups | 60 |

### Scaling Configuration

To adjust the number of auxiliary servers, modify the `docker-compose.yml` file by adding or removing auxiliary server instances. Make sure to:
1. Assign unique ports
2. Create corresponding volumes
3. Update the Prometheus configuration

## Usage

The distributed cache system provides a RESTful API for cache operations.

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/data` | POST | Add or update a key-value pair |
| `/data/{key}` | GET | Retrieve a value by key |
| `/data/{key}` | DELETE | Remove a key-value pair |
| `/health` | GET | Check service health |
| `/nodes` | GET | List all nodes in the system |
| `/nodes` | POST | Register a new node |
| `/nodes/{id}` | GET | Get node details |
| `/nodes/{id}` | DELETE | Remove a node |
| `/backup` | POST | Trigger a backup |
| `/restore` | POST | Restore from a backup |
| `/rebalance` | POST | Trigger data rebalancing |

### Example Requests

**Store a value:**
```bash
curl -X POST http://localhost:8081/data \
  -H "Content-Type: application/json" \
  -d '{"key":"user1", "value":"John Doe", "ttl": 3600}'
```

**Retrieve a value:**
```bash
curl http://localhost:8081/data/user1
```

**Delete a value:**
```bash
curl -X DELETE http://localhost:8081/data/user1
```

**Check health status:**
```bash
curl http://localhost:8081/health
```

**Trigger a manual backup:**
```bash
curl -X POST http://localhost:8081/backup
```

## Monitoring

The system includes comprehensive monitoring with Prometheus and Grafana.

### Prometheus Metrics

Prometheus collects metrics from all services. Access the Prometheus interface at http://localhost:9090.

Key metrics include:
- Cache hit/miss rates
- Request latencies
- Operation counts
- Memory usage
- Node health status

### Grafana Dashboards

A pre-configured Grafana dashboard is available at http://localhost:4000 (login with admin/admin).

The dashboard includes:
- System overview
- Cache performance metrics
- Node health status
- Operation metrics
- System resource usage

## Load Testing

The system includes a comprehensive load testing suite using Locust:

1. Navigate to the load testing directory:
   ```bash
   cd load_test
   ```

2. Run the load testing script:
   ```bash
   ./run_test.sh
   ```

3. For headless mode with custom parameters:
   ```bash
   ./run_test.sh --host http://localhost:8081 --users 200 --rate 20 --time 10m --headless
   ```

Available options:
- `--host`: Target host URL (default: http://localhost:8081)
- `--users`: Number of concurrent users (default: 100)
- `--rate`: User spawn rate per second (default: 10)
- `--time`: Test duration (default: 5m)
- `--headless`: Run in headless mode without UI

## Recovery and Backup

The system includes several mechanisms for fault tolerance and recovery:

1. **Health Monitoring**: Master servers continuously monitor the health of auxiliary nodes.

2. **Automatic Rebalancing**: When nodes are added, removed, or fail, the system automatically rebalances data.

3. **Periodic Backups**: Auxiliary servers create backups of their cache data at configurable intervals.

4. **Restore Capabilities**: The system can restore from backups in case of failures.

5. **Fault Isolation**: Issues with individual nodes don't affect the overall system.

### Recovery Scenarios

- **Node Failure**: If an auxiliary node fails, the master detects this during health checks and rebalances the data to other nodes.

- **Node Addition**: When a new auxiliary node starts, it registers with the master and triggers a rebalancing to distribute load.

- **Full System Recovery**: The system can be completely restored from backups when restarted after a catastrophic failure.

## Development

For development and extension of the distributed cache system:

1. **Setup Development Environment**:
   ```bash
   # Clone the repository
   git clone https://github.com/cruzelx/Distributed-Cache-System.git
   cd Distributed-Cache-System
   
   # Initialize Go modules
   go mod tidy
   ```

2. **Run Tests**:
   ```bash
   go test ./...
   ```

3. **Local Development**:
   ```bash
   # Run master server
   go run cmd/master/main.go
   
   # Run auxiliary server
   go run cmd/auxiliary/main.go
   ```

## TODO
-  [ ] Support for expirable key-value pairs with TTL
-  [ ] Local cache in Master server for better performance
-  [ ] Implement replicas for Auxiliary servers with leader selection
-  [ ] Authentication and authorization
-  [ ] Add more metrics and alerts
-  [ ] Support for different data types and data structures

## Contributing

Contributions to the Distributed Cache System are welcome! If you find any issues or have suggestions for improvements, please submit a GitHub issue or create a pull request.

## License

This project is licensed under the [MIT License](LICENSE).