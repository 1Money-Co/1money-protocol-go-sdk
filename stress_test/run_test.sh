#!/bin/bash

# 1Money Batch Mint Stress Test Runner
# ====================================

echo "1Money Batch Mint Stress Testing Tool"
echo "====================================="
echo

# Check if environment variables are set
if [ -z "$OPERATOR_PRIVATE_KEY" ] || [ -z "$OPERATOR_ADDRESS" ]; then
    echo "⚠️  WARNING: Operator credentials not found in environment variables"
    echo "Please set the following environment variables:"
    echo "  export OPERATOR_PRIVATE_KEY=\"your_operator_private_key_here\""
    echo "  export OPERATOR_ADDRESS=\"your_operator_address_here\""
    echo
    echo "Or configure TestOperatorPrivateKey and TestOperatorAddress in the SDK"
    echo
fi

echo "🚀 Starting stress test..."
echo

# Run the test with verbose output
go test -v -run TestBatchMint

# Check the exit code
if [ $? -eq 0 ]; then
    echo
    echo "✅ Stress test completed successfully!"
else
    echo
    echo "❌ Stress test failed!"
    exit 1
fi
