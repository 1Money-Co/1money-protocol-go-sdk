# Rate Limiting Examples

## How Rate Limiting Works

### Server Limits
- POST requests: 250 TPS per node
- GET requests: 500 TPS per node

### Examples

#### Example 1: Single Node
```bash
./run_load_test.sh 0xADDRESS -nodes "node1:8080" -concurrency 300
```
- Requested concurrency: 300
- Effective concurrency: 250 (1 node × 250 TPS)
- Result: Capped at 250 TPS

#### Example 2: Two Nodes
```bash
./run_load_test.sh 0xADDRESS -nodes "node1:8080,node2:8080" -concurrency 600
```
- Requested concurrency: 600
- Effective concurrency: 500 (2 nodes × 250 TPS)
- Result: Capped at 500 TPS

#### Example 3: Four Nodes
```bash
./run_load_test.sh 0xADDRESS -nodes "n1:8080,n2:8080,n3:8080,n4:8080" -concurrency 800
```
- Requested concurrency: 800
- Effective concurrency: 800 (within limit of 4 × 250 = 1000)
- Result: Uses full 800 concurrency

#### Example 4: Maximum Nodes (13)
```bash
./run_load_test.sh 0xADDRESS -nodes "n1:8080,n2:8080,...,n13:8080" -concurrency 5000
```
- Requested concurrency: 5000
- Effective concurrency: 3250 (13 nodes × 250 TPS)
- Result: Capped at 3250 TPS

### Verification Rate Limiting

Verification uses GET requests with higher limits:

```bash
# With 2 nodes, verification can use up to 1000 TPS (2 × 500)
# With 4 nodes, verification can use up to 2000 TPS (4 × 500)
```

### Benefits

1. **Prevents server overload**: Ensures you don't exceed server capacity
2. **Automatic adjustment**: No manual calculation needed
3. **Per-node scaling**: More nodes = higher total throughput
4. **Separate limits**: Different limits for POST and GET operations