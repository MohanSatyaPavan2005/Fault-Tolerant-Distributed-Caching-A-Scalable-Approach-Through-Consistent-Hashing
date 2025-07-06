#!/usr/bin/env python3
"""
Distributed Cache System Load Tester

This script generates traffic to test the distributed cache system by sending
concurrent GET and PUT requests. It tracks metrics such as response time,
success rate, and throughput.

Usage:
    python cache_load_test.py [options]

Options:
    --hosts HOST1,HOST2,...  Comma-separated list of cache servers (default: localhost:8000)
    --rate RATE              Target requests per second (default: 1000)
    --get-ratio RATIO        Percentage of GET requests (default: 80)
    --key-count COUNT        Number of unique keys to use (default: 10000)
    --value-size SIZE        Size of values in bytes (default: 1024)
    --duration DURATION      Test duration in seconds, 0 for unlimited (default: 0)
    --ramp-up SECONDS        Ramp-up period in seconds (default: 30)
    --report-interval SECS   Report interval in seconds (default: 10)
"""

import argparse
import concurrent.futures
import json
import random
import requests
import string
import sys
import threading
import time
from collections import defaultdict
from datetime import datetime
from urllib.parse import urljoin

# Global stats tracking
stats_lock = threading.Lock()
stats = {
    'get': {
        'count': 0,
        'success': 0,
        'latency_sum': 0,
        'errors': defaultdict(int)
    },
    'put': {
        'count': 0,
        'success': 0,
        'latency_sum': 0,
        'errors': defaultdict(int)
    },
    'start_time': time.time(),
    'last_report_time': time.time()
}

# Rate limiting
rate_limiter = threading.Semaphore(1)
current_rate = 0
adjust_rate_lock = threading.Lock()
stop_event = threading.Event()

def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(description="Load tester for distributed cache system")
    parser.add_argument("--hosts", default="localhost:8000", 
                        help="Comma-separated list of cache servers")
    parser.add_argument("--rate", type=int, default=1000,
                        help="Target requests per second")
    parser.add_argument("--get-ratio", type=float, default=80,
                        help="Percentage of GET requests (0-100)")
    parser.add_argument("--key-count", type=int, default=10000,
                        help="Number of unique keys to use")
    parser.add_argument("--value-size", type=int, default=1024,
                        help="Size of values in bytes")
    parser.add_argument("--duration", type=int, default=0,
                        help="Test duration in seconds, 0 for unlimited")
    parser.add_argument("--ramp-up", type=int, default=30,
                        help="Ramp-up period in seconds")
    parser.add_argument("--report-interval", type=int, default=10,
                        help="Report interval in seconds")
    return parser.parse_args()

def generate_random_key(args):
    """Generate a random key."""
    return f"key_{random.randint(1, args.key_count)}"

def generate_random_value(size):
    """Generate a random string of specified size."""
    return ''.join(random.choices(string.ascii_letters + string.digits, k=size))

def get_cache_url(hosts):
    """Get random host from the host list."""
    host = random.choice(hosts.split(','))
    if not host.startswith('http'):
        host = f"http://{host}"
    return host

def update_stats(op_type, success, latency, error_code=None):
    """Update global statistics."""
    with stats_lock:
        stats[op_type]['count'] += 1
        if success:
            stats[op_type]['success'] += 1
            stats[op_type]['latency_sum'] += latency
        else:
            stats[op_type]['errors'][error_code] += 1

def perform_get(hosts, key, timeout=2.0):
    """Perform a GET request to the cache server."""
    start_time = time.time()
    success = False
    error_code = None
    
    try:
        url = urljoin(get_cache_url(hosts), f"/cache/{key}")
        response = requests.get(url, timeout=timeout)
        success = 200 <= response.status_code < 300
        if not success:
            error_code = f"HTTP_{response.status_code}"
    except requests.exceptions.RequestException as e:
        error_code = type(e).__name__
    except Exception as e:
        error_code = "UnexpectedError"
    
    latency = time.time() - start_time
    update_stats('get', success, latency, error_code)
    return success

def perform_put(hosts, key, value, timeout=2.0):
    """Perform a PUT request to the cache server."""
    start_time = time.time()
    success = False
    error_code = None
    
    try:
        url = urljoin(get_cache_url(hosts), f"/cache/{key}")
        response = requests.put(url, data=value, timeout=timeout)
        success = 200 <= response.status_code < 300
        if not success:
            error_code = f"HTTP_{response.status_code}"
    except requests.exceptions.RequestException as e:
        error_code = type(e).__name__
    except Exception as e:
        error_code = "UnexpectedError"
    
    latency = time.time() - start_time
    update_stats('put', success, latency, error_code)
    return success

def perform_operation(args):
    """Perform a single cache operation (GET or PUT)."""
    key = generate_random_key(args)
    
    # Determine operation type based on get ratio
    is_get = random.random() * 100 < args.get_ratio
    
    if is_get:
        perform_get(args.hosts, key)
    else:
        value = generate_random_value(args.value_size)
        perform_put(args.hosts, key, value)

def adjust_rate(target_rate, ramp_up_seconds):
    """Gradually adjust request rate during ramp-up period."""
    global current_rate
    start_time = time.time()
    
    while not stop_event.is_set() and (time.time() - start_time) < ramp_up_seconds:
        elapsed = time.time() - start_time
        progress = min(1.0, elapsed / ramp_up_seconds)
        with adjust_rate_lock:
            current_rate = max(1, int(target_rate * progress))
        time.sleep(1)
    
    with adjust_rate_lock:
        current_rate = target_rate

def rate_limit(rate):
    """Rate limiting mechanism."""
    if rate <= 0:
        return
    
    with adjust_rate_lock:
        current = current_rate
        
    if current <= 0:
        return
        
    delay = 1.0 / current
    rate_limiter.acquire()
    try:
        time.sleep(delay)
    finally:
        rate_limiter.release()

def print_report():
    """Print performance report."""
    with stats_lock:
        now = time.time()
        interval = now - stats['last_report_time']
        total_duration = now - stats['start_time']
        
        # Calculate metrics for this interval
        get_count = stats['get']['count']
        put_count = stats['put']['count']
        total_count = get_count + put_count
        
        get_success = stats['get']['success']
        put_success = stats['put']['success']
        total_success = get_success + put_success
        
        if get_success > 0:
            get_latency_avg = stats['get']['latency_sum'] / get_success * 1000  # ms
        else:
            get_latency_avg = 0
            
        if put_success > 0:
            put_latency_avg = stats['put']['latency_sum'] / put_success * 1000  # ms
        else:
            put_latency_avg = 0
        
        # Calculate rates
        requests_per_sec = total_count / total_duration
        success_rate = (total_success / total_count * 100) if total_count > 0 else 0
        
        # Reset stats for next interval
        stats['last_report_time'] = now
        
        # Format metrics for output
        timestamp = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        with adjust_rate_lock:
            target = current_rate
            
        print(f"\n--- Load Test Report [{timestamp}] ---")
        print(f"Duration: {total_duration:.1f} seconds")
        print(f"Target rate: {target} req/sec, Actual: {requests_per_sec:.1f} req/sec")
        print(f"GET requests: {get_count} ({get_success} successful, "
              f"{get_latency_avg:.2f}ms avg latency)")
        print(f"PUT requests: {put_count} ({put_success} successful, "
              f"{put_latency_avg:.2f}ms avg latency)")
        print(f"Overall success rate: {success_rate:.1f}%")
        
        # Print error summary if there are errors
        total_errors = total_count - total_success
        if total_errors > 0:
            print("\nError Summary:")
            for op in ['get', 'put']:
                if stats[op]['errors']:
                    print(f"  {op.upper()} errors:")
                    for error, count in stats[op]['errors'].items():
                        print(f"    {error}: {count}")

def reporting_thread(report_interval):
    """Thread that periodically prints reports."""
    while not stop_event.is_set():
        time.sleep(report_interval)
        print_report()

def run_load_test(args):
    """Run the main load test."""
    print(f"Starting load test with target rate of {args.rate} requests/sec")
    print(f"GET/PUT ratio: {args.get_ratio}/{100-args.get_ratio}")
    print(f"Press Ctrl+C to stop the test")
    
    # Start the rate adjuster thread for ramp up
    rate_thread = threading.Thread(
        target=adjust_rate, 
        args=(args.rate, args.ramp_up),
        daemon=True
    )
    rate_thread.start()
    
    # Start the reporting thread
    report_thread = threading.Thread(
        target=reporting_thread,
        args=(args.report_interval,),
        daemon=True
    )
    report_thread.start()
    
    # Seed the cache with some initial data
    print("Seeding cache with initial data...")
    with concurrent.futures.ThreadPoolExecutor(max_workers=20) as executor:
        for i in range(min(1000, args.key_count)):
            key = f"key_{i+1}"
            value = generate_random_value(args.value_size)
            executor.submit(perform_put, args.hosts, key, value)
    
    print("Starting main test...")
    start_time = time.time()
    
    try:
        # Determine end time (if duration specified)
        end_time = start_time + args.duration if args.duration > 0 else float('inf')
        
        # Main test loop
        with concurrent.futures.ThreadPoolExecutor(max_workers=100) as executor:
            while time.time() < end_time and not stop_event.is_set():
                rate_limit(args.rate)
                executor.submit(perform_operation, args)
    
    except KeyboardInterrupt:
        print("\nTest interrupted by user. Shutting down...")
    finally:
        stop_event.set()
        
    # Final report
    print("\n----- Final Test Report -----")
    print_report()

if __name__ == "__main__":
    args = parse_args()
    run_load_test(args)

