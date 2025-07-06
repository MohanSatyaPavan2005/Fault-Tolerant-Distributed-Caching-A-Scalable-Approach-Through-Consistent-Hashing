import json
import random
import string
import time
import uuid
from locust import HttpUser, task, between, events
from typing import Dict, List, Optional

# Configuration
CONFIG = {
    "key_pattern": "user_{}", # Format for keys
    "small_value_size": 100,  # Size in bytes for small values
    "medium_value_size": 1024, # Size in bytes for medium values
    "large_value_size": 10240, # Size in bytes for large values
    "key_count": 1000,        # Number of distinct keys to use
    "value_sizes": {          # Distribution of value sizes
        "small": 0.7,         # 70% small values
        "medium": 0.2,        # 20% medium values
        "large": 0.1          # 10% large values
    },
    "operation_weights": {    # Weights for different operations
        "get": 80,            # 80% GET operations
        "put": 15,            # 15% PUT operations
        "delete": 5           # 5% DELETE operations
    }
}

# Utility functions
def generate_random_string(length: int) -> str:
    """Generate a random string of specified length."""
    return ''.join(random.choice(string.ascii_letters + string.digits) for _ in range(length))

def generate_value(size: int) -> Dict[str, str]:
    """Generate a JSON value of approximately the specified size."""
    # Create a base structure
    data = {
        "id": str(uuid.uuid4()),
        "timestamp": time.time(),
        "data": generate_random_string(max(0, size - 100))  # Adjust for overhead
    }
    return data

# Cache User class
class CacheUser(HttpUser):
    wait_time = between(0.1, 1.0)  # Wait between 0.1 and 1 second between tasks
    
    def on_start(self):
        """Initialize user session."""
        # Pre-generate keys
        self.keys = [CONFIG["key_pattern"].format(i) for i in range(CONFIG["key_count"])]
        
        # Pre-generate some values for different sizes
        self.values = {
            "small": [generate_value(CONFIG["small_value_size"]) for _ in range(10)],
            "medium": [generate_value(CONFIG["medium_value_size"]) for _ in range(5)],
            "large": [generate_value(CONFIG["large_value_size"]) for _ in range(2)]
        }
        
        # Keep track of keys we know exist in the cache
        self.existing_keys = set()
        
        # Seed the cache with some initial data
        self._seed_cache()
    
    def _seed_cache(self):
        """Seed the cache with some initial data."""
        # Seed 10% of keys to ensure some hits
        seed_count = int(CONFIG["key_count"] * 0.1)
        for i in range(seed_count):
            key = random.choice(self.keys)
            size = self._select_value_size()
            value = random.choice(self.values[size])
            
            response = self.client.post(
                "/data",
                json={"key": key, "value": json.dumps(value)},
                headers={"Content-Type": "application/json"}
            )
            
            if response.status_code in (200, 201):
                self.existing_keys.add(key)
    
    def _select_value_size(self) -> str:
        """Select a value size based on the configured distribution."""
        r = random.random()
        if r < CONFIG["value_sizes"]["small"]:
            return "small"
        elif r < CONFIG["value_sizes"]["small"] + CONFIG["value_sizes"]["medium"]:
            return "medium"
        else:
            return "large"
    
    def _select_operation(self) -> str:
        """Select an operation based on the configured weights."""
        r = random.random() * 100
        if r < CONFIG["operation_weights"]["get"]:
            return "get"
        elif r < CONFIG["operation_weights"]["get"] + CONFIG["operation_weights"]["put"]:
            return "put"
        else:
            return "delete"
    
    def _select_key(self, for_get: bool = False) -> str:
        """Select a key, optionally biasing towards existing keys for GET."""
        if for_get and self.existing_keys and random.random() < 0.8:
            # 80% of GET requests should be for existing keys (if we know any)
            return random.choice(list(self.existing_keys))
        return random.choice(self.keys)
    
    @task
    def perform_operation(self):
        """Perform a random cache operation based on configured weights."""
        operation = self._select_operation()
        
        if operation == "get":
            self.get_value()
        elif operation == "put":
            self.put_value()
        else:
            self.delete_value()
    
    def get_value(self):
        """Perform a GET operation."""
        key = self._select_key(for_get=True)
        start_time = time.time()
        
        response = self.client.get(f"/data/{key}")
        
        # If we hit an existing key, it should be found
        if key in self.existing_keys and response.status_code != 200:
            self.existing_keys.remove(key)
        
        # Log custom metrics
        value_size = 0
        if response.status_code == 200:
            try:
                value_size = len(response.text)
            except:
                pass
        
        # Report metrics
        events.request.fire(
            request_type="GET",
            name="get_value",
            response_time=(time.time() - start_time) * 1000,  # Convert to ms
            response_length=value_size,
            response=response
        )
    
    def put_value(self):
        """Perform a PUT operation."""
        key = self._select_key()
        size = self._select_value_size()
        value = random.choice(self.values[size])
        
        start_time = time.time()
        
        response = self.client.post(
            "/data",
            json={"key": key, "value": json.dumps(value)},
            headers={"Content-Type": "application/json"}
        )
        
        if response.status_code in (200, 201):
            self.existing_keys.add(key)
        
        # Report metrics
        events.request.fire(
            request_type="PUT",
            name=f"put_value_{size}",
            response_time=(time.time() - start_time) * 1000,  # Convert to ms
            response_length=len(json.dumps(value)),
            response=response
        )
    
    def delete_value(self):
        """Perform a DELETE operation."""
        # Prefer to delete existing keys if we know any
        if self.existing_keys and random.random() < 0.9:
            key = random.choice(list(self.existing_keys))
        else:
            key = self._select_key()
        
        start_time = time.time()
        
        response = self.client.delete(f"/data/{key}")
        
        if response.status_code in (200, 204) and key in self.existing_keys:
            self.existing_keys.remove(key)
        
        # Report metrics
        events.request.fire(
            request_type="DELETE",
            name="delete_value",
            response_time=(time.time() - start_time) * 1000,  # Convert to ms
            response_length=0,
            response=response
        )

