#!/bin/bash

# Test script for multi-node stress test

echo "Testing multi-node stress test with 4 nodes..."
echo "This will create token, grant authorities, mint tokens, and distribute them across multiple wallets"
echo ""

# Example with 4 nodes
./run_multinode_test.sh \
    -nodes "127.0.0.1:18555,127.0.0.1:18556,127.0.0.1:18557,127.0.0.1:18558" \
    -post-rate 500 \
    -get-rate 1000

echo ""
echo "Test completed. Check the log file for detailed results."