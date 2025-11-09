# Hive Integration Guide for Tor+ENR P2P Tests

This document explains how to integrate Tor+ENR tests into the Hive testing framework.

## Quick Start

### Run Tests Locally

```bash
# Build Docker image
docker build -f cmd/gethrelay/Dockerfile.gethrelay -t ethereum/gethrelay:local .

# Run Hive tests
bash cmd/gethrelay/test-hive.sh
```

### Run Specific Test Categories

```bash
# ENR tests only
HIVE_TEST_PATTERN=".*ENR.*" bash cmd/gethrelay/test-hive.sh

# SOCKS5 connection tests
HIVE_TEST_PATTERN=".*SOCKS5.*" bash cmd/gethrelay/test-hive.sh

# Fallback behavior tests
HIVE_TEST_PATTERN=".*Fallback.*" bash cmd/gethrelay/test-hive.sh

# Operational mode tests
HIVE_TEST_PATTERN=".*Mode.*" bash cmd/gethrelay/test-hive.sh
```

## Hive Simulator Integration

### Directory Structure

```
hive/
├── simulators/
│   └── ethereum/
│       └── tor-p2p/          # New Tor+ENR simulator
│           ├── suite.yaml
│           ├── main.go
│           └── tests/
│               └── tor_tests.go
└── clients/
    └── gethrelay/
        ├── hive.yaml
        ├── Dockerfile.local
        ├── gethrelay.sh
        └── enode.sh
```

### Creating Tor+ENR Simulator

**File: `hive/simulators/ethereum/tor-p2p/suite.yaml`**

```yaml
name: "Tor+ENR P2P Integration Tests"
description: "Tests Tor hidden service integration with Ethereum P2P networking"
version: "1.0.0"

tests:
  - name: "ENR Propagation Tests"
    description: "Validates .onion address distribution via ENR"
    pattern: ".*ENR.*"

  - name: "SOCKS5 Connection Tests"
    description: "Tests Tor proxy-based peer connections"
    pattern: ".*SOCKS5.*"

  - name: "Fallback Behavior Tests"
    description: "Tests clearnet fallback when Tor unavailable"
    pattern: ".*Fallback.*"

  - name: "Operational Mode Tests"
    description: "Tests default, prefer-tor, only-onion modes"
    pattern: ".*Mode.*"

  - name: "Dual-Stack Tests"
    description: "Tests simultaneous Tor and clearnet reachability"
    pattern: ".*DualStack.*"

  - name: "Error Handling Tests"
    description: "Tests graceful failure scenarios"
    pattern: ".*Invalid.*|.*Malformed.*|.*NoUsable.*"
```

**File: `hive/simulators/ethereum/tor-p2p/main.go`**

```go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/cmd/devp2p/internal/tortest"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

func main() {
	var (
		enodeURL   = flag.String("enode", os.Getenv("HIVE_TARGET_ENODE"), "target enode URL")
		socksAddr  = flag.String("socks", "127.0.0.1:9050", "SOCKS5 proxy address")
		testFilter = flag.String("pattern", ".*", "test filter pattern")
		tapOutput  = flag.Bool("tap", false, "use TAP output format")
	)
	flag.Parse()

	// Parse enode
	node, err := enode.ParseV4(*enodeURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse enode: %v\n", err)
		os.Exit(1)
	}

	// Check preconditions
	if err := tortest.CheckHivePreconditions(node, *socksAddr); err != nil {
		fmt.Fprintf(os.Stderr, "precondition check failed: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	var results []utesting.Result
	if *tapOutput {
		results, err = tortest.HiveReportTAP(node, *socksAddr)
	} else if *testFilter != ".*" {
		results, err = tortest.HiveTestFilter(node, *socksAddr, *testFilter)
	} else {
		results, err = tortest.RunHiveTorTests(node, *socksAddr)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "test execution failed: %v\n", err)
		os.Exit(1)
	}

	// Exit with failure code if any tests failed
	for _, result := range results {
		if result.Failed {
			os.Exit(1)
		}
	}
}
```

## Environment Variables

### Required

- `HIVE_TARGET_ENODE` - Target node enode URL to test

### Optional

- `TOR_SOCKS_ADDR` - SOCKS5 proxy address (default: `127.0.0.1:9050`)
- `HIVE_TEST_PATTERN` - Test filter regex (default: `.*`)
- `HIVE_LOGLEVEL` - Logging verbosity (0-5, default: 3)
- `TEST_TIMEOUT` - Per-test timeout (default: `10s`)

## CI/CD Integration

### GitHub Actions Workflow

**File: `.github/workflows/hive-tor-tests.yml`**

```yaml
name: Hive Tor+ENR Tests

on:
  push:
    branches: [main, master]
    paths:
      - 'p2p/tor_dialer.go'
      - 'p2p/enr/entries.go'
      - 'node/tor.go'
      - 'cmd/devp2p/internal/tortest/**'
  pull_request:
  workflow_dispatch:

jobs:
  hive-tor-tests:
    name: Run Hive Tor+ENR Tests
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build Docker image
        run: |
          docker build -f cmd/gethrelay/Dockerfile.gethrelay \
            -t ethereum/gethrelay:local .

      - name: Setup Hive
        run: |
          git clone --depth=1 https://github.com/ethereum/hive.git /tmp/hive
          cd /tmp/hive && go build -o /usr/local/bin/hive .

      - name: Setup Tor proxy (mock)
        run: |
          # For testing, use mock SOCKS5 server
          # In production, use: docker run -d --name tor -p 9050:9050 osminogin/tor-simple

      - name: Run Tor+ENR tests
        run: |
          hive --sim=ethereum/tor-p2p \
               --client=gethrelay:local \
               --loglevel=5
        env:
          TOR_SOCKS_ADDR: "127.0.0.1:9050"

      - name: Upload test results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: hive-tor-test-results
          path: /tmp/hive/workspace/logs/
          retention-days: 7
```

## Test Execution Flow

### 1. Hive Setup Phase

```
1. Clone Hive repository
2. Build Hive binary
3. Setup client configurations
4. Build client Docker images
```

### 2. Test Initialization

```
1. Start Tor SOCKS5 proxy (or mock)
2. Start target node with Tor enabled
3. Verify node is reachable
4. Extract ENR and check for .onion entry
```

### 3. Test Execution

```
For each test:
  1. Setup test environment
  2. Execute test scenario
  3. Validate expected behavior
  4. Collect results
  5. Cleanup resources
```

### 4. Result Reporting

```
1. Aggregate test results
2. Generate reports (console/TAP)
3. Upload artifacts
4. Exit with appropriate code
```

## Mock Tor Proxy

For testing without real Tor:

```bash
# Use built-in mock SOCKS5 server
go run cmd/devp2p/internal/tortest/mock_tor.go

# Or use Docker-based mock
docker run -d --name mock-tor \
  -p 9050:9050 \
  ethereum/mock-socks5:latest
```

## Real Tor Integration

For production testing with actual Tor:

```bash
# Install Tor
sudo apt-get install tor

# Configure Tor
cat > /etc/tor/torrc <<EOF
SOCKSPort 9050
SOCKSPolicy accept 127.0.0.1/8
Log notice stdout
EOF

# Start Tor
sudo systemctl start tor

# Verify Tor is running
curl --socks5 127.0.0.1:9050 https://check.torproject.org
```

## Debugging Failed Tests

### Enable Verbose Logging

```bash
export HIVE_LOGLEVEL=5
export TEST_TIMEOUT=30s
go test ./cmd/devp2p/internal/tortest -v -run FailingTest
```

### Check SOCKS5 Connectivity

```bash
# Test SOCKS5 proxy
curl --socks5 127.0.0.1:9050 https://check.torproject.org

# Check Tor circuits
sudo -u debian-tor tor-ctrl
```

### Inspect ENR Records

```bash
# Extract ENR from node
geth attach /path/to/geth.ipc --exec "admin.nodeInfo.enr"

# Decode ENR
go run cmd/devp2p enrdump <enr>
```

### Monitor Network Traffic

```bash
# Capture SOCKS5 traffic
tcpdump -i lo -A 'tcp port 9050'

# Monitor Tor connections
sudo netstat -tunap | grep :9050
```

## Performance Considerations

### Test Execution Time

- **ENR tests:** ~100ms each
- **SOCKS5 tests:** ~500ms each (includes handshake)
- **Fallback tests:** ~1-2s each (includes timeout)
- **Full suite:** ~15-30s total

### Resource Requirements

- **CPU:** Minimal (< 10% single core)
- **Memory:** ~100MB for test suite
- **Network:** Local (127.0.0.1) or Tor bandwidth
- **Disk:** ~50MB for logs

### Optimization Tips

```bash
# Run tests in parallel (where safe)
go test ./cmd/devp2p/internal/tortest -parallel 4

# Skip slow tests
go test ./cmd/devp2p/internal/tortest -short

# Use mock Tor for faster iteration
export TOR_SOCKS_ADDR="127.0.0.1:9051"  # Mock proxy
```

## Troubleshooting Matrix

| Symptom | Possible Cause | Solution |
|---------|---------------|----------|
| All tests timeout | Tor proxy not running | Start Tor or use mock |
| ENR validation fails | Invalid .onion format | Check base32 encoding |
| SOCKS5 connection fails | Proxy address incorrect | Verify TOR_SOCKS_ADDR |
| Fallback doesn't work | No clearnet address | Add IP to ENR |
| Only-onion mode fails | Node has no .onion | Configure Tor hidden service |

## Test Coverage Analysis

```bash
# Generate coverage report
go test ./cmd/devp2p/internal/tortest -coverprofile=coverage.out

# View in browser
go tool cover -html=coverage.out

# Check coverage percentage
go tool cover -func=coverage.out | grep total
```

**Expected Coverage:**
- **Overall:** 85%+
- **Core logic:** 95%+
- **Error paths:** 90%+
- **Integration scenarios:** 80%+

## Contributing

When adding new Tor+ENR tests:

1. Add test function to `suite.go`
2. Add to `TorTests()` method
3. Update `README.md` with test description
4. Add test scenario to this guide
5. Update CI workflow if needed
6. Verify Hive compatibility
7. Submit PR with test results

## References

- [Hive Documentation](https://github.com/ethereum/hive)
- [Tor SOCKS5 Protocol](https://www.rfc-editor.org/rfc/rfc1928)
- [ENR Specification (EIP-778)](https://eips.ethereum.org/EIPS/eip-778)
- [go-ethereum P2P](https://geth.ethereum.org/docs/developers/geth-developer/dev-guide)

## License

Copyright 2025 The go-ethereum Authors. Licensed under GNU GPL v3.0.
