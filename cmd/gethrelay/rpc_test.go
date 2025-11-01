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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

var testLogger = log.New("module", "test")

// createTestTransaction creates a minimal valid transaction for testing
func createTestTransaction() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}

	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	tx := types.NewTransaction(
		0,
		to,
		big.NewInt(1000),
		21000,
		big.NewInt(params.InitialBaseFee),
		nil,
	)

	signer := types.HomesteadSigner{}
	signedTx, err := types.SignTx(tx, signer, key)
	if err != nil {
		return "", err
	}

	data, err := signedTx.MarshalBinary()
	if err != nil {
		return "", err
	}

	return "0x" + hex.EncodeToString(data), nil
}

// TestRPCProxy_SendRawTransaction tests that eth_sendRawTransaction is handled locally
func TestRPCProxy_SendRawTransaction(t *testing.T) {
	// Create a valid test transaction
	txHex, err := createTestTransaction()
	if err != nil {
		t.Fatalf("failed to create test transaction: %v", err)
	}

	// Create a mock upstream server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		
		if req.Method == "eth_sendRawTransaction" {
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result":  "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			}
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer upstreamServer.Close()

	// Create local RPC server
	localServer := rpc.NewServer()
	ethAPI := &ethAPI{
		upstreamURL: upstreamServer.URL,
		httpClient:  &http.Client{},
		log:         testLogger,
	}
	if err := localServer.RegisterName("eth", ethAPI); err != nil {
		t.Fatalf("failed to register eth API: %v", err)
	}

	// Create proxy
	proxy := newRPCProxy(upstreamServer.URL, localServer)

	// Test request with valid transaction
	reqBody := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["%s"],"id":1}`, txHex)
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["error"] != nil {
		t.Errorf("unexpected error: %v", resp["error"])
	}
}

// TestRPCProxy_ProxiedRequest tests that non-eth_sendRawTransaction requests are proxied
func TestRPCProxy_ProxiedRequest(t *testing.T) {
	// Create a mock upstream server
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x1234",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer upstreamServer.Close()

	localServer := rpc.NewServer()
	proxy := newRPCProxy(upstreamServer.URL, localServer)

	// Test request for eth_blockNumber (should be proxied)
	reqBody := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["result"] == nil {
		t.Error("expected result in response")
	}
}

// TestRPCProxy_BatchRequest tests batch requests with mixed local and proxied methods
func TestRPCProxy_BatchRequest(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		var requests []map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requests); err != nil {
			// Might be a single request, try that
			var singleReq map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &singleReq); err == nil {
				requests = []map[string]interface{}{singleReq}
			}
		}
		
		var responses []map[string]interface{}
		for _, req := range requests {
			if req["method"] == "eth_blockNumber" {
				responses = append(responses, map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      req["id"],
					"result":  "0x1234",
				})
			}
		}
		json.NewEncoder(w).Encode(responses)
	}))
	defer upstreamServer.Close()

	localServer := rpc.NewServer()
	ethAPI := &ethAPI{
		upstreamURL: upstreamServer.URL,
		httpClient:  &http.Client{},
		log:         testLogger,
	}
	if err := localServer.RegisterName("eth", ethAPI); err != nil {
		t.Fatalf("failed to register eth API: %v", err)
	}
	proxy := newRPCProxy(upstreamServer.URL, localServer)

	// Create a valid test transaction for the batch request
	txHex, err := createTestTransaction()
	if err != nil {
		t.Fatalf("failed to create test transaction: %v", err)
	}

	// Batch request with both local and proxied methods
	reqBody := fmt.Sprintf(`[
		{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["%s"],"id":1},
		{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}
	]`, txHex)
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var responses []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&responses); err != nil {
		t.Fatalf("failed to decode response: %v, body: %s", err, w.Body.String())
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
}

