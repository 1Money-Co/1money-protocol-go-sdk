# Micro-Burst Prevention Example

## How It Works

The load runner now implements micro-burst prevention to ensure the instantaneous rate never exceeds 2x the configured rate limit, matching the server's micro-burst configuration.

### Implementation Details

1. **Dual Rate Limiting**:
   - **Token Bucket**: Controls average rate over time
   - **Sliding Window**: Prevents bursts in any 100ms window

2. **Burst Detection**:
   - Tracks timestamps of recent requests in a sliding 100ms window
   - If window contains >= (2x rate × 0.1s) requests, delays next request

### Example Scenarios

#### Scenario 1: Normal Operation (100 TPS)
- Configured rate: 100 TPS
- Token interval: 10ms (1 request every 10ms)
- Max burst in 100ms: 20 requests
- Result: Smooth, evenly distributed requests

#### Scenario 2: Burst Prevention
With 100 TPS rate limit:
- If 20 requests sent in first 50ms of a 100ms window
- Next request blocked until oldest request exits the window
- Prevents exceeding 2x rate in any 100ms period

#### Scenario 3: Multi-Node Distribution
With 4 nodes and 400 TPS total:
- Each node: 100 TPS average, max 20 requests/100ms
- Total system: 400 TPS average, max 80 requests/100ms
- Each node independently prevents micro-bursts

## Configuration Examples

### Conservative (Prevent All Bursts)
```bash
./load_runner -to 0x123... -post-rate 50 -get-rate 100
```
- POST: 50 TPS (max 10 requests/100ms)
- GET: 100 TPS (max 20 requests/100ms)

### Balanced Performance
```bash
./load_runner -to 0x123... -post-rate 200 -get-rate 400
```
- POST: 200 TPS (max 40 requests/100ms)
- GET: 400 TPS (max 80 requests/100ms)

### High Performance with Multiple Nodes
```bash
./load_runner -to 0x123... -nodes "n1:8080,n2:8080,n3:8080,n4:8080" -post-rate 1000
```
- Total: 1000 TPS across 4 nodes
- Per node: 250 TPS (max 50 requests/100ms)

## Benefits

1. **Server Protection**: Prevents triggering server-side micro-burst throttling
2. **Predictable Performance**: Smooth, consistent request flow
3. **Better Success Rate**: Reduces request failures due to bursts
4. **Optimal Throughput**: Maximizes successful requests within limits

## Monitoring

The rate limiter logs show burst prevention:
```
=== POST Rate Limiter Configuration ===
Total requested rate: 200 TPS
Number of nodes: 2
Base rate per node: 100 TPS
Micro-burst prevention: 2x rate limit in 100ms windows
Node 0 (http://node1:8080): 100 TPS (1 token every 10ms, max 20 reqs/100ms)
Node 1 (http://node2:8080): 100 TPS (1 token every 10ms, max 20 reqs/100ms)
```