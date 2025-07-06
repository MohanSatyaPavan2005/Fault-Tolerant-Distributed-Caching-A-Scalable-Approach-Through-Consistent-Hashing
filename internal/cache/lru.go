package cache

import (
	"sync"
)

// Node represents a node in the doubly linked list
type Node struct {
	Key   string
	Value string
	Prev  *Node
	Next  *Node
}

// LRUCache represents a thread-safe LRU cache with configurable capacity
type LRUCache struct {
	Capacity int
	Cache    map[string]*Node
	Head     *Node // Most recently used
	Tail     *Node // Least recently used
	Mutex    sync.RWMutex
}

// NewLRUCache creates a new LRUCache with the given capacity
func NewLRUCache(capacity int) *LRUCache {
	cache := &LRUCache{
		Capacity: capacity,
		Cache:    make(map[string]*Node),
		Head:     nil,
		Tail:     nil,
	}
	return cache
}

// Get retrieves a value from the cache by key
// If the key exists, it moves the node to the front (most recently used)
// Returns the value and true if found, empty string and false otherwise
func (c *LRUCache) Get(key string) (string, bool) {
	c.Mutex.RLock()
	node, exists := c.Cache[key]
	c.Mutex.RUnlock()

	if !exists {
		return "", false
	}

	// Move the accessed node to the front (most recently used)
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// Check again in case the node was removed between the read lock and write lock
	if _, exists := c.Cache[key]; !exists {
		return "", false
	}

	c.moveToFront(node)
	return node.Value, true
}

// Put adds or updates a key-value pair in the cache
// If the key already exists, it updates the value and moves the node to the front
// If the cache is at capacity, it removes the least recently used item
func (c *LRUCache) Put(key, value string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// If key already exists, update its value and move to front
	if node, exists := c.Cache[key]; exists {
		node.Value = value
		c.moveToFront(node)
		return
	}

	// Create a new node
	newNode := &Node{
		Key:   key,
		Value: value,
		Prev:  nil,
		Next:  nil,
	}

	// Add to cache map
	c.Cache[key] = newNode

	// If this is the first node
	if c.Head == nil {
		c.Head = newNode
		c.Tail = newNode
	} else {
		// Add to the front of the list (most recently used)
		newNode.Next = c.Head
		c.Head.Prev = newNode
		c.Head = newNode
	}

	// If we've exceeded capacity, remove the least recently used item (tail)
	if len(c.Cache) > c.Capacity {
		c.removeLRU()
	}
}

// Remove removes a key-value pair from the cache
// Returns true if the key was found and removed, false otherwise
func (c *LRUCache) Remove(key string) bool {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	node, exists := c.Cache[key]
	if !exists {
		return false
	}

	// Remove from the doubly linked list
	c.removeNode(node)
	
	// Remove from the cache map
	delete(c.Cache, key)
	
	return true
}

// Size returns the current size of the cache
func (c *LRUCache) Size() int {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	return len(c.Cache)
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	
	c.Cache = make(map[string]*Node)
	c.Head = nil
	c.Tail = nil
}

// GetAll returns all key-value pairs in the cache
func (c *LRUCache) GetAll() map[string]string {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()
	
	result := make(map[string]string)
	for k, node := range c.Cache {
		result[k] = node.Value
	}
	return result
}

// Internal helper methods

// moveToFront moves a node to the front of the list (most recently used)
func (c *LRUCache) moveToFront(node *Node) {
	// If node is already at the front, nothing to do
	if node == c.Head {
		return
	}

	// Remove the node from its current position
	c.removeNode(node)

	// Add to the front of the list
	node.Next = c.Head
	node.Prev = nil
	if c.Head != nil {
		c.Head.Prev = node
	}
	c.Head = node

	// If the list was empty before, this node is also the tail
	if c.Tail == nil {
		c.Tail = node
	}
}

// removeNode removes a node from the linked list without deleting from the map
func (c *LRUCache) removeNode(node *Node) {
	// If node is the head
	if node == c.Head {
		c.Head = node.Next
	}

	// If node is the tail
	if node == c.Tail {
		c.Tail = node.Prev
	}

	// Update adjacent nodes
	if node.Prev != nil {
		node.Prev.Next = node.Next
	}
	if node.Next != nil {
		node.Next.Prev = node.Prev
	}
}

// removeLRU removes the least recently used item (tail of the list)
func (c *LRUCache) removeLRU() {
	if c.Tail == nil {
		return
	}

	// Remove from cache map
	delete(c.Cache, c.Tail.Key)

	// Update tail reference
	c.Tail = c.Tail.Prev

	// If tail exists, update its next pointer
	if c.Tail != nil {
		c.Tail.Next = nil
	} else {
		// If tail is nil, the list is now empty
		c.Head = nil
	}
}

