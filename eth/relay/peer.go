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
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// RelayPeer represents a peer connection in relay mode.
type RelayPeer struct {
	ID       enode.ID
	Peer     *p2p.Peer
	Version  uint
	Inbound  bool
	AddedAt  time.Time
	connLock sync.RWMutex
}

// RelayPeerSet manages the set of relay peers.
type RelayPeerSet struct {
	peers map[enode.ID]*RelayPeer
	lock  sync.RWMutex
}

// NewRelayPeerSet creates a new peer set.
func NewRelayPeerSet() *RelayPeerSet {
	return &RelayPeerSet{
		peers: make(map[enode.ID]*RelayPeer),
	}
}

// Add adds a peer to the set.
func (ps *RelayPeerSet) Add(peer *RelayPeer) bool {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, exists := ps.peers[peer.ID]; exists {
		return false
	}
	ps.peers[peer.ID] = peer
	return true
}

// Remove removes a peer from the set.
func (ps *RelayPeerSet) Remove(id enode.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	delete(ps.peers, id)
}

// Get retrieves a peer by ID.
func (ps *RelayPeerSet) Get(id enode.ID) *RelayPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.peers[id]
}

// All returns all peers.
func (ps *RelayPeerSet) All() []*RelayPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	peers := make([]*RelayPeer, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peers = append(peers, peer)
	}
	return peers
}

// IDs returns all peer IDs.
func (ps *RelayPeerSet) IDs() []enode.ID {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	ids := make([]enode.ID, 0, len(ps.peers))
	for id := range ps.peers {
		ids = append(ids, id)
	}
	return ids
}

// Len returns the number of peers.
func (ps *RelayPeerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return len(ps.peers)
}

