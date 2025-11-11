// Copyright 2025 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestHTTPRPCIntegration tests the full HTTP RPC server with command-line flags
func TestHTTPRPCIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build gethrelay binary for testing
	buildCmd := exec.Command("go", "build", "-o", "/tmp/gethrelay-test", ".")
	buildCmd.Dir = "."
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build gethrelay: %v\n%s", err, output)
	}
	defer os.Remove("/tmp/gethrelay-test")

	// Start gethrelay with HTTP RPC enabled
	cmd := exec.Command("/tmp/gethrelay-test",
		"--chain", "sepolia",
		"--nodiscover",
		"--maxpeers", "0",
		"--http",
		"--http.addr", "127.0.0.1",
		"--http.port", "0", // Random port
		"--http.api", "admin,eth,net,web3",
	)

	// Capture output to find the HTTP port
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Failed to get stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start gethrelay: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Read output to find HTTP endpoint
	outputChan := make(chan string, 100)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				line := string(buf[:n])
				outputChan <- line
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				line := string(buf[:n])
				outputChan <- line
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for server to start (look for "HTTP server started")
	var httpEndpoint string
	timeout := time.After(10 * time.Second)
	for {
		select {
		case line := <-outputChan:
			t.Logf("Output: %s", line)
			// Look for HTTP endpoint in logs: "HTTP server started endpoint=127.0.0.1:PORT"
			if strings.Contains(line, "HTTP server started") && strings.Contains(line, "endpoint=") {
				// Extract endpoint from log line
				parts := strings.Split(line, "endpoint=")
				if len(parts) > 1 {
					endpointParts := strings.Fields(parts[1])
					if len(endpointParts) > 0 {
						endpoint := strings.TrimSpace(endpointParts[0])
						httpEndpoint = "http://" + endpoint
					}
				}
			}
			if httpEndpoint != "" {
				goto TestAPI
			}
		case <-timeout:
			t.Fatal("Timeout waiting for HTTP server to start")
		}
	}

TestAPI:
	// Give server a moment to fully initialize
	time.Sleep(500 * time.Millisecond)

	// If we didn't capture the endpoint from logs, try default
	if httpEndpoint == "" {
		httpEndpoint = "http://127.0.0.1:8545"
		t.Logf("Using default endpoint: %s", httpEndpoint)
	}

	// Test admin_nodeInfo API
	requestBody := `{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`
	resp, err := http.Post(httpEndpoint, "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to call admin_nodeInfo: %v (endpoint: %s)", err, httpEndpoint)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, body)
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var rpcResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Enode string `json:"enode"`
			ENR   string `json:"enr"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &rpcResp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nResponse: %s", err, body)
	}

	if rpcResp.Error != nil {
		t.Fatalf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	// Verify ENR is present
	if rpcResp.Result.ENR == "" {
		t.Fatal("Expected ENR in nodeInfo response, got empty string")
	}

	// ENR should start with "enr:"
	if !strings.HasPrefix(rpcResp.Result.ENR, "enr:") {
		t.Errorf("Expected ENR to start with 'enr:', got: %s", rpcResp.Result.ENR)
	}

	t.Logf("Integration test passed! ENR: %s", rpcResp.Result.ENR[:30]+"...")
}
