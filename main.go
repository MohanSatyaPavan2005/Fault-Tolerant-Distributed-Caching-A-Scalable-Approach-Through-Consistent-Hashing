package main

import (
    "fmt"
    "log"
    "net/http"
    "os"
    "sync"
    
    "github.com/prometheus/client_golang/prometheus/promhttp"
    
    "github.com/mmspavan/distributed-cache-system/Distributed-Cache-System/pkg"
)

// Global key-value store with mutex for concurrent access
var (
    cache     = make(map[string]string)
    cacheLock = sync.RWMutex{}
)

// updateCacheMetrics updates the cache size metric
func updateCacheMetrics() {
    cacheLock.RLock()
    size := len(cache)
    cacheLock.RUnlock()
    pkg.UpdateCacheSize(size)
}

// cacheHandler handles the /cache/ endpoint for PUT and GET operations
func cacheHandler(w http.ResponseWriter, r *http.Request) {
    // Get the key from the URL query parameters
    key := r.URL.Query().Get("key")
    if key == "" {
        msg := "Missing key parameter"
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(msg))
        return
    }

    // Process based on HTTP method
    switch r.Method {
    case http.MethodPut:
        // For PUT, store the key-value pair
        value := r.URL.Query().Get("value")
        if value == "" {
            msg := "Missing value parameter for PUT request"
            w.Header().Set("Content-Type", "text/plain; charset=utf-8")
            w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
            w.Header().Set("Connection", "keep-alive")
            w.WriteHeader(http.StatusBadRequest)
            w.Write([]byte(msg))
            return
        }

        // Store the key-value pair in the cache
        cacheLock.Lock()
        cache[key] = value
        cacheLock.Unlock()
        
        // Update metrics
        pkg.RecordCacheOperation("put", "success")
        updateCacheMetrics()

        // Create response message first
        responseMsg := fmt.Sprintf("Stored key %s with value %s", key, value)
        
        // Set headers with explicit content length for HTTP/1.1 compatibility
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseMsg)))
        w.Header().Set("Connection", "keep-alive")
        
        // Write status code
        w.WriteHeader(http.StatusOK)
        
        // Write response body
        w.Write([]byte(responseMsg))

    case http.MethodGet:
        // For GET, retrieve the value for the given key
        cacheLock.RLock()
        value, exists := cache[key]
        cacheLock.RUnlock()

        // Update metrics
        if !exists {
            pkg.RecordCacheOperation("get", "miss")
            msg := fmt.Sprintf("Key not found: %s", key)
            w.Header().Set("Content-Type", "text/plain; charset=utf-8")
            w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
            w.Header().Set("Connection", "keep-alive")
            w.WriteHeader(http.StatusNotFound)
            w.Write([]byte(msg))
            return
        }
        
        pkg.RecordCacheOperation("get", "hit")

        // Create response message first
        responseMsg := fmt.Sprintf("Value for key %s: %s", key, value)
        
        // Set headers with explicit content length for HTTP/1.1 compatibility
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(responseMsg)))
        w.Header().Set("Connection", "keep-alive")
        
        // Write status code
        w.WriteHeader(http.StatusOK)
        
        // Write response body
        w.Write([]byte(responseMsg))

    default:
        msg := "Method not allowed. Use PUT to store values and GET to retrieve them."
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusMethodNotAllowed)
        w.Write([]byte(msg))
    }
}

// metricsMux returns a new ServeMux that only handles the metrics endpoint
func metricsMux() *http.ServeMux {
    mux := http.NewServeMux()
    
    // Register the Prometheus metrics handler
    mux.Handle("/metrics", promhttp.Handler())
    
    log.Printf("Metrics handler registered at /metrics")
    return mux
}

// metricsHandler wraps a metrics mux with logging for debugging
func metricsHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("METRICS ROUTER: Received request for %s", r.URL.Path)
        next.ServeHTTP(w, r)
    })
}

// Route requests either to metrics or application handlers
func routeHandler(metricsMux, appMux http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check if this is a request for metrics
        if r.URL.Path == "/metrics" {
            log.Printf("MAIN ROUTER: Routing %s to metrics handler", r.URL.Path)
            metricsMux.ServeHTTP(w, r)
            return
        }

        // Otherwise route to application handlers
        log.Printf("MAIN ROUTER: Routing %s to application handler", r.URL.Path)
        appMux.ServeHTTP(w, r)
    })
}

func main() {
    // Initialize Prometheus metrics first
    pkg.StartMetricsCollection()
    pkg.SetServerInfo("master")
    
    log.Printf("=========================================")
    log.Printf("Starting server with Prometheus metrics")
    log.Printf("=========================================")

    // Create a dedicated mux for metrics
    metrics := metricsHandler(metricsMux())
    
    // Create a separate application mux for all non-metrics routes
    appMux := http.NewServeMux()
    
    // Set up the HTTP handler for the root path
    appMux.HandleFunc("/", pkg.MeasureRequestDuration("root", func(w http.ResponseWriter, r *http.Request) {
        // Only handle the exact root path
        if r.URL.Path != "/" {
            log.Printf("ROOT HANDLER: Not found: %s", r.URL.Path)
            http.NotFound(w, r)
            return
        }

        log.Printf("ROOT HANDLER: Serving root path")
        msg := "Master Server Running"
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(msg))
    }))
    
    // Set up the cache endpoint handler on the app mux
    appMux.HandleFunc("/cache/", pkg.MeasureRequestDuration("cache", cacheHandler))
    
    // Add a health check endpoint on the app mux
    appMux.HandleFunc("/health", pkg.MeasureRequestDuration("health", func(w http.ResponseWriter, r *http.Request) {
        msg := "healthy"
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.Header().Set("Content-Length", fmt.Sprintf("%d", len(msg)))
        w.Header().Set("Connection", "keep-alive")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(msg))
    }))
    
    // Register a test metrics endpoint that doesn't use the Prometheus handler
    // This helps verify our routing logic is correct
    appMux.HandleFunc("/test-metrics", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("TEST-METRICS: Serving test metrics endpoint")
        msg := "This is a test metrics endpoint, not the real Prometheus metrics"
        w.Header().Set("Content-Type", "text/plain; charset=utf-8") 
        w.Write([]byte(msg))
    })

    // Create our main handler that routes requests to either metrics or app
    mainHandler := routeHandler(metrics, appMux)
    
    log.Printf("Server configured with dedicated /metrics endpoint for Prometheus metrics")
    log.Printf("Routes registered:")
    log.Printf("  - /metrics (Prometheus metrics)")
    log.Printf("  - / (Root endpoint)")
    log.Printf("  - /cache/ (Cache operations)")
    log.Printf("  - /health (Health check)")
    log.Printf("  - /test-metrics (Test endpoint)")

    // Get port from environment variable, default to 8000 if not set
    port := os.Getenv("PORT")
    if port == "" {
        port = "8000"
    }

    // Log which port we're using
    log.Printf("Using port %s from environment", port)

    // Start the HTTP server with our routing handler
    fmt.Printf("Master server starting on port %s...\n", port)
    serverAddr := ":" + port
    log.Printf("Listening on %s", serverAddr)
    if err := http.ListenAndServe(serverAddr, mainHandler); err != nil {
        fmt.Fprintf(os.Stderr, "Server failed: %v\n", err)
        os.Exit(1)
    }
}
