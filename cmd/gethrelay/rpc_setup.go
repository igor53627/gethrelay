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
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

// ethAPI provides the eth_sendRawTransaction method
type ethAPI struct {
	upstreamURL string
	httpClient  *http.Client
	log         log.Logger
}

// SendRawTransaction handles eth_sendRawTransaction requests
// For relay nodes, we validate the transaction locally then forward to upstream
func (api *ethAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	// Validate the transaction
	tx := new(types.Transaction)
	if err := tx.UnmarshalBinary(encodedTx); err != nil {
		return common.Hash{}, fmt.Errorf("invalid transaction: %v", err)
	}

	api.log.Info("Received raw transaction", "hash", tx.Hash().Hex())

	// Forward to upstream RPC endpoint
	body := fmt.Sprintf(`{"jsonrpc":"2.0","method":"eth_sendRawTransaction","params":["0x%s"],"id":1}`, hex.EncodeToString(encodedTx))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api.upstreamURL, strings.NewReader(body))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to create upstream request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := api.httpClient.Do(req)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to forward to upstream: %v", err)
	}
	defer resp.Body.Close()

	// Parse upstream response
	var upstreamResp struct {
		JSONRPC string       `json:"jsonrpc"`
		ID      int          `json:"id"`
		Result  *common.Hash `json:"result,omitempty"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&upstreamResp); err != nil {
		return common.Hash{}, fmt.Errorf("failed to decode upstream response: %v", err)
	}

	if upstreamResp.Error != nil {
		return common.Hash{}, fmt.Errorf("upstream RPC error: %s (code: %d)", upstreamResp.Error.Message, upstreamResp.Error.Code)
	}

	if upstreamResp.Result == nil {
		return common.Hash{}, fmt.Errorf("upstream returned no result")
	}

	return *upstreamResp.Result, nil
}

// setupRPCProxy configures the RPC proxy for the node
// It creates a standalone HTTP server on the specified address and port
func setupRPCProxy(stack *node.Node, upstreamURL string, addr string, port int) error {
	// Create a minimal RPC server for local methods
	localServer := rpc.NewServer()
	
	// Create eth API
	ethAPI := &ethAPI{
		upstreamURL: upstreamURL,
		httpClient:  &http.Client{},
		log:         log.New("module", "ethapi"),
	}
	
	// Register the eth API
	if err := localServer.RegisterName("eth", ethAPI); err != nil {
		return fmt.Errorf("failed to register eth API: %v", err)
	}
	
	// Create the proxy handler
	proxy := newRPCProxy(upstreamURL, localServer)

	// Start HTTP server on configured address and port
	listenAddr := fmt.Sprintf("%s:%d", addr, port)
	go func() {
		server := &http.Server{
			Addr:    listenAddr,
			Handler: proxy,
		}

		log.Info("Starting JSON-RPC proxy server", "upstream", upstreamURL, "addr", addr, "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("RPC proxy server error", "err", err)
		}
	}()

	return nil
}

