from locust import HttpUser, TaskSet, task, between
import random
import json
import logging

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


class CacheBehaviour(TaskSet):
    @task
    def put(self):
        headers = {"Content-Type": "application/json"}
        payloads = [{"key": "water", "value": "Nile"},
                    {"key": "Interest", "value": "Dance"},
                    {"key": "Escape", "value": "World"}]

        try:
            payload = random.choice(payloads)
            response = self.client.post("/cache/data", json.dumps(payload), headers)
            
            if response.status_code == 200:
                logger.info(f"PUT successful for key {payload['key']}")
            else:
                logger.error(f"PUT failed with status code {response.status_code}: {response.text}")
        except Exception as e:
            logger.error(f"Exception during PUT request: {str(e)}")

    @task
    def get(self):
        try:
            key = random.choice(["water", "Interest", "Escape"])
            response = self.client.get("/cache/data/" + key)
            
            if response.status_code == 200:
                logger.info(f"GET successful for key {key}")
            elif response.status_code == 404:
                logger.warning(f"GET key not found: {key}")
            else:
                logger.error(f"GET failed with status code {response.status_code}: {response.text}")
        except Exception as e:
            logger.error(f"Exception during GET request: {str(e)}")


class CacheLoadTest(HttpUser):
    tasks = [CacheBehaviour]
    wait_time = between(1, 2)
