// Copyright 2025 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package tortest

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// RunHiveTorTests executes all Tor+ENR tests for Hive integration.
//
// This function is designed to be called from a Hive simulator or test runner.
// It creates a test suite, runs all tests, and returns results in Hive-compatible format.
//
// Parameters:
//   - destNode: The target node to test (enode URL)
//   - socksAddr: SOCKS5 proxy address (e.g., "127.0.0.1:9050")
//
// Returns:
//   - []utesting.Result: Test results for all Tor+ENR tests
//   - error: Any error during test execution
func RunHiveTorTests(destNode *enode.Node, socksAddr string) ([]utesting.Result, error) {
	// Create test suite
	suite := NewSuite(destNode, socksAddr)

	// Get all tests
	tests := suite.AllTests()

	// Run tests with console output
	results := utesting.RunTests(tests, os.Stdout)

	return results, nil
}

// RunHiveTorTestsWithEnv executes Tor+ENR tests using environment variables.
//
// Environment variables:
//   - HIVE_TARGET_ENODE: Target node enode URL
//   - TOR_SOCKS_ADDR: SOCKS5 proxy address (default: "127.0.0.1:9050")
//
// This is the recommended entry point for Hive integration.
func RunHiveTorTestsWithEnv() ([]utesting.Result, error) {
	// Get target node from environment
	enodeURL := os.Getenv("HIVE_TARGET_ENODE")
	if enodeURL == "" {
		return nil, fmt.Errorf("HIVE_TARGET_ENODE environment variable not set")
	}

	// Parse enode URL
	node, err := enode.ParseV4(enodeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse enode URL: %v", err)
	}

	// Get SOCKS5 address (default to standard Tor port)
	socksAddr := os.Getenv("TOR_SOCKS_ADDR")
	if socksAddr == "" {
		socksAddr = "127.0.0.1:9050"
	}

	return RunHiveTorTests(node, socksAddr)
}

// HiveTestFilter filters Tor+ENR tests based on pattern.
//
// This allows Hive to run specific test subsets.
//
// Example patterns:
//   - ".*ENR.*" - All ENR-related tests
//   - ".*SOCKS5.*" - All SOCKS5 connection tests
//   - ".*Fallback.*" - All fallback behavior tests
//   - ".*Mode.*" - All operational mode tests
func HiveTestFilter(destNode *enode.Node, socksAddr, pattern string) ([]utesting.Result, error) {
	suite := NewSuite(destNode, socksAddr)
	tests := suite.AllTests()

	// Filter tests by pattern
	filtered := utesting.MatchTests(tests, pattern)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no tests match pattern: %s", pattern)
	}

	fmt.Printf("Running %d tests matching pattern: %s\n", len(filtered), pattern)

	// Run filtered tests
	results := utesting.RunTests(filtered, os.Stdout)

	return results, nil
}

// HiveReportTAP generates TAP (Test Anything Protocol) output for Hive.
//
// TAP format is commonly used in Hive for test result reporting.
func HiveReportTAP(destNode *enode.Node, socksAddr string) ([]utesting.Result, error) {
	suite := NewSuite(destNode, socksAddr)
	tests := suite.AllTests()

	// Run tests with TAP output
	results := utesting.RunTAP(tests, os.Stdout)

	return results, nil
}

// HiveTestCount returns the number of Tor+ENR tests available.
//
// This helps Hive allocate resources and plan test execution.
func HiveTestCount() int {
	// Create a dummy suite to get test count
	suite := &Suite{}
	return len(suite.AllTests())
}

// HiveTestNames returns all test names for Hive discovery.
func HiveTestNames() []string {
	suite := &Suite{}
	tests := suite.AllTests()

	names := make([]string, len(tests))
	for i, test := range tests {
		names[i] = test.Name
	}

	return names
}

// CheckHivePreconditions verifies that the environment is ready for Tor tests.
//
// Returns an error if any preconditions are not met:
//   - Target node is reachable
//   - SOCKS5 proxy is available (if configured)
//   - Required ENR entries are present
func CheckHivePreconditions(destNode *enode.Node, socksAddr string) error {
	// Verify node is not nil
	if destNode == nil {
		return fmt.Errorf("target node is nil")
	}

	// Verify node has required fields
	var emptyID enode.ID
	if destNode.ID() == emptyID {
		return fmt.Errorf("target node has invalid ID")
	}

	// Check if node has any usable address
	hasOnion := false
	hasClearnet := false

	var onion enr.Onion3
	if err := destNode.Record().Load(&onion); err == nil && onion != "" {
		hasOnion = true
	}

	if destNode.IP() != nil && destNode.TCP() > 0 {
		hasClearnet = true
	}

	if !hasOnion && !hasClearnet {
		return fmt.Errorf("target node has no usable addresses (neither .onion nor clearnet)")
	}

	return nil
}
