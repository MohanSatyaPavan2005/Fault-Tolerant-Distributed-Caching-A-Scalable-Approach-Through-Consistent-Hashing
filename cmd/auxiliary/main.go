package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mmspavan/distributed-cache-system/Distributed-Cache-System/internal/cache"
	"github.com/mmspavan/distributed-cache-system/Distributed-Cache-System/pkg/models"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Configuration constants with defaults
const (
	defaultPort             = "3001"
	defaultMasterAddress    = "master:8000"
	defaultCacheSize        = 1000
	defaultBackupDir        = "/tmp/cache-backup"
	defaultBackupInterval   = 10 * time.Second // 10 seconds
	defaultReadTimeout      = 30 * time.Second
	defaultWriteTimeout     = 30 * time.Second
	defaultIdleTimeout      = 120 * time.Second
	defaultShutdownTimeout  = 30 * time.Second
	defaultMaxHeaderBytes   = 1 << 20 // 1 MB
)

// Global variables
var (
	// Server start time for uptime calculation
	startTime = time.Now()

	// LRU cache for storing data
	lruCache *cache.LRUCache

	// Node identification
	nodeID   string
	nodeName string
	nodePort int

	// Path to backup directory
	backupDir string

	// Metrics
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "The total number of cache hits",
	})
	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "The total number of cache misses",
	})
	cacheOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "The total number of cache operations",
		},
		[]string{"operation", "status"},
	)
	cacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_size_current",
		Help: "The current number of items in the cache",
	})
	cacheCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_capacity_total",
		Help: "The total capacity of the cache",
	})
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)
)

// AuxiliaryServer represents the auxiliary node in the distributed cache system
type AuxiliaryServer struct {
	Port          string
	MasterAddress   string
	CacheSize       int
	BackupDir       string
	BackupInterval  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	Cache           *cache.LRUCache
}

// NewAuxiliaryServer creates a new auxiliary server with the given configuration
func NewAuxiliaryServer() *AuxiliaryServer {
	// Load configuration from environment variables
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	masterAddress := os.Getenv("MASTER_ADDRESS")
	if masterAddress == "" {
		masterAddress = defaultMasterAddress
	}

	cacheSize, err := strconv.Atoi(os.Getenv("CACHE_SIZE"))
	if err != nil || cacheSize <= 0 {
		cacheSize = defaultCacheSize
	}

	backupDirEnv := os.Getenv("BACKUP_DIR")
	if backupDirEnv == "" {
		backupDirEnv = defaultBackupDir
	}
	backupDir = backupDirEnv

	backupIntervalSec, err := strconv.Atoi(os.Getenv("BACKUP_INTERVAL"))
	var backupInterval time.Duration
	if err != nil || backupIntervalSec <= 0 {
		backupInterval = defaultBackupInterval
	} else {
		backupInterval = time.Duration(backupIntervalSec) * time.Second
	}

	// Create directories for backups
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Printf("Warning: Failed to create backup directory: %v", err)
	}

	// Initialize the LRU cache
	lruCache = cache.NewLRUCache(cacheSize)

	// Set metrics
	cacheCapacity.Set(float64(cacheSize))

	// Generate node identification
	portNum, _ := strconv.Atoi(port)
	hostname, _ := os.Hostname()
	nodeName = hostname
	if nodeName == "" {
		nodeName = "aux-" + port
	}
	nodeID = fmt.Sprintf("auxiliary-%s-%s", nodeName, port)
	nodePort = portNum

	// Log configuration
	log.Printf("Auxiliary Server Configuration:")
	log.Printf("- Port: %s", port)
	log.Printf("- Master Address: %s", masterAddress)
	log.Printf("- Cache Size: %d", cacheSize)
	log.Printf("- Backup Directory: %s", backupDir)
	log.Printf("- Backup Interval: %v", backupInterval)
	log.Printf("- Node ID: %s", nodeID)
	log.Printf("- Server Timeouts: Read=%v, Write=%v, Idle=%v", defaultReadTimeout, defaultWriteTimeout, defaultIdleTimeout)

	return &AuxiliaryServer{
		Port:           port,
		MasterAddress:  masterAddress,
		CacheSize:      cacheSize,
		BackupDir:      backupDir,
		BackupInterval: backupInterval,
		ReadTimeout:    defaultReadTimeout,
		WriteTimeout:   defaultWriteTimeout,
		IdleTimeout:    defaultIdleTimeout,
		ShutdownTimeout: defaultShutdownTimeout,
		Cache:          lruCache,
	}
}

// Start starts the auxiliary server and all its background processes
func (s *AuxiliaryServer) Start() error {
	// Set up HTTP routes
	s.setupRoutes()

	// Create a context for background processes
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Try to restore from the most recent backup if available
	s.restoreFromBackup()

	// Register with the master server
	if err := s.registerWithMaster(); err != nil {
		log.Printf("Warning: Failed to register with master server: %v", err)
	}

	// Start background backup process
	go s.runBackupProcess(ctx)

	// Create HTTP server with proper timeouts
	server := &http.Server{
		Addr:           ":" + s.Port,
		ReadTimeout:    s.ReadTimeout,
		WriteTimeout:   s.WriteTimeout,
		IdleTimeout:    s.IdleTimeout,
		MaxHeaderBytes: defaultMaxHeaderBytes,
		Handler:        s.setupMiddleware(http.DefaultServeMux),
	}

	// Channel to signal server errors
	serverCh := make(chan error, 1)

	// Start HTTP server in a goroutine
	go func() {
		log.Printf("Auxiliary server starting on port %s...", s.Port)
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

	// Create a backup before shutting down
	backupID, backupErr := s.createBackup("shutdown")
	if backupErr != nil {
		log.Printf("Warning: Failed to create shutdown backup: %v", backupErr)
	} else {
		log.Printf("Created shutdown backup: %s", backupID)
	}

	// Deregister from master before shutting down
	if deregErr := s.deregisterFromMaster(); deregErr != nil {
		log.Printf("Warning: Failed to deregister from master: %v", deregErr)
	}

	// Gracefully shutdown the server
	log.Println("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.ShutdownTimeout)
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

// setupMiddleware configures middleware for request handling
func (s *AuxiliaryServer) setupMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add request ID to context
		requestID := fmt.Sprintf("%d", time.Now().UnixNano())
		
		// Request Logger Middleware
		startTime := time.Now()
		log.Printf("[%s] Started %s %s", requestID, r.Method, r.URL.Path)
		
		// Recovery Middleware
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[%s] PANIC: %v\n%s", requestID, err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		
		// Process the request
		handler.ServeHTTP(w, r)
		
		// Log request completion
		log.Printf("[%s] Completed %s %s in %v", requestID, r.Method, r.URL.Path, time.Since(startTime))
	})
}

// setupRoutes configures all HTTP endpoints for the server
func (s *AuxiliaryServer) setupRoutes() {
	// Root endpoint
	http.HandleFunc("/", s.handleRoot)
	
	// Data endpoints for cache operations
	http.HandleFunc("/data", s.withValidation(s.handleData))
	http.HandleFunc("/data/", s.withValidation(s.handleDataWithKey))
	
	// Health check endpoint
	http.HandleFunc("/health", s.handleHealth)
	
	// Backup endpoints
	http.HandleFunc("/backup", s.withValidation(s.handleBackup))
	http.HandleFunc("/restore", s.withValidation(s.handleRestore))
	
	// Rebalance endpoint
	http.HandleFunc("/rebalance", s.withValidation(s.handleRebalance))
	
	// Expose Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())
}

// withValidation adds basic validation to handlers
func (s *AuxiliaryServer) withValidation(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate Content-Type for POST/PUT requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" && r.ContentLength > 0 {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
		}
		
		// Execute the handler
		next(w, r)
	}
}

// handleRoot handles the root endpoint
func (s *AuxiliaryServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Distributed Cache System - Auxiliary Server\n")
	fmt.Fprintf(w, "Node ID: %s\n", nodeID)
	fmt.Fprintf(w, "Version: 1.0.0\n")
	fmt.Fprintf(w, "Uptime: %s\n", time.Since(startTime))
	fmt.Fprintf(w, "Cache Size: %d/%d\n", s.Cache.Size(), s.CacheSize)
}

// handleData handles PUT and POST requests to the /data endpoint
func (s *AuxiliaryServer) handleData(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues(r.Method, "/data"))
	defer timer.ObserveDuration()

	// Only allow PUT and POST methods for storing data
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use PUT or POST to store data.", http.StatusMethodNotAllowed)
		return
	}

	// Decode request body
	var req models.CachePutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		cacheOperations.WithLabelValues("put", "error").Inc()
		return
	}

	// Validate request
	if req.Key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		cacheOperations.WithLabelValues("put", "error").Inc()
		return
	}
	
	// Prevent excessively large values
	if len(req.Value) > 10*1024*1024 { // 10 MB limit
		http.Error(w, "Value exceeds maximum allowed size (10 MB)", http.StatusRequestEntityTooLarge)
		cacheOperations.WithLabelValues("put", "error").Inc()
		return
	}

	// Store in cache
	s.Cache.Put(req.Key, req.Value)
	cacheSize.Set(float64(s.Cache.Size()))
	cacheOperations.WithLabelValues("put", "success").Inc()

	// Create response
	response := models.CachePutResponse{
		Success: true,
		Node:    nodeID,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)

	log.Printf("PUT: Stored key '%s' with value of length %d", req.Key, len(req.Value))
}

// handleDataWithKey handles GET and DELETE requests to the /data/{key} endpoint
func (s *AuxiliaryServer) handleDataWithKey(w http.ResponseWriter, r *http.Request) {
	timer := prometheus.NewTimer(requestDuration.WithLabelValues(r.Method, "/data/{key}"))
	defer timer.ObserveDuration()

	// Extract key from URL path
	key := strings.TrimPrefix(r.URL.Path, "/data/")
	if key == "" {
		http.Error(w, "Key cannot be empty", http.StatusBadRequest)
		cacheOperations.WithLabelValues("key_operation", "error").Inc()
		return
	}
	
	// Validate key length
	if len(key) > 256 {
		http.Error(w, "Key exceeds maximum allowed length (256 characters)", http.StatusBadRequest)
		cacheOperations.WithLabelValues("key_operation", "error").Inc()
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
func (s *AuxiliaryServer) handleGetRequest(w http.ResponseWriter, r *http.Request, key string) {
	// Try to get value from cache
	value, found := s.Cache.Get(key)

	// Update metrics
	if found {
		cacheHits.Inc()
		cacheOperations.WithLabelValues("get", "hit").Inc()
	} else {
		cacheMisses.Inc()
		cacheOperations.WithLabelValues("get", "miss").Inc()
		
		// Return 404 if key not found
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	// Create response
	response := models.CacheGetResponse{
		Key:     key,
		Value:   value,
		Found:   true,
		Node:    nodeID,
		Created: time.Now(), // We don't store creation time in our cache, so use current time
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("GET: Retrieved key '%s' with value of length %d", key, len(value))
}

// handleDeleteRequest handles DELETE requests for a specific key
func (s *AuxiliaryServer) handleDeleteRequest(w http.ResponseWriter, r *http.Request, key string) {
	// Remove key from cache
	removed := s.Cache.Remove(key)
	cacheSize.Set(float64(s.Cache.Size()))

	if removed {
		cacheOperations.WithLabelValues("delete", "success").Inc()
	} else {
		cacheOperations.WithLabelValues("delete", "not_found").Inc()
	}

	// Create response
	response := models.CacheDeleteResponse{
		Success: removed,
		Node:    nodeID,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if !removed {
		w.WriteHeader(http.StatusNotFound)
	}
	json.NewEncoder(w).Encode(response)

	log.Printf("DELETE: Removed key '%s': %v", key, removed)
}

// handleHealth handles the /health endpoint for health checks
func (s *AuxiliaryServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get system stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Create health check response
	response := models.HealthCheckResponse{
		Status:        models.NodeStatusUp,
		Timestamp:     time.Now(),
		Uptime:        int64(time.Since(startTime).Seconds()),
		Version:       "1.0.0",
		Load:          float64(runtime.NumGoroutine()),
		MemoryUsage:   float64(memStats.Alloc) / 1024 / 1024, // Convert to MB
		CacheSize:     s.Cache.Size(),
		CacheCapacity: s.CacheSize,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleBackup handles the /backup endpoint for creating backups
func (s *AuxiliaryServer) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode request
	var req models.BackupRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Create backup
	backupID, err := s.createBackup("manual")
	if err != nil {
		http.Error(w, "Failed to create backup: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create response
	response := models.BackupResponse{
		Success:    true,
		BackupID:   backupID,
		Timestamp:  time.Now(),
		EntryCount: s.Cache.Size(),
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("Created backup: %s with %d entries", backupID, s.Cache.Size())
}

// handleRestore handles the /restore endpoint for restoring from backups
func (s *AuxiliaryServer) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode request
	var req models.RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Restore from backup
	entryCount, err := s.restoreFromSpecificBackup(req.BackupID)
	if err != nil {
		http.Error(w, "Failed to restore from backup: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create response
	response := models.RestoreResponse{
		Success:    true,
		Timestamp:  time.Now(),
		EntryCount: entryCount,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("Restored from backup: %s with %d entries", req.BackupID, entryCount)
}

// handleRebalance handles the /rebalance endpoint for redistributing cache data
func (s *AuxiliaryServer) handleRebalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Decode request
	var req models.RebalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Start rebalancing in a background goroutine
	go s.rebalanceCache(req)

	// Create response
	response := models.RebalanceResponse{
		Success:   true,
		Timestamp: time.Now(),
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("Started rebalance operation: reason=%s", req.Reason)
}

// registerWithMaster registers this auxiliary node with the master server
func (s *AuxiliaryServer) registerWithMaster() error {
	// Create registration request
	req := models.NodeRegistrationRequest{
		Name:    nodeName,
		Address: s.getLocalIP(),
		Port:    nodePort,
		Type:    models.NodeTypeAux,
		Metadata: map[string]string{
			"started": startTime.Format(time.RFC3339),
			"version": "1.0.0",
		},
	}

	// Convert to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal registration request: %v", err)
	}

	// Send request to master with timeout
	masterURL := fmt.Sprintf("http://%s/nodes", s.MasterAddress)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(masterURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("master server returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	// Decode response
	var regResp models.NodeRegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to decode registration response: %v", err)
	}

	// Update nodeID if it was assigned by the master
	if regResp.ID != "" {
		nodeID = regResp.ID
		log.Printf("Registered with master server, assigned ID: %s", nodeID)
	} else {
		log.Printf("Registered with master server")
	}

	return nil
}

// deregisterFromMaster deregisters this auxiliary node from the master server
func (s *AuxiliaryServer) deregisterFromMaster() error {
	// Create HTTP request
	masterURL := fmt.Sprintf("http://%s/nodes/%s", s.MasterAddress, nodeID)
	req, err := http.NewRequest(http.MethodDelete, masterURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create deregistration request: %v", err)
	}

	// Send request to master
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send deregistration request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("master server returned error: %s (status: %d)", string(body), resp.StatusCode)
	}

	log.Printf("Deregistered from master server")
	return nil
}

// getLocalIP returns the service name for Docker networking
func (s *AuxiliaryServer) getLocalIP() string {
	// First check if SERVICE_NAME environment variable is set
	if serviceName := os.Getenv("SERVICE_NAME"); serviceName != "" {
		return serviceName
	}
	
	// Try to determine service name from hostname
	hostname, err := os.Hostname()
	if err == nil {
		// Docker typically sets the hostname to the container ID or name
		// Extract "aux1", "aux2", etc. from hostname if possible
		if strings.HasPrefix(hostname, "aux") {
			return hostname
		}
		
		// Check if hostname contains the port number to determine which aux node it is
		if s.Port == "3001" {
			return "aux1"
		} else if s.Port == "3002" {
			return "aux2"
		} else if s.Port == "3003" {
			return "aux3"
		}
	}
	
	// If we can't determine the exact service name, use a fallback format
	// that includes the port number to help with identification
	return "aux" + s.Port
}

// runBackupProcess periodically creates backups of the cache
func (s *AuxiliaryServer) runBackupProcess(ctx context.Context) {
	ticker := time.NewTicker(s.BackupInterval)
	defer ticker.Stop()

	log.Printf("Starting automatic backup process with interval of %v", s.BackupInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("Backup process stopping")
			return
		case <-ticker.C:
			if backupID, err := s.createBackup("auto"); err != nil {
				log.Printf("Automatic backup failed: %v", err)
			} else {
				log.Printf("Created automatic backup: %s", backupID)
			}
		}
	}
}

// createBackup creates a backup of the current cache state
func (s *AuxiliaryServer) createBackup(reason string) (string, error) {
	// Generate backup ID
	backupID := fmt.Sprintf("backup-%s-%s-%d", nodeName, reason, time.Now().UnixNano())

	// Get all cache data
	cacheData := s.Cache.GetAll()

	// Create backup data structure
	backupData := struct {
		ID        string
		Timestamp time.Time
		NodeID    string
		Reason    string
		Data      map[string]string
	}{
		ID:        backupID,
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Reason:    reason,
		Data:      cacheData,
	}

	// Marshal to JSON
	backupJSON, err := json.Marshal(backupData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal backup data: %v", err)
	}

	// Create backup file
	backupPath := filepath.Join(backupDir, backupID+".json")
	if err := os.WriteFile(backupPath, backupJSON, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %v", err)
	}

	log.Printf("Created backup file: %s with %d entries", backupPath, len(cacheData))
	return backupID, nil
}

// restoreFromBackup tries to restore the cache from the most recent backup
func (s *AuxiliaryServer) restoreFromBackup() {
	// Find the most recent backup file
	matches, err := filepath.Glob(filepath.Join(backupDir, "backup-*.json"))
	if err != nil {
		log.Printf("Error finding backup files: %v", err)
		return
	}

	if len(matches) == 0 {
		log.Printf("No backup files found in %s", backupDir)
		return
	}

	// Find the most recent backup file
	var mostRecent string
	var mostRecentTime int64
	for _, file := range matches {
		// Parse timestamp from filename
		parts := strings.Split(filepath.Base(file), "-")
		if len(parts) >= 4 {
			timestamp, err := strconv.ParseInt(parts[len(parts)-1][:len(parts[len(parts)-1])-5], 10, 64)
			if err == nil && timestamp > mostRecentTime {
				mostRecent = file
				mostRecentTime = timestamp
			}
		}
	}

	if mostRecent == "" {
		log.Printf("No valid backup files found in %s", backupDir)
		return
	}

	// Extract backup ID from filename
	backupID := strings.TrimSuffix(filepath.Base(mostRecent), ".json")

	// Restore from this backup
	entries, err := s.restoreFromSpecificBackup(backupID)
	if err != nil {
		log.Printf("Failed to restore from backup %s: %v", backupID, err)
		return
	}

	log.Printf("Restored %d entries from backup: %s", entries, backupID)
}

// restoreFromSpecificBackup restores the cache from a specific backup
func (s *AuxiliaryServer) restoreFromSpecificBackup(backupID string) (int, error) {
	// Find the backup file
	backupPath := filepath.Join(backupDir, backupID+".json")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("backup file not found: %s", backupPath)
	}

	// Read the backup file
	backupJSON, err := os.ReadFile(backupPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read backup file: %v", err)
	}

	// Unmarshal backup data
	var backupData struct {
		ID        string
		Timestamp time.Time
		NodeID    string
		Reason    string
		Data      map[string]string
	}
	if err := json.Unmarshal(backupJSON, &backupData); err != nil {
		return 0, fmt.Errorf("failed to unmarshal backup data: %v", err)
	}

	// Clear the cache
	s.Cache.Clear()

	// Restore data to cache
	for key, value := range backupData.Data {
		s.Cache.Put(key, value)
	}

	// Update metrics
	cacheSize.Set(float64(s.Cache.Size()))

	log.Printf("Restored %d entries from backup: %s", len(backupData.Data), backupID)
	return len(backupData.Data), nil
}

// rebalanceCache redistributes cache data based on a rebalance request
func (s *AuxiliaryServer) rebalanceCache(req models.RebalanceRequest) {
	log.Printf("Rebalancing cache: reason=%s, nodeID=%s", req.Reason, req.NodeID)

	// Get nodes list from master with timeout
	masterURL := fmt.Sprintf("http://%s/nodes", s.MasterAddress)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(masterURL)
	if err != nil {
		log.Printf("Failed to get nodes list for rebalancing: %v", err)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Master server returned error when getting nodes: %s (status: %d)", string(body), resp.StatusCode)
		return
	}

	// Decode response
	var nodesResp models.NodesListResponse
	if err := json.NewDecoder(resp.Body).Decode(&nodesResp); err != nil {
		log.Printf("Failed to decode nodes response: %v", err)
		return
	}

	// Check if we have nodes
	if len(nodesResp.Nodes) == 0 {
		log.Printf("No nodes returned from master for rebalancing")
		return
	}

	// Get cache data for potential redistribution
	cacheData := s.Cache.GetAll()
	if len(cacheData) == 0 {
		log.Printf("No cache data to rebalance")
		return
	}

	// Create a map of node IDs to their info
	nodeMap := make(map[string]models.NodeInfo)
	for _, node := range nodesResp.Nodes {
		if node.Status == models.NodeStatusUp && node.Type == models.NodeTypeAux {
			nodeMap[node.ID] = node
		}
	}

	// Create a simple hash function to decide which node should own which key
	// This simulates the consistent hashing without implementing the full algorithm again
	getNodeForKey := func(key string) string {
		// Simple hash function: sum of character codes modulo number of healthy nodes
		sum := 0
		for _, c := range key {
			sum += int(c)
		}
		
		// Get list of node IDs
		nodeIDs := make([]string, 0, len(nodeMap))
		for id := range nodeMap {
			nodeIDs = append(nodeIDs, id)
		}
		
		// If there are no nodes, return empty string
		if len(nodeIDs) == 0 {
			return ""
		}
		
		// Select node based on hash
		return nodeIDs[sum%len(nodeIDs)]
	}

	// Track keys to remove and stats
	keysToRemove := make([]string, 0)
	movedCount := 0
	errorCount := 0

	// For each key, check if it should be on this node
	for key, value := range cacheData {
		targetNodeID := getNodeForKey(key)
		
		// If this key belongs to another node, move it there
		if targetNodeID != "" && targetNodeID != nodeID {
			targetNode, exists := nodeMap[targetNodeID]
			if exists {
				// Create PUT request to move key to target node
				putReq := models.CachePutRequest{
					Key:   key,
					Value: value,
				}
				
				// Convert to JSON
				reqBody, err := json.Marshal(putReq)
				if err != nil {
					log.Printf("Failed to marshal PUT request for key %s: %v", key, err)
					errorCount++
					continue
				}
				
				// Send request to target node
				targetURL := fmt.Sprintf("http://%s:%d/data", targetNode.Address, targetNode.Port)
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Post(targetURL, "application/json", bytes.NewBuffer(reqBody))
				if err != nil {
					log.Printf("Failed to send PUT request for key %s to node %s: %v", key, targetNodeID, err)
					errorCount++
					continue
				}
				defer resp.Body.Close()
				
				// Check response status
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
					body, _ := io.ReadAll(resp.Body)
					log.Printf("Target node returned error for key %s: %s (status: %d)", key, string(body), resp.StatusCode)
					errorCount++
					continue
				}
				
				// Key was successfully moved, we'll remove it from this node
				keysToRemove = append(keysToRemove, key)
				movedCount++
			}
		}
	}

	// Remove keys that were moved to other nodes
	for _, key := range keysToRemove {
		s.Cache.Remove(key)
	}

	// Update metrics
	cacheSize.Set(float64(s.Cache.Size()))

	log.Printf("Rebalance completed: %d keys moved, %d errors", movedCount, errorCount)
}

// main function to start the auxiliary server
func main() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting auxiliary server...")

	// Create and start the server
	server := NewAuxiliaryServer()
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

