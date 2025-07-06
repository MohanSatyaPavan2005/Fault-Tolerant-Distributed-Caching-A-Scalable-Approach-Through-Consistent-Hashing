# Prometheus Queries for Distributed Cache System

This document contains useful Prometheus queries (PromQL) for monitoring the distributed cache system. These queries can be used in Prometheus UI, Grafana dashboards, or alerts.

## Basic Cache Operation Metrics

### Total Cache Operations

```promql
sum(cache_operations_total)
```
*Measures the total number of cache operations (all types combined).*

### Cache Operations by Type

```promql
sum by (operation) (cache_operations_total)
```
*Breaks down cache operations by type (put, get).*

### Cache Operations by Type and Status

```promql
sum by (operation, status) (cache_operations_total)
```
*Shows cache operations broken down by both type (put, get) and status (hit, miss, success).*

### Cache Operations Rate (per second)

```promql
rate(cache_operations_total[1m])
```
*Measures the rate of cache operations per second, averaged over the last minute.*

### Current Cache Size

```promql
cache_size
```
*Shows the current number of items in the cache.*

## Performance Metrics

### Average Request Duration

```promql
avg(rate(request_duration_seconds_sum[5m]) / rate(request_duration_seconds_count[5m]))
```
*Measures the average duration of all requests over the last 5 minutes.*

### Request Duration by Handler and Method

```promql
sum by (handler, method) (rate(request_duration_seconds_sum[5m])) / sum by (handler, method) (rate(request_duration_seconds_count[5m]))
```
*Shows average request duration broken down by handler (endpoint) and HTTP method.*

### 95th Percentile Request Duration

```promql
histogram_quantile(0.95, sum(rate(request_duration_seconds_bucket[5m])) by (le, handler))
```
*Shows the 95th percentile request duration for each handler, meaning 95% of requests are faster than this value.*

### Slow Requests (>100ms)

```promql
sum(rate(request_duration_seconds_bucket{le="0.1"}[5m])) by (handler) / sum(rate(request_duration_seconds_count[5m])) by (handler)
```
*Shows the proportion of requests taking longer than 100ms for each handler.*

## Cache Effectiveness

### Cache Hit Rate

```promql
sum(cache_operations_total{operation="get", status="hit"}) / sum(cache_operations_total{operation="get"})
```
*Measures the cache hit rate as a ratio (proportion of gets that were hits).*

### Cache Hit Rate Percentage

```promql
100 * sum(cache_operations_total{operation="get", status="hit"}) / sum(cache_operations_total{operation="get"})
```
*Measures the cache hit rate as a percentage.*

### Cache Hit Rate Over Time

```promql
rate(cache_operations_total{operation="get", status="hit"}[5m]) / rate(cache_operations_total{operation="get"}[5m])
```
*Shows how the cache hit rate has changed over the last 5 minutes.*

## Resource Usage

### Memory Usage

```promql
cache_memory_bytes
```
*Shows the current memory usage of the cache in bytes.*

### Memory Usage in MB

```promql
cache_memory_bytes / (1024 * 1024)
```
*Shows the current memory usage of the cache in megabytes.*

### Process Resident Memory

```promql
process_resident_memory_bytes
```
*Shows the total resident memory used by the process.*

### CPU Usage

```promql
rate(process_cpu_seconds_total[1m])
```
*Shows the CPU usage rate (both user and system) averaged over the last minute.*

### Goroutine Count

```promql
go_goroutines
```
*Shows the number of currently running goroutines, which can help detect goroutine leaks.*

## Error Rates

### Cache Operation Errors

```promql
sum(cache_operations_total{status="error"}) or vector(0)
```
*Shows the total count of cache operation errors (if you've added error status metrics).*

### Error Rate for GET Operations

```promql
sum(cache_operations_total{operation="get", status="miss"}) / sum(cache_operations_total{operation="get"}) * 100
```
*Shows the percentage of GET operations that resulted in a miss (which may indicate an issue depending on your application).*

### HTTP 4xx/5xx Responses

```promql
sum by (handler, status) (request_duration_seconds_count{status=~"Bad Request|Not Found|Internal Server Error"})
```
*Shows the count of HTTP responses that had error status codes, broken down by handler and status.*

### Error Rate by Handler

```promql
sum by (handler) (request_duration_seconds_count{status=~"Bad Request|Not Found|Internal Server Error"}) / sum by (handler) (request_duration_seconds_count) * 100
```
*Shows the percentage of requests that resulted in errors for each handler.*

## System Health

### Server Information

```promql
server_info
```
*Shows meta information about the server, with labels indicating server type.*

### Uptime

```promql
time() - process_start_time_seconds
```
*Shows how long the cache service has been running in seconds.*

### Open File Descriptors

```promql
process_open_fds / process_max_fds
```
*Shows the ratio of used file descriptors to the maximum allowed, which can help detect resource leaks.*

## Alerts and Thresholds Examples

### High Cache Miss Rate Alert

```promql
100 - (sum(cache_operations_total{operation="get", status="hit"}) / sum(cache_operations_total{operation="get"}) * 100) > 80
```
*Triggers when the cache miss rate exceeds 80% (or hit rate falls below 20%).*

### Slow Response Time Alert

```promql
histogram_quantile(0.95, sum(rate(request_duration_seconds_bucket[5m])) by (le)) > 0.5
```
*Triggers when the 95th percentile response time exceeds 500ms.*

### Memory Growth Alert

```promql
deriv(cache_memory_bytes[10m]) > 0
```
*Triggers when memory usage is consistently growing over a 10-minute window, which might indicate a memory leak.*

### Server Count Alert

```promql
count(server_info) < 3
```
*Triggers when fewer than 3 servers are reporting metrics, which might indicate node failures in the cluster.*

## Dashboard Examples

For Grafana dashboards, you can combine these queries with appropriate visualizations. Some recommended dashboard panels include:

1. Cache Operation Rates - Line graph of operation rates over time
2. Cache Hit/Miss Ratio - Gauge showing current hit ratio
3. Request Latency - Heatmap of request durations
4. Memory Usage - Line graph of memory consumption over time
5. Error Rates - Line graph of error percentages over time

Remember to adjust time windows and thresholds based on your specific application characteristics and requirements.

