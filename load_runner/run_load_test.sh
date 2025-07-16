#!/bin/bash

echo "1Money Load Runner - Transaction Sender"
echo "======================================"
echo

if [ -z "$1" ]; then
    echo "Usage: ./run_load_test.sh <target_address> [options]"
    echo
    echo "Options:"
    echo "  -csv <file>      CSV file path (default: ../stress_test/accounts_detail.csv)"
    echo "  -amount <value>  Amount to send per transaction (default: 1000000)"
    echo "  -concurrency <n> Number of concurrent transactions (default: 10)"
    echo "  -max <n>         Maximum accounts to process (default: all)"
    echo "  -mainnet         Use mainnet instead of testnet"
    echo
    echo "Example:"
    echo "  ./run_load_test.sh 0x742d35Cc6634C0532925a3b844Bc9e7595f87890"
    echo "  ./run_load_test.sh 0x742d35Cc6634C0532925a3b844Bc9e7595f87890 -concurrency 20 -max 100"
    exit 1
fi

TARGET_ADDRESS=$1
shift

echo "🎯 Target Address: $TARGET_ADDRESS"
echo

go mod download

# Build if binary doesn't exist
if [ ! -f ./load_runner ]; then
    echo "Building load_runner..."
    go build -o load_runner .
fi

echo "🚀 Starting load test..."
echo

./load_runner -to "$TARGET_ADDRESS" "$@"

if [ $? -eq 0 ]; then
    echo
    echo "✅ Load test completed successfully!"
else
    echo
    echo "❌ Load test failed!"
    exit 1
fi