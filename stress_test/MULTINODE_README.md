# Multi-Node Stress Test for 1Money

This stress test has been refactored to support multi-node operation with per-node rate limiting.

## Features

- **Multi-node support**: Distribute load across multiple nodes
- **Per-node rate limiting**: Each node has its own independent rate limiter
- **Round-robin distribution**: Requests are evenly distributed across nodes
- **Smooth rate limiting**: Uses small intervals (5ms) to ensure smooth distribution
- **Backward compatibility**: Still supports single-node operation

## Usage

### Single Node (Legacy Mode)

Run without any node parameters to use the default test node:

```bash
./stress_test
```

### Multi-Node Mode

Use the `-nodes` flag to specify multiple nodes:

```bash
./stress_test -nodes "127.0.0.1:18555,127.0.0.1:18556,127.0.0.1:18557,127.0.0.1:18558"
```

### With Custom Rate Limits

Specify custom POST and GET rate limits:

```bash
./stress_test -nodes "127.0.0.1:18555,127.0.0.1:18556" -post-rate 500 -get-rate 1000
```

### Using the Helper Script

A convenience script is provided:

```bash
./run_multinode_test.sh -nodes 127.0.0.1:18555,127.0.0.1:18556,127.0.0.1:18557,127.0.0.1:18558 -post-rate 500 -get-rate 1000
```

## Rate Limiting

- **POST operations**: Includes token creation, minting, transfers, and authority grants
- **GET operations**: Includes nonce checks, receipt verification, and balance queries

The total rate is distributed evenly across all nodes. For example:
- 4 nodes with 800 POST TPS = 200 POST TPS per node
- 4 nodes with 2000 GET TPS = 500 GET TPS per node

## Architecture

### Key Components

1. **NodePool**: Manages multiple nodes and provides round-robin distribution
2. **MultiNodeRateLimiter**: Manages per-node rate limiters
3. **NodeRateLimiter**: Individual rate limiter for each node with smooth token distribution
4. **Multi-node operations**: All operations (create token, grant authority, mint, transfer) support multi-node

### Operation Distribution

- **Token Creation**: Single operation, uses one node
- **Authority Grants**: Distributed across nodes with concurrency limit
- **Minting**: Each mint wallet's operations distributed across nodes
- **Transfers**: Distributed across nodes with worker pools

## Statistics

The system provides detailed statistics:

1. **Rate Limiter Statistics**: Shows actual TPS per node
2. **Node Distribution Statistics**: Shows operation counts per node
3. **Performance Metrics**: Overall throughput and timing

## Configuration

Edit `config.go` to adjust:

- `MINT_WALLETS_COUNT`: Number of mint authority wallets
- `TRANSFER_WALLETS_COUNT`: Number of primary transfer wallets
- `DISTRIBUTION_WALLETS_COUNT`: Number of final distribution wallets
- `POST_RATE_LIMIT_TPS`: Default POST rate limit
- `GET_RATE_LIMIT_TPS`: Default GET rate limit
- `TRANSFER_WORKERS_COUNT`: Number of concurrent transfer workers

## Example Output

```
=== Multi-Node Configuration ===
Number of nodes: 4
Node 1: http://127.0.0.1:18555
Node 2: http://127.0.0.1:18556
Node 3: http://127.0.0.1:18557
Node 4: http://127.0.0.1:18558
Total POST rate: 500 TPS (125.00 TPS per node)
Total GET rate: 1000 TPS (250.00 TPS per node)

=== Rate Limiter Configuration ===
Total requested POST rate: 500 TPS
Total requested GET rate: 1000 TPS
Number of nodes: 4
Base POST rate per node: 125 TPS
Base GET rate per node: 250 TPS
Node 0 (http://127.0.0.1:18555): POST 125 TPS (1 token every 8ms), GET 250 TPS (1 token every 4ms)
Node 1 (http://127.0.0.1:18556): POST 125 TPS (1 token every 8ms), GET 250 TPS (1 token every 4ms)
Node 2 (http://127.0.0.1:18557): POST 125 TPS (1 token every 8ms), GET 250 TPS (1 token every 4ms)
Node 3 (http://127.0.0.1:18558): POST 125 TPS (1 token every 8ms), GET 250 TPS (1 token every 4ms)
```