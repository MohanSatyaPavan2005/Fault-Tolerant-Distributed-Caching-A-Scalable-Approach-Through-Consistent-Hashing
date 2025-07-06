package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"bytes"

	"github.com/mmspavan/distributed-cache-system/Distributed-Cache-System/internal/hashing"
	"github.com/mmspavan/distributed-cache-system/Distributed-Cache-System/pkg/models"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Configuration constants with defaults
const (
	defaultPort             = "8000"
	defaultReplicationFactor = 10
	defaultHealthCheckInterval = 10 // seconds
	defaultBackupInterval     = 300 // seconds
)

// Global variables
var (
	// Server start time for uptime calculation
	startTime = time.Now()

	// Consistent hash ring for node management
	hashRing *hashing.ConsistentHash

	// Map to store auxiliary node information
	auxNodes      = make(map[string]*models.NodeInfo)
	auxNodesMutex = sync.RWMutex{}

	// Metrics
	cacheGetCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_get_requests_total",
			Help: "Total number of get requests",
		},
		[]string{"status"},
	)
	cachePutCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_put_requests_total",
			Help: "Total number of put requests",
		},
		[]string{"status"},
	)
	cacheDeleteCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_delete_requests_total",
			Help: "Total number of delete requests",
		},
		[]string{"status"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Duration of requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
	nodeStatusGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_status",
			Help: "Status of auxiliary nodes (1=up, 0=down)",
		},
		[]string{"node_id"},
	)
)

// MasterServer represents the master node in the distributed cache system
type MasterServer struct {
	Port               string
	ReplicationFactor  int
	HealthCheckInterval time.Duration
	BackupInterval     time.Duration
	HashRing           *hashing.ConsistentHash
}

// handleHealth handles the /health endpoint for server health checks
func (s *MasterServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Count healthy nodes
	auxNodesMutex.RLock()
	totalNodes := len(auxNodes)
	healthyNodes := 0
	for _, node := range auxNodes {
		if node.Status == models.NodeStatusUp {
			healthyNodes++
		}
	}
	auxNodesMutex.RUnlock()

	// Create health check response
	response := models.HealthCheckResponse{
		Status:        models.NodeStatusUp,
		Timestamp:     time.Now(),
		Uptime:        int64(time.Since(startTime).Seconds()),
		Version:       "1.0.0",
		CacheSize:     totalNodes,
		CacheCapacity: totalNodes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRebalance handles the /rebalance endpoint for manual rebalancing
func (s *MasterServer) handleRebalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Start rebalancing in a separate goroutine
	go s.triggerRebalance(models.RebalanceReasonManual, "")

	// Send response
	response := models.RebalanceResponse{
		Success:   true,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleBackup handles the /backup endpoint for creating backups
func (s *MasterServer) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.BackupRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	req.Timestamp = time.Now()

	// Start backup in a separate goroutine
	go s.triggerBackup(req)

	// Send response
	response := models.BackupResponse{
		Success:   true,
		Timestamp: time.Now(),
		BackupID:  fmt.Sprintf("backup-%d", time.Now().UnixNano()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRestore handles the /restore endpoint for restoring from backups
func (s *MasterServer) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req models.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.BackupID == "" {
		http.Error(w, "BackupID is required", http.StatusBadRequest)
		return
	}

	req.Timestamp = time.Now()

	// Start restore in a separate goroutine
	go s.triggerRestore(req)

	// Send response
	response := models.RestoreResponse{
		Success:   true,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// runHealthChecks periodically checks the health of all auxiliary nodes
func (s *MasterServer) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(s.HealthCheckInterval)
	defer ticker.Stop()

	log.Printf("Starting health check routine with interval of %v", s.HealthCheckInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Health check routine stopping")
			return
		case <-ticker.C:
			s.checkAllNodes()
		}
	}
}

// checkAllNodes performs health checks on all registered auxiliary nodes
func (s *MasterServer) checkAllNodes() {
	log.Println("Running health checks on all nodes")

	// Get a copy of the nodes to check
	auxNodesMutex.RLock()
	nodesToCheck := make(map[string]*models.NodeInfo)
	for id, node := range auxNodes {
		nodesToCheck[id] = node
	}
	auxNodesMutex.RUnlock()

	// Check each node
	for id, node := range nodesToCheck {
		go s.checkNodeHealth(id, node)
	}
}

// checkNodeHealth checks the health of a specific node
func (s *MasterServer) checkNodeHealth(nodeID string, node *models.NodeInfo) {
	// Create health check request
	healthURL := fmt.Sprintf("http://%s:%d/health", node.Address, node.Port)
	req, err := http.NewRequest(http.MethodGet, healthURL, nil)
	if err != nil {
		log.Printf("Failed to create health check request for node %s: %v", nodeID, err)
		s.markNodeDown(nodeID)
		return
	}

	// Send request with timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Health check failed for node %s: %v", nodeID, err)
		s.markNodeDown(nodeID)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		log.Printf("Health check for node %s returned status %d", nodeID, resp.StatusCode)
		s.markNodeDown(nodeID)
		return
	}

	// Parse response
	var healthResp models.HealthCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		log.Printf("Failed to parse health check response from node %s: %v", nodeID, err)
		s.markNodeDown(nodeID)
		return
	}

	// Update node status
	auxNodesMutex.Lock()
	if node, exists := auxNodes[nodeID]; exists {
		node.Status = healthResp.Status
		node.Timestamp = time.Now()
		
		// Add health metrics to node metadata
		if node.Metadata == nil {
			node.Metadata = make(map[string]string)
		}
		node.Metadata["cache_size"] = strconv.Itoa(healthResp.CacheSize)
		node.Metadata["uptime"] = strconv.FormatInt(healthResp.Uptime, 10)
	}
	auxNodesMutex.Unlock()

	// Update metrics
	if healthResp.Status == models.NodeStatusUp {
		nodeStatusGauge.WithLabelValues(nodeID).Set(1)
	} else {
		nodeStatusGauge.WithLabelValues(nodeID).Set(0)
	}
}

// markNodeDown marks a node as down and triggers rebalancing if needed
func (s *MasterServer) markNodeDown(nodeID string) {
	auxNodesMutex.Lock()
	node, exists := auxNodes[nodeID]
	wasUp := false
	if exists {
		wasUp = node.Status == models.NodeStatusUp
		node.Status = models.NodeStatusDown
		node.Timestamp = time.Now()
	}
	auxNodesMutex.Unlock()

	if exists {
		// Update metrics
		nodeStatusGauge.WithLabelValues(nodeID).Set(0)
		
		log.Printf("Node %s marked as down", nodeID)
		
		// Trigger rebalancing if the node was previously up
		if exists && wasUp {
			go s.triggerRebalance(models.RebalanceReasonNodeFailed, nodeID)
		}
	}
}

// coordinateBackups manages periodic backups of the cache system
func (s *MasterServer) coordinateBackups(ctx context.Context) {
	ticker := time.NewTicker(s.BackupInterval)
	defer ticker.Stop()

	log.Printf("Starting backup coordination routine with interval of %v", s.BackupInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Backup coordination routine stopping")
			return
		case <-ticker.C:
			s.triggerBackup(models.BackupRequest{
				Timestamp: time.Now(),
			})
		}
	}
}

// triggerBackup sends backup requests to all healthy auxiliary nodes
func (s *MasterServer) triggerBackup(req models.BackupRequest) {
	log.Println("Triggering backup on all healthy nodes")

	// Get a list of healthy nodes
	auxNodesMutex.RLock()
	healthyNodes := make(map[string]*models.NodeInfo)
	for id, node := range auxNodes {
		if node.Status == models.NodeStatusUp {
			healthyNodes[id] = node
		}
	}
	auxNodesMutex.RUnlock()

	if len(healthyNodes) == 0 {
		log.Println("No healthy nodes available for backup")
		return
	}

	// Send backup requests to each healthy node
	for id, node := range healthyNodes {
		go func(nodeID string, node *models.NodeInfo) {
			// Create backup request
			backupURL := fmt.Sprintf("http://%s:%d/backup", node.Address, node.Port)
			reqBody, _ := json.Marshal(req)
			httpReq, err := http.NewRequest(http.MethodPost, backupURL, bytes.NewBuffer(reqBody))
			if err != nil {
				log.Printf("Failed to create backup request for node %s: %v", nodeID, err)
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")

			// Send request with timeout
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				log.Printf("Backup failed for node %s: %v", nodeID, err)
				return
			}
			defer resp.Body.Close()

			// Check response status
			if resp.StatusCode != http.StatusOK {
				log.Printf("Backup request for node %s returned status %d", nodeID, resp.StatusCode)
				return
			}

			log.Printf("Backup successful for node %s", nodeID)
		}(id, node)
	}
}

// triggerRestore sends restore requests to all auxiliary nodes
func (s *MasterServer) triggerRestore(req models.RestoreRequest) {
	log.Printf("Triggering restore from backup %s", req.BackupID)

	// Get all nodes
	auxNodesMutex.RLock()
	allNodes := make(map[string]*models.NodeInfo)
	for id, node := range auxNodes {
		allNodes[id] = node
	}
	auxNodesMutex.RUnlock()

	if len(allNodes) == 0 {
		log.Println("No nodes available for restore")
		return
	}

	// Trigger restore on each node
	for id, node := range allNodes {
		go func(nodeID string, node *models.NodeInfo) {
			// Create restore request
			restoreURL := fmt.Sprintf("http://%s:%d/restore", node.Address, node.Port)
			reqBody, _ := json.Marshal(req)
			httpReq, err := http.NewRequest(http.MethodPost, restoreURL, bytes.NewBuffer(reqBody))
			if err != nil {
				log.Printf("Failed to create restore request for node %s: %v", nodeID, err)
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")

			// Send request with timeout
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				log.Printf("Restore failed for node %s: %v", nodeID, err)
				return
			}
			defer resp.Body.Close()

			// Check response status
			if resp.StatusCode != http.StatusOK {
				log.Printf("Restore request for node %s returned status %d", nodeID, resp.StatusCode)
				return
			}

			log.Printf("Restore successful for node %s", nodeID)
		}(id, node)
	}

	// Trigger rebalancing after restore
	go s.triggerRebalance(models.RebalanceReasonManual, "")
}

// triggerRebalance starts the rebalancing process
func (s *MasterServer) triggerRebalance(reason models.RebalanceReason, nodeID string) {
	log.Printf("Triggering rebalance: reason=%s, nodeID=%s", reason, nodeID)

	// Get all healthy nodes
	auxNodesMutex.RLock()
	healthyNodes := make(map[string]*models.NodeInfo)
	for id, node := range auxNodes {
		if node.Status == models.NodeStatusUp {
			healthyNodes[id] = node
		}
	}
	auxNodesMutex.RUnlock()

	if len(healthyNodes) == 0 {
		log.Println("No healthy nodes available for rebalancing")
		return
	}

	// Create rebalance request
	req := models.RebalanceRequest{
		Reason:    reason,
		NodeID:    nodeID,
		Timestamp: time.Now(),
	}

	// Send rebalance request to each healthy node
	for id, node := range healthyNodes {
		go func(nodeID string, node *models.NodeInfo) {
			// Create rebalance request
			rebalanceURL := fmt.Sprintf("http://%s:%d/rebalance", node.Address, node.Port)
			reqBody, _ := json.Marshal(req)
			httpReq, err := http.NewRequest(http.MethodPost, rebalanceURL, bytes.NewBuffer(reqBody))
			if err != nil {
				log.Printf("Failed to create rebalance request for node %s: %v", nodeID, err)
				return
			}

			httpReq.Header.Set("Content-Type", "application/json")

			// Send request with timeout
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				log.Printf("Rebalance failed for node %s: %v", nodeID, err)
				return
			}
			defer resp.Body.Close()

			// Check response status
			if resp.StatusCode != http.StatusOK {
				log.Printf("Rebalance request for node %s returned status %d", nodeID, resp.StatusCode)
				return
			}

			log.Printf("Rebalance successful for node %s", nodeID)
		}(id, node)
	}
}

// handleDeleteRequest handles DELETE requests for a specific key
func (s *MasterServer) handleDeleteRequest(w http.ResponseWriter, r *http.Request, key string) {
	// Get node for the key using consistent hashing
	auxNodesMutex.RLock()
	nodeCount := len(auxNodes)
	auxNodesMutex.RUnlock()

	if nodeCount == 0 {
		http.Error(w, "No auxiliary nodes available", http.StatusServiceUnavailable)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}

	nodeName := s.HashRing.Get(key)
	if nodeName == "" {
		http.Error(w, "Failed to determine node for key", http.StatusInternalServerError)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}

	// Get node info
	auxNodesMutex.RLock()
	node, exists := auxNodes[nodeName]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Selected node not available", http.StatusInternalServerError)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}

	// Forward request to auxiliary node
	auxURL := fmt.Sprintf("http://%s:%d/data/%s", node.Address, node.Port, key)
	auxReq, err := http.NewRequest(http.MethodDelete, auxURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			auxReq.Header.Add(name, value)
		}
	}

	// Send request to auxiliary node
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(auxReq)
	if err != nil {
		http.Error(w, "Failed to forward request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheDeleteCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code and write response
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	// Update metrics
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		cacheDeleteCounter.WithLabelValues("success").Inc()
	} else {
		cacheDeleteCounter.WithLabelValues("error").Inc()
	}

	log.Printf("DELETE request for key '%s' forwarded to node %s, status: %d", key, nodeName, resp.StatusCode)
}

// handleNodes handles the /nodes endpoint for listing and registering nodes
func (s *MasterServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all nodes
		s.listNodes(w, r)
	case http.MethodPost:
		// Register a new node
		s.registerNode(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listNodes handles GET requests to list all auxiliary nodes
func (s *MasterServer) listNodes(w http.ResponseWriter, r *http.Request) {
	auxNodesMutex.RLock()
	nodes := make([]models.NodeInfo, 0, len(auxNodes))
	for _, node := range auxNodes {
		nodes = append(nodes, *node)
	}
	auxNodesMutex.RUnlock()

	response := models.NodesListResponse{
		Success: true,
		Nodes:   nodes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// registerNode handles POST requests to register a new auxiliary node
func (s *MasterServer) registerNode(w http.ResponseWriter, r *http.Request) {
	// Decode request
	var req models.NodeRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Name == "" || req.Address == "" || req.Port == 0 {
		http.Error(w, "Invalid node data: name, address, and port are required", http.StatusBadRequest)
		return
	}

	if req.Type != models.NodeTypeAux {
		http.Error(w, "Only auxiliary nodes can be registered", http.StatusBadRequest)
		return
	}

	// Generate node ID
	nodeID := fmt.Sprintf("%s-%s-%d", req.Type, req.Name, req.Port)

	// Check if node already exists
	auxNodesMutex.RLock()
	_, exists := auxNodes[nodeID]
	auxNodesMutex.RUnlock()

	if exists {
		// Update existing node
		auxNodesMutex.Lock()
		auxNodes[nodeID] = &models.NodeInfo{
			ID:        nodeID,
			Name:      req.Name,
			Address:   req.Address,
			Port:      req.Port,
			Type:      req.Type,
			Status:    models.NodeStatusUp,
			Timestamp: time.Now(),
			Metadata:  req.Metadata,
		}
		auxNodesMutex.Unlock()

		log.Printf("Updated auxiliary node: %s (%s:%d)", nodeID, req.Address, req.Port)
	} else {
		// Add to hash ring
		s.HashRing.Add(nodeID)

		// Register new node
		auxNodesMutex.Lock()
		auxNodes[nodeID] = &models.NodeInfo{
			ID:        nodeID,
			Name:      req.Name,
			Address:   req.Address,
			Port:      req.Port,
			Type:      req.Type,
			Status:    models.NodeStatusUp,
			Timestamp: time.Now(),
			Metadata:  req.Metadata,
		}
		auxNodesMutex.Unlock()

		// Update metrics
		nodeStatusGauge.WithLabelValues(nodeID).Set(1)

		log.Printf("Registered new auxiliary node: %s (%s:%d)", nodeID, req.Address, req.Port)

		// Trigger rebalancing
		go s.triggerRebalance(models.RebalanceReasonNodeAdded, nodeID)
	}

	// Send response
	response := models.NodeRegistrationResponse{
		Success: true,
		ID:      nodeID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// handleNodesWithID handles the /nodes/{id} endpoint for specific node operations
func (s *MasterServer) handleNodesWithID(w http.ResponseWriter, r *http.Request) {
	// Extract node ID from URL path
	nodeID := strings.TrimPrefix(r.URL.Path, "/nodes/")
	if nodeID == "" {
		http.Error(w, "Node ID cannot be empty", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get node info
		s.getNodeInfo(w, r, nodeID)
	case http.MethodDelete:
		// Deregister node
		s.deregisterNode(w, r, nodeID)
	case http.MethodPut:
		// Update node status
		s.updateNodeStatus(w, r, nodeID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getNodeInfo handles GET requests to retrieve information about a specific node
func (s *MasterServer) getNodeInfo(w http.ResponseWriter, r *http.Request, nodeID string) {
	// Get node info
	auxNodesMutex.RLock()
	node, exists := auxNodes[nodeID]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// deregisterNode handles DELETE requests to deregister a node
func (s *MasterServer) deregisterNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	// Check if node exists
	auxNodesMutex.RLock()
	_, exists := auxNodes[nodeID]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Remove from hash ring
	s.HashRing.Remove(nodeID)

	// Remove from nodes map
	auxNodesMutex.Lock()
	delete(auxNodes, nodeID)
	auxNodesMutex.Unlock()

	// Update metrics
	nodeStatusGauge.DeleteLabelValues(nodeID)

	log.Printf("Deregistered auxiliary node: %s", nodeID)

	// Trigger rebalancing
	go s.triggerRebalance(models.RebalanceReasonNodeRemoved, nodeID)

	// Send response
	response := models.NodeDeregistrationResponse{
		Success: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateNodeStatus handles PUT requests to update node status
func (s *MasterServer) updateNodeStatus(w http.ResponseWriter, r *http.Request, nodeID string) {
	// Decode request
	var nodeInfo models.NodeInfo
	if err := json.NewDecoder(r.Body).Decode(&nodeInfo); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check if node exists
	auxNodesMutex.RLock()
	node, exists := auxNodes[nodeID]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}

	// Update node status
	auxNodesMutex.Lock()
	node.Status = nodeInfo.Status
	node.Timestamp = time.Now()
	if nodeInfo.Metadata != nil {
		node.Metadata = nodeInfo.Metadata
	}
	auxNodesMutex.Unlock()

	// Update metrics
	if nodeInfo.Status == models.NodeStatusUp {
		nodeStatusGauge.WithLabelValues(nodeID).Set(1)
	} else {
		nodeStatusGauge.WithLabelValues(nodeID).Set(0)
	}

	log.Printf("Updated node status: %s -> %s", nodeID, nodeInfo.Status)

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// NewMasterServer creates a new master server with the given configuration
func NewMasterServer() *MasterServer {
	// Load configuration from environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	replicationFactor, err := strconv.Atoi(os.Getenv("REPLICATION_FACTOR"))
	if err != nil || replicationFactor < 1 {
		replicationFactor = defaultReplicationFactor
	}

	healthCheckInterval, err := strconv.Atoi(os.Getenv("HEALTH_CHECK_INTERVAL"))
	if err != nil || healthCheckInterval < 1 {
		healthCheckInterval = defaultHealthCheckInterval
	}

	backupInterval, err := strconv.Atoi(os.Getenv("BACKUP_INTERVAL"))
	if err != nil || backupInterval < 1 {
		backupInterval = defaultBackupInterval
	}

	// Initialize consistent hash ring
	hashRing = hashing.NewConsistentHash(replicationFactor)

	// Log configuration
	log.Printf("Master Server Configuration:")
	log.Printf("- Port: %s", port)
	log.Printf("- Replication Factor: %d", replicationFactor)
	log.Printf("- Health Check Interval: %d seconds", healthCheckInterval)
	log.Printf("- Backup Interval: %d seconds", backupInterval)

	return &MasterServer{
		Port:               port,
		ReplicationFactor:  replicationFactor,
		HealthCheckInterval: time.Duration(healthCheckInterval) * time.Second,
		BackupInterval:     time.Duration(backupInterval) * time.Second,
		HashRing:           hashRing,
	}
}

// Start starts the master server and all its background processes
func (s *MasterServer) Start() error {
	// Set up HTTP routes
	s.setupRoutes()

	// Create a context for background processes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background health check process
	go s.runHealthChecks(ctx)

	// Start background backup coordination process
	go s.coordinateBackups(ctx)

	// Create HTTP server
	server := &http.Server{
		Addr: ":" + s.Port,
	}

	// Channel to signal server errors
	serverCh := make(chan error, 1)

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Master server starting on port %s...", s.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			serverCh <- err
		}
	}()

	// Channel to handle OS signals
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	var err error
	select {
	case <-stopCh:
		log.Println("Shutdown signal received")
	case err = <-serverCh:
		log.Printf("Server error: %v", err)
	}

	// Gracefully shutdown the server
	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
		log.Printf("Server shutdown error: %v", shutdownErr)
		if err == nil {
			err = shutdownErr
		}
	}

	// Cancel context to signal all background processes to stop
	cancel()

	return err
}

// setupRoutes configures all HTTP endpoints for the server
func (s *MasterServer) setupRoutes() {
	// Root endpoint
	http.HandleFunc("/", s.handleRoot)

	// Data endpoints for cache operations
	http.HandleFunc("/data", s.handleData)
	http.HandleFunc("/data/", s.handleDataWithKey)

	// Node management endpoints
	http.HandleFunc("/nodes", s.handleNodes)
	http.HandleFunc("/nodes/", s.handleNodesWithID)

	// Health check endpoint
	http.HandleFunc("/health", s.handleHealth)

	// Rebalance endpoint
	http.HandleFunc("/rebalance", s.handleRebalance)

	// Backup endpoints
	http.HandleFunc("/backup", s.handleBackup)
	http.HandleFunc("/restore", s.handleRestore)

	// Expose Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
}

// handleRoot handles the root endpoint
func (s *MasterServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Distributed Cache System - Master Server\n")
	fmt.Fprintf(w, "Version: 1.0.0\n")
	fmt.Fprintf(w, "Uptime: %s\n", time.Since(startTime))
	fmt.Fprintf(w, "Nodes: %d\n", len(auxNodes))
}

// handleData handles PUT and POST requests to the /data endpoint
func (s *MasterServer) handleData(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues(r.Method, "/data"))
	defer timer.ObserveDuration()

	// Only allow PUT and POST methods
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use PUT or POST to store values.", http.StatusMethodNotAllowed)
		return
	}

	// Read the entire request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusBadRequest)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}
	defer r.Body.Close()
	
	// Log the size of the request body
	log.Printf("Request body size: %d bytes", len(bodyBytes))
	
	// Check if body is empty
	if len(bodyBytes) == 0 {
		log.Printf("Empty request body received")
		http.Error(w, "Request body cannot be empty", http.StatusBadRequest)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Create a copy for decoding
	bodyReader := bytes.NewReader(bodyBytes)

	// Decode request body
	var req models.CachePutRequest
	if err := json.NewDecoder(bodyReader).Decode(&req); err != nil {
		log.Printf("Failed to decode request body as JSON: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}
	
	// Log the key being processed
	log.Printf("Processing PUT request for key: %s", req.Key)

	// Validate request
	if req.Key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Get node for the key using consistent hashing
	auxNodesMutex.RLock()
	nodeCount := len(auxNodes)
	auxNodesMutex.RUnlock()

	if nodeCount == 0 {
		http.Error(w, "No auxiliary nodes available", http.StatusServiceUnavailable)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	nodeName := s.HashRing.Get(req.Key)
	if nodeName == "" {
		http.Error(w, "Failed to determine node for key", http.StatusInternalServerError)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Get node info
	auxNodesMutex.RLock()
	node, exists := auxNodes[nodeName]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Selected node not available", http.StatusInternalServerError)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Forward request to auxiliary node with a new copy of the body
	auxURL := fmt.Sprintf("http://%s:%d/data", node.Address, node.Port)
	auxReq, err := http.NewRequest(r.Method, auxURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			auxReq.Header.Add(name, value)
		}
	}

	// Set content type
	auxReq.Header.Set("Content-Type", "application/json")

	// Send request to auxiliary node
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(auxReq)
	if err != nil {
		log.Printf("Failed to forward request to auxiliary node %s:%d for key %s: %v", 
			node.Address, node.Port, req.Key, err)
		http.Error(w, "Failed to forward request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cachePutCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code and write response
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	// Update metrics
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		cachePutCounter.WithLabelValues("success").Inc()
	} else {
		cachePutCounter.WithLabelValues("error").Inc()
	}

	log.Printf("PUT request for key '%s' forwarded to node %s, status: %d", req.Key, nodeName, resp.StatusCode)
}

// handleDataWithKey handles GET and DELETE requests to the /data/{key} endpoint
func (s *MasterServer) handleDataWithKey(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues(r.Method, "/data/{key}"))
	defer timer.ObserveDuration()

	// Extract key from URL path
	key := strings.TrimPrefix(r.URL.Path, "/data/")
	if key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		if r.Method == http.MethodGet {
			cacheGetCounter.WithLabelValues("error").Inc()
		} else if r.Method == http.MethodDelete {
			cacheDeleteCounter.WithLabelValues("error").Inc()
		}
		return
	}

	// Handle based on HTTP method
	switch r.Method {
	case http.MethodGet:
		s.handleGetRequest(w, r, key)
	case http.MethodDelete:
		s.handleDeleteRequest(w, r, key)
	default:
		http.Error(w, "Method not allowed. Use GET to retrieve values or DELETE to remove them.", http.StatusMethodNotAllowed)
	}
}

// handleGetRequest handles GET requests for a specific key
func (s *MasterServer) handleGetRequest(w http.ResponseWriter, r *http.Request, key string) {
	// Get node for the key using consistent hashing
	auxNodesMutex.RLock()
	nodeCount := len(auxNodes)
	auxNodesMutex.RUnlock()

	if nodeCount == 0 {
		http.Error(w, "No auxiliary nodes available", http.StatusServiceUnavailable)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}

	nodeName := s.HashRing.Get(key)
	if nodeName == "" {
		http.Error(w, "Failed to determine node for key", http.StatusInternalServerError)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}

	// Get node info
	auxNodesMutex.RLock()
	node, exists := auxNodes[nodeName]
	auxNodesMutex.RUnlock()

	if !exists {
		http.Error(w, "Selected node not available", http.StatusInternalServerError)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}

	// Forward request to auxiliary node
	auxURL := fmt.Sprintf("http://%s:%d/data/%s", node.Address, node.Port, key)
	auxReq, err := http.NewRequest(http.MethodGet, auxURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			auxReq.Header.Add(name, value)
		}
	}

	// Send request to auxiliary node
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(auxReq)
	if err != nil {
		http.Error(w, "Failed to forward request to auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from auxiliary node: "+err.Error(), http.StatusInternalServerError)
		cacheGetCounter.WithLabelValues("error").Inc()
		return
	}

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set status code and write response
	w.WriteHeader(resp.StatusCode)
	w.Write(body)

	// Update metrics
	if resp.StatusCode == http.StatusOK {
		cacheGetCounter.WithLabelValues("success").Inc()
	} else {
		cacheGetCounter.WithLabelValues("error").Inc()
	}

	log.Printf("GET request for key '%s' forwarded to node %s, status: %d", key, nodeName, resp.StatusCode)
}

// main is the entry point for the master server
func main() {
	// Create a new master server instance
	server := NewMasterServer()

	// Start the server
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
