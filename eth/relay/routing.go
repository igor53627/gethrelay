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

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

var (
	ErrPeerDisconnected = errors.New("peer disconnected")
	ErrNoPeers          = errors.New("no peers available")
)

// QueuedMessage represents a message waiting to be forwarded.
type QueuedMessage struct {
	MsgCode   uint64
	Payload   []byte
	ToPeer    enode.ID
	RequestID uint64
}

// OrderedQueue ensures single-threaded message ordering per peer.
type OrderedQueue struct {
	peerID   enode.ID
	messages chan *QueuedMessage
	quit     chan struct{}
	wg       sync.WaitGroup
	backend  *Backend
}

// MessageRouter handles ordered message forwarding.
type MessageRouter struct {
	relay      *Backend
	peerQueues map[enode.ID]*OrderedQueue
	queuesLock sync.RWMutex
}

// NewMessageRouter creates a new message router.
func NewMessageRouter(relay *Backend) *MessageRouter {
	return &MessageRouter{
		relay:      relay,
		peerQueues: make(map[enode.ID]*OrderedQueue),
	}
}

// ForwardMessage adds message to peer's ordered queue and forwards to all other peers.
func (mr *MessageRouter) ForwardMessage(from enode.ID, msgCode uint64, payload []byte) error {
	mr.relay.peersLock.RLock()
	defer mr.relay.peersLock.RUnlock()

	allPeers := mr.relay.peers.All()
	targetCount := len(allPeers)
	if targetCount == 0 {
		return nil // No peers to forward to
	}

	// Log message being relayed
	log.Trace("Relaying message", 
		"from", from.String()[:16]+"...",
		"code", msgCode,
		"codeName", msgCodeToString(msgCode),
		"size", len(payload),
		"targets", targetCount)

	// Get ordered queue for sender (ensures ordering)
	queue := mr.getQueue(from)

	// Broadcast to all other peers
	for _, peer := range allPeers {
		if peer.ID != from {
			select {
			case queue.messages <- &QueuedMessage{
				MsgCode: msgCode,
				Payload: payload,
				ToPeer:  peer.ID,
			}:
			case <-queue.quit:
				return ErrPeerDisconnected
			}
		}
	}
	return nil
}

// getQueue returns or creates ordered queue for peer.
func (mr *MessageRouter) getQueue(peerID enode.ID) *OrderedQueue {
	mr.queuesLock.Lock()
	defer mr.queuesLock.Unlock()

	if queue, exists := mr.peerQueues[peerID]; exists {
		return queue
	}

	queue := &OrderedQueue{
		peerID:   peerID,
		messages: make(chan *QueuedMessage, 100), // Buffered channel
		quit:     make(chan struct{}),
		backend:  mr.relay,
	}
	mr.peerQueues[peerID] = queue

	// Start single-threaded sender goroutine
	queue.wg.Add(1)
	go queue.processMessages()

	return queue
}

// processMessages runs in single goroutine per peer - ensures ordering.
func (oq *OrderedQueue) processMessages() {
	defer oq.wg.Done()

	for {
		select {
		case msg := <-oq.messages:
			// Send to target peer
			log.Trace("Forwarding message to peer",
				"from", oq.peerID.String()[:16]+"...",
				"to", msg.ToPeer.String()[:16]+"...",
				"code", msg.MsgCode,
				"codeName", msgCodeToString(msg.MsgCode),
				"size", len(msg.Payload))
			oq.backend.sendToPeer(msg.ToPeer, msg.MsgCode, msg.Payload)
		case <-oq.quit:
			return
		}
	}
}


// Stop stops the queue.
func (oq *OrderedQueue) Stop() {
	close(oq.quit)
	oq.wg.Wait()
}

// Stop stops all queues.
func (mr *MessageRouter) Stop() {
	mr.queuesLock.Lock()
	defer mr.queuesLock.Unlock()

	for _, queue := range mr.peerQueues {
		queue.Stop()
	}
	mr.peerQueues = make(map[enode.ID]*OrderedQueue)
}

