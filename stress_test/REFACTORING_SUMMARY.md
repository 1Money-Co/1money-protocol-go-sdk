# Stress Test Multi-Node Refactoring Summary

## Overview

The stress test has been systematically refactored to support multi-node operation with per-node rate limiting, similar to the load_runner implementation.

## Key Changes

### 1. New Files Created

- **node_pool.go**: Manages multiple nodes with round-robin distribution
- **per_node_rate_limiter.go**: Implements per-node rate limiting with smooth token distribution
- **stress_tester_multinode.go**: Contains multi-node initialization and helper functions
- **multinode_operations.go**: All operations (token creation, authority grants, minting, transfers) with multi-node support
- **stress_test_runner_multinode.go**: Multi-node test runner with detailed statistics
- **csv_generator.go**: Updated CSV generator with multi-node support
- **run_multinode_test.sh**: Shell script for running multi-node tests
- **test_multinode.sh**: Example test script
- **MULTINODE_README.md**: Documentation for multi-node features

### 2. Modified Files

- **main.go**: Added command-line flag parsing for multi-node support
- **config.go**: Updated StressTester struct to use NodePool and MultiNodeRateLimiter
- **stress_tester.go**: All methods now delegate to multi-node versions for consistency
- **transaction_utils.go**: Helper methods delegate to multi-node versions

### 3. Architecture Changes

#### Before (Single Node)
```
StressTester
├── client: *onemoney.Client (single client)
├── postRateLimiter: *rate.Limiter (global)
└── getRateLimiter: *rate.Limiter (global)
```

#### After (Multi-Node)
```
StressTester
├── nodePool: *NodePool (manages multiple nodes)
│   └── nodes: []*NodeInfo (client per node)
└── rateLimiter: *MultiNodeRateLimiter
    └── nodeLimiters: []*NodeRateLimiter (per-node rate limiters)
```

### 4. Rate Limiting Improvements

- **Per-node independence**: Each node has its own rate limiter with no interference
- **Smooth distribution**: Uses 5ms intervals for 200 TPS (configurable)
- **No bursts**: Strict sequential token distribution per node
- **Even distribution**: Total rate divided evenly across nodes

### 5. Command Line Interface

```bash
# Single node (legacy)
./stress_test

# Multi-node with default rates
./stress_test -nodes "127.0.0.1:18555,127.0.0.1:18556"

# Multi-node with custom rates
./stress_test -nodes "127.0.0.1:18555,127.0.0.1:18556" -post-rate 500 -get-rate 1000
```

### 6. Backward Compatibility

- Single-node mode still works without any changes
- All existing methods maintained as wrappers to multi-node versions
- Default behavior unchanged when no nodes specified

## Benefits

1. **Scalability**: Can distribute load across multiple nodes
2. **Reliability**: If one node has issues, others continue working
3. **Performance**: Higher overall throughput with multiple nodes
4. **Flexibility**: Easy to add/remove nodes via command line
5. **Monitoring**: Detailed per-node statistics and distribution metrics

## Example Usage

```bash
# Run stress test with 4 nodes, 500 POST TPS, 1000 GET TPS
./run_multinode_test.sh \
    -nodes "127.0.0.1:18555,127.0.0.1:18556,127.0.0.1:18557,127.0.0.1:18558" \
    -post-rate 500 \
    -get-rate 1000
```

This will:
- Distribute 500 POST TPS across 4 nodes (125 TPS per node)
- Distribute 1000 GET TPS across 4 nodes (250 TPS per node)
- Create token, grant authorities, mint tokens, and distribute them
- Provide detailed statistics for each node

## Testing

The refactored system has been tested to ensure:
- Compilation succeeds without errors
- Command-line parsing works correctly
- Multi-node distribution is even
- Rate limiting is accurate per node
- Backward compatibility is maintained