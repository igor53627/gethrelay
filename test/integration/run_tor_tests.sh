#!/bin/bash
# Copyright 2025 The go-ethereum Authors
# This file is part of go-ethereum.
#
# Run Tor integration tests for gethrelay ENR+Tor connectivity.
#
# Usage:
#   ./run_tor_tests.sh              # Run all integration tests
#   ./run_tor_tests.sh --real-tor   # Run with real Tor daemon (requires Tor installed)
#   ./run_tor_tests.sh --verbose    # Run with verbose output

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TOR_PID=""
USE_REAL_TOR=false
VERBOSE=false

# Parse arguments
for arg in "$@"; do
    case $arg in
        --real-tor)
            USE_REAL_TOR=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            echo "Usage: $0 [--real-tor] [--verbose] [--help]"
            echo ""
            echo "Options:"
            echo "  --real-tor    Use real Tor daemon instead of mock SOCKS5 proxy"
            echo "  --verbose     Enable verbose test output"
            echo "  --help        Show this help message"
            exit 0
            ;;
    esac
done

echo "=== Tor Integration Test Runner ==="
echo "Project root: $PROJECT_ROOT"
echo ""

# Check if Tor is available
if [ "$USE_REAL_TOR" = true ]; then
    if ! command -v tor &> /dev/null; then
        echo "ERROR: Tor not found. Please install Tor:"
        echo "  macOS:   brew install tor"
        echo "  Ubuntu:  sudo apt-get install tor"
        echo "  Arch:    sudo pacman -S tor"
        exit 1
    fi

    echo "Starting Tor daemon..."
    tor --ControlPort 9051 --SOCKSPort 9050 --DataDirectory /tmp/tor-test-data &
    TOR_PID=$!

    # Wait for Tor to be ready
    sleep 3

    if ! ps -p $TOR_PID > /dev/null; then
        echo "ERROR: Tor failed to start"
        exit 1
    fi

    echo "Tor daemon started (PID: $TOR_PID)"
    echo "  SOCKS5 proxy: 127.0.0.1:9050"
    echo "  Control port: 127.0.0.1:9051"
    echo ""
fi

# Cleanup function
cleanup() {
    if [ ! -z "$TOR_PID" ]; then
        echo ""
        echo "Stopping Tor daemon..."
        kill $TOR_PID 2>/dev/null || true
        wait $TOR_PID 2>/dev/null || true
        rm -rf /tmp/tor-test-data
    fi
}

trap cleanup EXIT INT TERM

# Run integration tests
echo "Running Tor integration tests..."
echo ""

cd "$PROJECT_ROOT"

TEST_FLAGS="-run TestTorIntegration"
if [ "$VERBOSE" = true ]; then
    TEST_FLAGS="$TEST_FLAGS -v"
fi

# Run tests
if go test $TEST_FLAGS ./p2p -timeout 2m; then
    echo ""
    echo "=== All integration tests passed! ==="
    exit 0
else
    echo ""
    echo "=== Integration tests failed ==="
    exit 1
fi
