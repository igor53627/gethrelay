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

package relay

import (
	"errors"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

var (
	ErrNoTargetPeer         = errors.New("no target peer available")
	ErrRequestTimeout       = errors.New("request timeout")
	ErrUnknownRequest       = errors.New("unknown request ID")
	ErrUnexpectedResponsePeer = errors.New("response from unexpected peer")
)

// PendingRequest represents a pending request-response pair.
type PendingRequest struct {
	RequestID    uint64
	FromPeer    enode.ID
	ToPeer      enode.ID
	MsgCode     uint64
	ResponseChan chan []byte
	Timeout     time.Time
}

// PeerSelector interface for choosing target peers.
type PeerSelector interface {
	SelectPeer(exclude enode.ID) enode.ID
}

// RoundRobinSelector implements round-robin peer selection.
type RoundRobinSelector struct {
	backend *Backend
	index   int
	lock    sync.Mutex
}

// NewRoundRobinSelector creates a new round-robin selector.
func NewRoundRobinSelector(backend *Backend) *RoundRobinSelector {
	return &RoundRobinSelector{
		backend: backend,
	}
}

// SelectPeer selects a peer using round-robin, excluding the given peer.
func (rrs *RoundRobinSelector) SelectPeer(exclude enode.ID) enode.ID {
	rrs.lock.Lock()
	defer rrs.lock.Unlock()

	rrs.backend.peersLock.RLock()
	peers := rrs.backend.peers.All()
	rrs.backend.peersLock.RUnlock()

	if len(peers) == 0 {
		return enode.ID{}
	}

	// Filter out excluded peer
	availablePeers := make([]enode.ID, 0, len(peers))
	for _, peer := range peers {
		if peer.ID != exclude {
			availablePeers = append(availablePeers, peer.ID)
		}
	}

	if len(availablePeers) == 0 {
		return enode.ID{} // No available peer
	}

	// Round-robin selection
	peer := availablePeers[rrs.index%len(availablePeers)]
	rrs.index++
	return peer
}

// RequestProxy handles request-response proxying.
type RequestProxy struct {
	backend        *Backend
	pendingRequests map[uint64]*PendingRequest
	requestLock    sync.RWMutex
	peerSelector   PeerSelector
	requestTimeout time.Duration
	cleanupTicker  *time.Ticker
	quit           chan struct{}
	wg             sync.WaitGroup
}

// NewRequestProxy creates a new request proxy.
func NewRequestProxy(backend *Backend, selector PeerSelector, timeout time.Duration) *RequestProxy {
	proxy := &RequestProxy{
		backend:        backend,
		pendingRequests: make(map[uint64]*PendingRequest),
		peerSelector:   selector,
		requestTimeout: timeout,
		quit:          make(chan struct{}),
	}

	// Start cleanup ticker to remove timed-out requests
	proxy.cleanupTicker = time.NewTicker(5 * time.Second)
	proxy.wg.Add(1)
	go proxy.cleanupLoop()

	return proxy
}

// cleanupLoop periodically cleans up timed-out requests.
func (rp *RequestProxy) cleanupLoop() {
	defer rp.wg.Done()

	for {
		select {
		case <-rp.cleanupTicker.C:
			rp.cleanupTimedOut()
		case <-rp.quit:
			return
		}
	}
}

// cleanupTimedOut removes timed-out requests.
func (rp *RequestProxy) cleanupTimedOut() {
	rp.requestLock.Lock()
	defer rp.requestLock.Unlock()

	now := time.Now()
	for id, pending := range rp.pendingRequests {
		if now.After(pending.Timeout) {
			close(pending.ResponseChan)
			delete(rp.pendingRequests, id)
		}
	}
}

// ProxyRequest forwards request and awaits response.
func (rp *RequestProxy) ProxyRequest(fromPeer enode.ID, msgCode uint64, requestID uint64, payload []byte) error {
	// Select target peer using round-robin
	targetPeer := rp.peerSelector.SelectPeer(fromPeer)
	if targetPeer == (enode.ID{}) {
		log.Trace("No target peer available for request proxying",
			"from", fromPeer.String()[:16]+"...",
			"code", msgCodeToString(msgCode))
		return ErrNoTargetPeer
	}

	log.Trace("Proxying request",
		"from", fromPeer.String()[:16]+"...",
		"to", targetPeer.String()[:16]+"...",
		"code", msgCodeToString(msgCode),
		"requestID", requestID,
		"size", len(payload))

	// Store pending request
	pending := &PendingRequest{
		RequestID:     requestID,
		FromPeer:      fromPeer,
		ToPeer:        targetPeer,
		MsgCode:       msgCode,
		ResponseChan:  make(chan []byte, 1),
		Timeout:       time.Now().Add(rp.requestTimeout),
	}

	rp.requestLock.Lock()
	rp.pendingRequests[requestID] = pending
	rp.requestLock.Unlock()

	// Forward request to target
	err := rp.backend.sendToPeer(targetPeer, msgCode, payload)
	if err != nil {
		rp.removePendingRequest(requestID)
		log.Debug("Failed to send proxied request", "err", err)
		return err
	}

	// Wait for response (with timeout)
	select {
	case response := <-pending.ResponseChan:
		if response == nil {
			log.Debug("Proxied request timed out",
				"from", fromPeer.String()[:16]+"...",
				"to", targetPeer.String()[:16]+"...",
				"code", msgCodeToString(msgCode),
				"requestID", requestID)
			return ErrRequestTimeout
		}
		// Forward response back to original requester
		responseMsgCode := getResponseMsgCode(msgCode)
		log.Trace("Proxying response back",
			"to", fromPeer.String()[:16]+"...",
			"code", msgCodeToString(responseMsgCode),
			"requestID", requestID,
			"size", len(response))
		return rp.backend.sendToPeer(fromPeer, responseMsgCode, response)
	case <-time.After(rp.requestTimeout):
		rp.removePendingRequest(requestID)
		log.Debug("Proxied request timed out waiting for response",
			"from", fromPeer.String()[:16]+"...",
			"to", targetPeer.String()[:16]+"...",
			"code", msgCodeToString(msgCode),
			"requestID", requestID)
		return ErrRequestTimeout
	}
}

// HandleResponse processes response from target peer.
func (rp *RequestProxy) HandleResponse(fromPeer enode.ID, msgCode uint64, requestID uint64, payload []byte) error {
	rp.requestLock.Lock()
	pending, exists := rp.pendingRequests[requestID]
	rp.requestLock.Unlock()

	if !exists {
		log.Trace("Received response for unknown request",
			"from", fromPeer.String()[:16]+"...",
			"code", msgCodeToString(msgCode),
			"requestID", requestID)
		return ErrUnknownRequest
	}

	// Verify response came from expected peer
	if pending.ToPeer != fromPeer {
		log.Debug("Response from unexpected peer",
			"expected", pending.ToPeer.String()[:16]+"...",
			"got", fromPeer.String()[:16]+"...",
			"requestID", requestID)
		return ErrUnexpectedResponsePeer
	}

	log.Trace("Received proxied response",
		"from", fromPeer.String()[:16]+"...",
		"code", msgCodeToString(msgCode),
		"requestID", requestID,
		"size", len(payload))

	// Send response to original requester
	select {
	case pending.ResponseChan <- payload:
		log.Trace("Delivered proxied response to requester",
			"requestID", requestID,
			"from", fromPeer.String()[:16]+"...",
			"to", pending.FromPeer.String()[:16]+"...")
	default:
		// Channel already closed or full
		log.Debug("Failed to deliver proxied response, channel closed or full", "requestID", requestID)
	}

	rp.removePendingRequest(requestID)
	return nil
}

// removePendingRequest removes a pending request.
func (rp *RequestProxy) removePendingRequest(requestID uint64) {
	rp.requestLock.Lock()
	defer rp.requestLock.Unlock()

	if pending, exists := rp.pendingRequests[requestID]; exists {
		close(pending.ResponseChan)
		delete(rp.pendingRequests, requestID)
	}
}

// Stop stops the proxy.
func (rp *RequestProxy) Stop() {
	close(rp.quit)
	rp.cleanupTicker.Stop()
	rp.wg.Wait()

	// Clean up all pending requests
	rp.requestLock.Lock()
	for _, pending := range rp.pendingRequests {
		close(pending.ResponseChan)
	}
	rp.pendingRequests = make(map[uint64]*PendingRequest)
	rp.requestLock.Unlock()
}

// getResponseMsgCode maps request message codes to response message codes.
func getResponseMsgCode(requestCode uint64) uint64 {
	switch requestCode {
	case 0x03: // GetBlockHeadersMsg
		return 0x04 // BlockHeadersMsg
	case 0x05: // GetBlockBodiesMsg
		return 0x06 // BlockBodiesMsg
	case 0x0f: // GetReceiptsMsg
		return 0x10 // ReceiptsMsg
	case 0x09: // GetPooledTransactionsMsg
		return 0x0a // PooledTransactionsMsg
	default:
		return requestCode // Default to same code
	}
}

