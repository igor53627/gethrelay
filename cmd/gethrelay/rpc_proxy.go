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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

// rpcProxy wraps an RPC server and proxies requests to an upstream endpoint
// unless the method is eth_sendRawTransaction, which is handled locally.
type rpcProxy struct {
	localServer *rpc.Server
	upstreamURL string
	httpClient  *http.Client
	log          log.Logger
	mu           sync.RWMutex
}

// newRPCProxy creates a new RPC proxy handler.
func newRPCProxy(upstreamURL string, localServer *rpc.Server) *rpcProxy {
	return &rpcProxy{
		localServer: localServer,
		upstreamURL: upstreamURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		log:         log.New("module", "rpcproxy"),
	}
}

// setUpstreamURL updates the upstream URL.
func (p *rpcProxy) setUpstreamURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.upstreamURL = url
	p.log.Info("Updated upstream RPC URL", "url", url)
}

// ServeHTTP implements http.Handler and proxies requests.
func (p *rpcProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Parse the JSON-RPC request(s)
	var requests []jsonrpcMessage
	var singleRequest jsonrpcMessage

	if err := json.Unmarshal(body, &singleRequest); err == nil {
		// Single request
		if singleRequest.isCall() || singleRequest.isNotification() {
			requests = []jsonrpcMessage{singleRequest}
		}
	} else {
		// Try as batch
		if err := json.Unmarshal(body, &requests); err != nil {
			http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
			return
		}
	}

	if len(requests) == 0 {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	// Check if any request should be handled locally
	localRequests := make([]jsonrpcMessage, 0)
	forwardRequests := make([]jsonrpcMessage, 0)
	localIndices := make(map[int]bool)

	for i, req := range requests {
		if req.Method == "eth_sendRawTransaction" {
			localRequests = append(localRequests, req)
			localIndices[i] = true
		} else {
			forwardRequests = append(forwardRequests, req)
		}
	}

	// If all requests should be forwarded, proxy to upstream
	if len(localRequests) == 0 {
		p.forwardToUpstream(w, r, body)
		return
	}

	// If all requests should be handled locally, use local server
	if len(forwardRequests) == 0 {
		p.localServer.ServeHTTP(w, r)
		return
	}

	// Mixed case: handle some locally, forward others
	// For simplicity, forward all to upstream and let it handle non-eth_sendRawTransaction
	// Then handle eth_sendRawTransaction locally and merge responses
	// This is complex for batches, so we'll forward everything and intercept locally
	// For now, let's handle batches by splitting them
	p.handleMixedRequest(w, r, localRequests, forwardRequests)
}

func (p *rpcProxy) handleMixedRequest(w http.ResponseWriter, r *http.Request, local, forward []jsonrpcMessage) {
	// For mixed requests, we need to handle both locally and forward to upstream
	// and merge responses properly by matching request IDs
	
	// Create a map to track which responses belong to which requests
	responseMap := make(map[string]*jsonrpcMessage)
	
	// Handle local requests
	for _, req := range local {
		resp := p.handleLocalRequest(r.Context(), req)
		// Use request ID as key for merging
		key := string(req.ID)
		responseMap[key] = resp
	}
	
	// Forward non-local requests
	if len(forward) > 0 {
		forwardBody, _ := json.Marshal(forward)
		forwardReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, p.getUpstreamURL(), bytes.NewReader(forwardBody))
		if err != nil {
			p.log.Error("Failed to create upstream request", "err", err)
			http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
			return
		}
		forwardReq.Header.Set("Content-Type", "application/json")

		upstreamResp, err := p.httpClient.Do(forwardReq)
		if err != nil {
			p.log.Error("Failed to forward request to upstream", "err", err)
			http.Error(w, "Failed to forward request", http.StatusBadGateway)
			return
		}
		defer upstreamResp.Body.Close()

		// Decode upstream response(s)
		var forwardResponses []*jsonrpcMessage
		bodyBytes, _ := io.ReadAll(upstreamResp.Body)
		
		// Try batch first
		if err := json.Unmarshal(bodyBytes, &forwardResponses); err != nil {
			// Try single response
			var singleResp jsonrpcMessage
			if err := json.Unmarshal(bodyBytes, &singleResp); err == nil {
				forwardResponses = []*jsonrpcMessage{&singleResp}
			} else {
				p.log.Error("Failed to decode upstream response", "err", err)
				http.Error(w, "Failed to decode upstream response", http.StatusInternalServerError)
				return
			}
		}
		
		// Map forward responses
		for _, resp := range forwardResponses {
			key := string(resp.ID)
			responseMap[key] = resp
		}
	}
	
	// Reconstruct response in original order
	var allResponses []*jsonrpcMessage
	for _, req := range local {
		key := string(req.ID)
		if resp, ok := responseMap[key]; ok {
			allResponses = append(allResponses, resp)
		}
	}
	for _, req := range forward {
		key := string(req.ID)
		if resp, ok := responseMap[key]; ok {
			allResponses = append(allResponses, resp)
		}
	}
	
	// Send response
	if len(allResponses) == 1 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allResponses[0])
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(allResponses)
	}
}

func (p *rpcProxy) handleLocalRequest(ctx context.Context, req jsonrpcMessage) *jsonrpcMessage {
	// For local requests (like eth_sendRawTransaction), we need to call the local RPC server
	// Create a temporary HTTP request/response to invoke the local server
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		return req.errorResponse(fmt.Errorf("failed to create request: %v", err))
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Create a response recorder to capture the local server's response
	recorder := &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
	}

	// Call the local RPC server
	p.localServer.ServeHTTP(recorder, httpReq)

	// Parse the response
	var resp jsonrpcMessage
	if err := json.NewDecoder(recorder.body).Decode(&resp); err != nil {
		return req.errorResponse(fmt.Errorf("failed to decode local response: %v", err))
	}
	return &resp
}

// responseRecorder implements http.ResponseWriter for capturing responses
type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	code   int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.code = statusCode
}

func (p *rpcProxy) forwardToUpstream(w http.ResponseWriter, r *http.Request, body []byte) {
	upstreamURL := p.getUpstreamURL()
	
	forwardReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		p.log.Error("Failed to create upstream request", "err", err)
		http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			forwardReq.Header.Add(key, value)
		}
	}
	forwardReq.Header.Set("Content-Type", "application/json")

	upstreamResp, err := p.httpClient.Do(forwardReq)
	if err != nil {
		p.log.Error("Failed to forward request to upstream", "err", err, "upstream", upstreamURL)
		http.Error(w, "Failed to forward request to upstream", http.StatusBadGateway)
		return
	}
	defer upstreamResp.Body.Close()

	// Copy response headers
	for key, values := range upstreamResp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(upstreamResp.StatusCode)

	// Copy response body
	io.Copy(w, upstreamResp.Body)
}

func (p *rpcProxy) getUpstreamURL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.upstreamURL
}

// jsonrpcMessage is a copy of rpc.jsonrpcMessage for use in this package
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) isNotification() bool {
	return msg.Version == "2.0" && msg.ID == nil && msg.Method != ""
}

func (msg *jsonrpcMessage) isCall() bool {
	return msg.Version == "2.0" && len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '[' && msg.Method != ""
}

func (msg *jsonrpcMessage) errorResponse(err error) *jsonrpcMessage {
	code := -32000
	message := err.Error()
	if rpcErr, ok := err.(rpc.Error); ok {
		code = rpcErr.ErrorCode()
	}
	return &jsonrpcMessage{
		Version: "2.0",
		ID:      msg.ID,
		Error: &jsonError{
			Code:    code,
			Message: message,
		},
	}
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

