package hashing

import (
	"crypto/md5"
	"encoding/binary"
	"sort"
	"strconv"
	"sync"
)

// ConsistentHash represents a consistent hashing ring with virtual node support
type ConsistentHash struct {
	ring           map[uint32]string // Maps hash values to node names
	sortedHashes   []uint32          // Sorted list of hash values for binary search
	nodes          map[string]bool   // Set of node names
	replicationFactor int            // Number of virtual nodes per physical node
	mutex          sync.RWMutex      // For thread safety
}

// NewConsistentHash creates a new consistent hash ring with the given replication factor
func NewConsistentHash(replicationFactor int) *ConsistentHash {
	if replicationFactor < 1 {
		replicationFactor = 10 // Default to 10 virtual nodes per physical node
	}
	
	return &ConsistentHash{
		ring:              make(map[uint32]string),
		sortedHashes:      []uint32{},
		nodes:             make(map[string]bool),
		replicationFactor: replicationFactor,
	}
}

// Add adds a node to the hash ring
func (c *ConsistentHash) Add(node string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if the node already exists
	if _, exists := c.nodes[node]; exists {
		return
	}
	
	// Add the node to the set of nodes
	c.nodes[node] = true
	
	// Add virtual nodes to the ring
	for i := 0; i < c.replicationFactor; i++ {
		virtualNode := node + "-" + strconv.Itoa(i)
		hash := c.hashKey(virtualNode)
		c.ring[hash] = node
		c.sortedHashes = append(c.sortedHashes, hash)
	}
	
	// Sort hashes for binary search
	sort.Slice(c.sortedHashes, func(i, j int) bool {
		return c.sortedHashes[i] < c.sortedHashes[j]
	})
}

// Remove removes a node and its virtual nodes from the hash ring
func (c *ConsistentHash) Remove(node string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if the node exists
	if _, exists := c.nodes[node]; !exists {
		return
	}
	
	// Remove the node from the set of nodes
	delete(c.nodes, node)
	
	// Remove virtual nodes from the ring
	var newSortedHashes []uint32
	for i := 0; i < c.replicationFactor; i++ {
		virtualNode := node + "-" + strconv.Itoa(i)
		hash := c.hashKey(virtualNode)
		delete(c.ring, hash)
		
		// Rebuild sorted hashes without the removed node's hashes
		for _, h := range c.sortedHashes {
			if h != hash {
				newSortedHashes = append(newSortedHashes, h)
			}
		}
	}
	
	c.sortedHashes = newSortedHashes
}

// Get returns the node that should handle the given key
func (c *ConsistentHash) Get(key string) string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	if len(c.ring) == 0 {
		return ""
	}
	
	// Hash the key
	hash := c.hashKey(key)
	
	// Find the first node with a hash greater than or equal to the key's hash
	idx := sort.Search(len(c.sortedHashes), func(i int) bool {
		return c.sortedHashes[i] >= hash
	})
	
	// If we reached the end of the ring, wrap around to the first node
	if idx == len(c.sortedHashes) {
		idx = 0
	}
	
	return c.ring[c.sortedHashes[idx]]
}

// GetNodes returns all nodes in the hash ring
func (c *ConsistentHash) GetNodes() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	nodes := make([]string, 0, len(c.nodes))
	for node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNodesCount returns the number of physical nodes in the hash ring
func (c *ConsistentHash) GetNodesCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return len(c.nodes)
}

// hashKey generates a uint32 hash of the given key using MD5
func (c *ConsistentHash) hashKey(key string) uint32 {
	hasher := md5.New()
	hasher.Write([]byte(key))
	hash := hasher.Sum(nil)
	
	// Use the first 4 bytes of the MD5 hash to get a uint32
	return binary.BigEndian.Uint32(hash[:4])
}

// Rebalance rebuilds the hash ring with the current set of nodes
// This can be useful after changing the replication factor
func (c *ConsistentHash) Rebalance() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Save the current nodes
	nodes := make([]string, 0, len(c.nodes))
	for node := range c.nodes {
		nodes = append(nodes, node)
	}
	
	// Clear the ring
	c.ring = make(map[uint32]string)
	c.sortedHashes = []uint32{}
	c.nodes = make(map[string]bool)
	
	// Re-add all nodes
	for _, node := range nodes {
		c.nodes[node] = true
		for i := 0; i < c.replicationFactor; i++ {
			virtualNode := node + "-" + strconv.Itoa(i)
			hash := c.hashKey(virtualNode)
			c.ring[hash] = node
			c.sortedHashes = append(c.sortedHashes, hash)
		}
	}
	
	// Sort hashes for binary search
	sort.Slice(c.sortedHashes, func(i, j int) bool {
		return c.sortedHashes[i] < c.sortedHashes[j]
	})
}

// SetReplicationFactor changes the replication factor and rebalances the ring
func (c *ConsistentHash) SetReplicationFactor(replicationFactor int) {
	if replicationFactor < 1 {
		replicationFactor = 10 // Default to 10 virtual nodes per physical node
	}
	
	c.mutex.Lock()
	c.replicationFactor = replicationFactor
	c.mutex.Unlock()
	
	c.Rebalance()
}

