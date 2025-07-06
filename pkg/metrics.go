package pkg

import (
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CacheOperations counts the number of cache operations by type (get, put, delete)
	CacheOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_operations_total",
		Help: "The total number of cache operations by type",
	}, []string{"operation", "status"})

	// CacheSize measures the current number of items in the cache
	CacheSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_size",
		Help: "The current number of items in the cache",
	})

	// CacheMemoryUsage measures the current memory usage of the application
	CacheMemoryUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cache_memory_bytes",
		Help: "The current memory usage of the cache in bytes",
	})

	// RequestDuration measures the duration of HTTP requests
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "request_duration_seconds",
		Help:    "The duration of HTTP requests in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"handler", "method", "status"})

	// ServerInfo provides information about the server
	ServerInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "server_info",
		Help: "Information about the server",
	}, []string{"server_type"})
)

// RecordCacheOperation increments the counter for a specific cache operation
func RecordCacheOperation(operation, status string) {
	CacheOperations.WithLabelValues(operation, status).Inc()
	log.Printf("Recorded cache operation: %s, status: %s", operation, status)
}

// UpdateCacheSize updates the gauge with the current number of items in the cache
func UpdateCacheSize(size int) {
	CacheSize.Set(float64(size))
	log.Printf("Updated cache size metric: %d items", size)
}

// UpdateMemoryUsage updates the gauge with the current memory usage
func UpdateMemoryUsage() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	CacheMemoryUsage.Set(float64(memStats.Alloc))
}

// SetServerInfo sets information about the server type
func SetServerInfo(serverType string) {
	ServerInfo.WithLabelValues(serverType).Set(1)
	log.Printf("Set server info: type=%s", serverType)
}

// MeasureRequestDuration is middleware that measures the duration of HTTP requests
func MeasureRequestDuration(handler string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture the status code
		rw := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default to 200 OK
		}
		
		// Call the next handler
		next(rw, r)
		
		// Record the duration with the appropriate labels
		duration := time.Since(start).Seconds()
		RequestDuration.WithLabelValues(
			handler,
			r.Method,
			http.StatusText(rw.statusCode),
		).Observe(duration)
		
		log.Printf("Request to %s (%s) completed in %.2fs with status %d", 
			handler, r.Method, duration, rw.statusCode)
	}
}

// StartMetricsCollection initializes background metrics collection
func StartMetricsCollection() {
	log.Println("Starting metrics collection")
	
	// Update memory usage periodically
	go func() {
		for {
			UpdateMemoryUsage()
			time.Sleep(15 * time.Second)
		}
	}()
}

// RegisterMetricsHandler is deprecated and now a no-op since metrics registration
// is handled directly in main.go
func RegisterMetricsHandler() {
	log.Println("Metrics handler registration is now done directly in main.go")
}

// responseWriterWrapper is a wrapper for http.ResponseWriter that captures the status code
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before calling the wrapped WriteHeader
func (rww *responseWriterWrapper) WriteHeader(statusCode int) {
	rww.statusCode = statusCode
	rww.ResponseWriter.WriteHeader(statusCode)
}

// Write captures writes to the response writer
func (rww *responseWriterWrapper) Write(b []byte) (int, error) {
	// If WriteHeader hasn't been called yet, assume 200 OK
	if rww.statusCode == 0 {
		rww.statusCode = http.StatusOK
	}
	return rww.ResponseWriter.Write(b)
}

