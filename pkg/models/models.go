package models

import (
	"time"
)

// ===== Cache Operation Models =====

// CacheEntry represents a key-value pair in the cache
type CacheEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	TTL       int64     `json:"ttl,omitempty"` // Time-to-live in seconds, 0 means no expiration
}

// CachePutRequest represents a request to put a key-value pair in the cache
type CachePutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl,omitempty"` // Time-to-live in seconds, 0 means no expiration
}

// CachePutResponse represents a response from putting a key-value pair in the cache
type CachePutResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Node    string `json:"node,omitempty"` // The node that handled the request
}

// CacheGetRequest represents a request to get a value from the cache
type CacheGetRequest struct {
	Key string `json:"key"`
}

// CacheGetResponse represents a response from getting a value from the cache
type CacheGetResponse struct {
	Key     string    `json:"key"`
	Value   string    `json:"value,omitempty"`
	Found   bool      `json:"found"`
	Error   string    `json:"error,omitempty"`
	Node    string    `json:"node,omitempty"` // The node that handled the request
	Created time.Time `json:"created,omitempty"`
}

// CacheDeleteRequest represents a request to delete a key-value pair from the cache
type CacheDeleteRequest struct {
	Key string `json:"key"`
}

// CacheDeleteResponse represents a response from deleting a key-value pair from the cache
type CacheDeleteResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Node    string `json:"node,omitempty"` // The node that handled the request
}

// CacheClearRequest represents a request to clear all key-value pairs from the cache
type CacheClearRequest struct {
	Node string `json:"node,omitempty"` // Optional - if specified, only clear this node
}

// CacheClearResponse represents a response from clearing all key-value pairs from the cache
type CacheClearResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Count   int    `json:"count"` // Number of entries cleared
}

// ===== Node Information Models =====

// NodeInfo represents information about a node in the system
type NodeInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Type      NodeType  `json:"type"`
	Status    NodeStatus `json:"status"`
	Timestamp time.Time `json:"timestamp"` // Last updated timestamp
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// NodeType represents the type of a node
type NodeType string

const (
	NodeTypeMaster NodeType = "master"
	NodeTypeAux    NodeType = "auxiliary"
)

// NodeStatus represents the status of a node
type NodeStatus string

const (
	NodeStatusUp      NodeStatus = "up"
	NodeStatusDown    NodeStatus = "down"
	NodeStatusUnknown NodeStatus = "unknown"
)

// NodeRegistrationRequest represents a request to register a node in the system
type NodeRegistrationRequest struct {
	Name    string            `json:"name"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Type    NodeType          `json:"type"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeRegistrationResponse represents a response from registering a node in the system
type NodeRegistrationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	ID      string `json:"id,omitempty"`
}

// NodesListRequest represents a request to list all nodes in the system
type NodesListRequest struct {
	Type NodeType `json:"type,omitempty"` // Optional - if specified, only return nodes of this type
}

// NodesListResponse represents a response from listing all nodes in the system
type NodesListResponse struct {
	Success bool       `json:"success"`
	Error   string     `json:"error,omitempty"`
	Nodes   []NodeInfo `json:"nodes"`
}

// NodeDeregistrationRequest represents a request to deregister a node from the system
type NodeDeregistrationRequest struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// NodeDeregistrationResponse represents a response from deregistering a node from the system
type NodeDeregistrationResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ===== Health Check Models =====

// HealthCheckRequest represents a request to check the health of a node
type HealthCheckRequest struct {
	Timestamp time.Time `json:"timestamp"`
}

// HealthCheckResponse represents a response from checking the health of a node
type HealthCheckResponse struct {
	Status    NodeStatus `json:"status"`
	Timestamp time.Time  `json:"timestamp"`
	Uptime    int64      `json:"uptime"` // Uptime in seconds
	Version   string     `json:"version"`
	Load      float64    `json:"load,omitempty"`
	MemoryUsage float64  `json:"memory_usage,omitempty"` // Memory usage in MB
	CacheSize   int      `json:"cache_size,omitempty"`
	CacheCapacity int    `json:"cache_capacity,omitempty"`
}

// ===== Metrics Models =====

// MetricsData represents metrics data for a node
type MetricsData struct {
	NodeID           string    `json:"node_id"`
	Timestamp        time.Time `json:"timestamp"`
	RequestsReceived int64     `json:"requests_received"`
	RequestsHandled  int64     `json:"requests_handled"`
	CacheHits        int64     `json:"cache_hits"`
	CacheMisses      int64     `json:"cache_misses"`
	AvgResponseTime  float64   `json:"avg_response_time"` // in milliseconds
	MemoryUsage      float64   `json:"memory_usage"`      // in MB
	CacheSize        int       `json:"cache_size"`
	ErrorCount       int64     `json:"error_count"`
}

// ===== Rebalance Models =====

// RebalanceRequest represents a request to rebalance the cache distribution
type RebalanceRequest struct {
	Reason    RebalanceReason `json:"reason"`
	NodeID    string          `json:"node_id,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// RebalanceReason represents the reason for rebalancing
type RebalanceReason string

const (
	RebalanceReasonNodeAdded   RebalanceReason = "node_added"
	RebalanceReasonNodeRemoved RebalanceReason = "node_removed"
	RebalanceReasonNodeFailed  RebalanceReason = "node_failed"
	RebalanceReasonManual      RebalanceReason = "manual"
)

// RebalanceResponse represents a response from rebalancing the cache distribution
type RebalanceResponse struct {
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	EntriesMoved int       `json:"entries_moved"`
	Timestamp    time.Time `json:"timestamp"`
}

// ===== Backup Models =====

// BackupRequest represents a request to create a backup of the cache
type BackupRequest struct {
	NodeID    string    `json:"node_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// BackupResponse represents a response from creating a backup of the cache
type BackupResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	BackupID  string    `json:"backup_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	EntryCount int      `json:"entry_count"`
	Size      int64     `json:"size"` // Size in bytes
}

// RestoreRequest represents a request to restore a cache from a backup
type RestoreRequest struct {
	BackupID  string    `json:"backup_id"`
	NodeID    string    `json:"node_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// RestoreResponse represents a response from restoring a cache from a backup
type RestoreResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	EntryCount int      `json:"entry_count"`
}

// ===== Error Types =====

// ErrorCode represents an error code for the distributed cache system
type ErrorCode string

const (
	// General errors
	ErrorNone                    ErrorCode = ""
	ErrorInternal                ErrorCode = "internal_error"
	ErrorInvalidRequest          ErrorCode = "invalid_request"
	ErrorTimeout                 ErrorCode = "timeout"
	ErrorUnavailable             ErrorCode = "service_unavailable"
	
	// Cache operation errors
	ErrorKeyNotFound             ErrorCode = "key_not_found"
	ErrorKeyAlreadyExists        ErrorCode = "key_already_exists"
	ErrorValueTooLarge           ErrorCode = "value_too_large"
	ErrorCacheFull               ErrorCode = "cache_full"
	
	// Node errors
	ErrorNodeNotFound            ErrorCode = "node_not_found"
	ErrorNodeAlreadyRegistered   ErrorCode = "node_already_registered"
	ErrorNodeNotRegistered       ErrorCode = "node_not_registered"
	ErrorNodeUnreachable         ErrorCode = "node_unreachable"
	
	// Rebalance errors
	ErrorRebalanceInProgress     ErrorCode = "rebalance_in_progress"
	ErrorRebalanceFailed         ErrorCode = "rebalance_failed"
	
	// Backup/restore errors
	ErrorBackupFailed            ErrorCode = "backup_failed"
	ErrorRestoreFailed           ErrorCode = "restore_failed"
	ErrorBackupNotFound          ErrorCode = "backup_not_found"
)

// SystemError represents a system error with code and message
type SystemError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// NewSystemError creates a new system error with the given code and message
func NewSystemError(code ErrorCode, message string) *SystemError {
	return &SystemError{
		Code:    code,
		Message: message,
	}
}

// Error returns the error message
func (e *SystemError) Error() string {
	return e.Message
}

// ===== Configuration Models =====

// SystemConfig represents the configuration for the distributed cache system
type SystemConfig struct {
	// Master node configuration
	MasterPort int `json:"master_port"`
	MasterCount int `json:"master_count"`
	
	// Auxiliary node configuration
	AuxPort int `json:"aux_port"`
	AuxCount int `json:"aux_count"`
	
	// Cache configuration
	CacheSize int `json:"cache_size"`
	DefaultTTL int64 `json:"default_ttl"`
	
	// Consistent hashing configuration
	ReplicationFactor int `json:"replication_factor"`
	
	// Backup configuration
	BackupInterval int `json:"backup_interval"` // in seconds
	BackupEnabled bool `json:"backup_enabled"`
	BackupDir string `json:"backup_dir"`
	
	// Health check configuration
	HealthCheckInterval int `json:"health_check_interval"` // in seconds
	HealthCheckTimeout int `json:"health_check_timeout"` // in seconds
	
	// Monitoring configuration
	MetricsEnabled bool `json:"metrics_enabled"`
	MetricsPort int `json:"metrics_port"`
	
	// Load balancer configuration
	LoadBalancerEnabled bool `json:"load_balancer_enabled"`
	LoadBalancerPort int `json:"load_balancer_port"`
}

