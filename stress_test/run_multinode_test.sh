#!/bin/bash

# run_multinode_test.sh - Run multi-node stress test for 1Money

# Default values
NODES=""
POST_RATE=250
GET_RATE=500

# Usage function
usage() {
    echo "Usage: $0 -nodes <node1,node2,...> [-post-rate <rate>] [-get-rate <rate>]"
    echo ""
    echo "Required:"
    echo "  -nodes        Comma-separated list of node URLs (e.g., '127.0.0.1:18555,127.0.0.1:18556')"
    echo ""
    echo "Optional:"
    echo "  -post-rate    Total POST rate limit in TPS (default: 250)"
    echo "  -get-rate     Total GET rate limit in TPS (default: 500)"
    echo ""
    echo "Example:"
    echo "  $0 -nodes 127.0.0.1:18555,127.0.0.1:18556,127.0.0.1:18557,127.0.0.1:18558 -post-rate 500 -get-rate 1000"
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -nodes)
            NODES="$2"
            shift 2
            ;;
        -post-rate)
            POST_RATE="$2"
            shift 2
            ;;
        -get-rate)
            GET_RATE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "Unknown option: $1"
            usage
            ;;
    esac
done

# Check required arguments
if [ -z "$NODES" ]; then
    echo "Error: -nodes is required"
    usage
fi

# Build the stress test
echo "Building stress test..."
if ! go build -o stress_test .; then
    echo "Build failed!"
    exit 1
fi

# Run the stress test
echo "Running multi-node stress test..."
echo "Nodes: $NODES"
echo "POST Rate: $POST_RATE TPS"
echo "GET Rate: $GET_RATE TPS"
echo ""

./stress_test -nodes "$NODES" -post-rate "$POST_RATE" -get-rate "$GET_RATE"