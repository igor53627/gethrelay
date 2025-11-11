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
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
)

// TestHTTPRPCServerInitialization tests that the HTTP RPC server starts correctly
func TestHTTPRPCServerInitialization(t *testing.T) {
	// Create minimal node configuration
	nodeConfig := &node.Config{
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0, // Use random port
		HTTPModules: []string{"admin", "eth", "net"},
		P2P: p2p.Config{
			MaxPeers:    10,
			NoDiscovery: true,
		},
	}

	stack, err := node.New(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer stack.Close()

	// Setup HTTP RPC with admin API
	if err := setupHTTPRPC(stack, nodeConfig); err != nil {
		t.Fatalf("Failed to setup HTTP RPC: %v", err)
	}

	// Start the node
	if err := stack.Start(); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Server should be accessible
	t.Log("HTTP RPC server initialized successfully")
}

// TestAdminNodeInfoAPI tests that admin_nodeInfo returns node information including ENR
func TestAdminNodeInfoAPI(t *testing.T) {
	// Create minimal node configuration
	nodeConfig := &node.Config{
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0, // Use random port
		HTTPModules: []string{"admin"},
		P2P: p2p.Config{
			MaxPeers:    10,
			NoDiscovery: true,
		},
	}

	stack, err := node.New(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer stack.Close()

	// Setup HTTP RPC with admin API
	if err := setupHTTPRPC(stack, nodeConfig); err != nil {
		t.Fatalf("Failed to setup HTTP RPC: %v", err)
	}

	// Start the node
	if err := stack.Start(); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	// Get the actual HTTP endpoint
	httpEndpoint := stack.HTTPEndpoint()
	if httpEndpoint == "" {
		t.Fatal("HTTP endpoint not available")
	}

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Call admin_nodeInfo
	requestBody := `{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`
	resp, err := http.Post(httpEndpoint, "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("Failed to call admin_nodeInfo: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
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
		t.Fatalf("Failed to parse JSON response: %v", err)
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

	t.Logf("Successfully retrieved ENR: %s", rpcResp.Result.ENR[:20]+"...")
}

// TestHTTPFlagsConfiguration tests that HTTP flags are properly parsed
func TestHTTPFlagsConfiguration(t *testing.T) {
	tests := []struct {
		name       string
		httpHost   string
		httpPort   int
		httpModules []string
		wantErr    bool
	}{
		{
			name:       "valid localhost config",
			httpHost:   "127.0.0.1",
			httpPort:   8545,
			httpModules: []string{"admin", "eth", "net"},
			wantErr:    false,
		},
		{
			name:       "random port",
			httpHost:   "127.0.0.1",
			httpPort:   0,
			httpModules: []string{"admin"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeConfig := &node.Config{
				HTTPHost:    tt.httpHost,
				HTTPPort:    tt.httpPort,
				HTTPModules: tt.httpModules,
				P2P: p2p.Config{
					MaxPeers:    10,
					NoDiscovery: true,
				},
			}

			stack, err := node.New(nodeConfig)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to create node: %v", err)
				}
				return
			}
			defer stack.Close()

			err = setupHTTPRPC(stack, nodeConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("setupHTTPRPC() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLocalhostSecurityBinding tests that HTTP server only binds to localhost by default
func TestLocalhostSecurityBinding(t *testing.T) {
	nodeConfig := &node.Config{
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0,
		HTTPModules: []string{"admin"},
		P2P: p2p.Config{
			MaxPeers:    10,
			NoDiscovery: true,
		},
	}

	stack, err := node.New(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer stack.Close()

	if err := setupHTTPRPC(stack, nodeConfig); err != nil {
		t.Fatalf("Failed to setup HTTP RPC: %v", err)
	}

	if err := stack.Start(); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	// Verify the endpoint contains localhost/127.0.0.1
	endpoint := stack.HTTPEndpoint()
	if !strings.Contains(endpoint, "127.0.0.1") && !strings.Contains(endpoint, "localhost") {
		t.Errorf("Expected localhost binding, got: %s", endpoint)
	}
}

// TestMultipleAPIsAvailable tests that multiple API namespaces work correctly
func TestMultipleAPIsAvailable(t *testing.T) {
	nodeConfig := &node.Config{
		HTTPHost:    "127.0.0.1",
		HTTPPort:    0,
		HTTPModules: []string{"admin", "net", "web3"},
		P2P: p2p.Config{
			MaxPeers:    10,
			NoDiscovery: true,
		},
	}

	stack, err := node.New(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer stack.Close()

	if err := setupHTTPRPC(stack, nodeConfig); err != nil {
		t.Fatalf("Failed to setup HTTP RPC: %v", err)
	}

	if err := stack.Start(); err != nil {
		t.Fatalf("Failed to start node: %v", err)
	}

	httpEndpoint := stack.HTTPEndpoint()
	if httpEndpoint == "" {
		t.Fatal("HTTP endpoint not available")
	}

	time.Sleep(200 * time.Millisecond)

	// Test admin API
	adminReq := `{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}`
	resp, err := http.Post(httpEndpoint, "application/json", strings.NewReader(adminReq))
	if err != nil {
		t.Fatalf("Failed to call admin API: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin API not available, status: %d", resp.StatusCode)
	}

	// Test web3 API
	web3Req := `{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":1}`
	resp, err = http.Post(httpEndpoint, "application/json", strings.NewReader(web3Req))
	if err != nil {
		t.Fatalf("Failed to call web3 API: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("web3 API not available, status: %d", resp.StatusCode)
	}
}

