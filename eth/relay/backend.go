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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
)

// Backend is the relay backend that manages peer connections and message routing.
type Backend struct {
	config *Config
	p2pServer *p2p.Server
	networkID uint64
	genesisHash common.Hash
	chainConfig *params.ChainConfig
	forkID forkid.ID
	blockRange BlockRange

	// Peer management
	peers *RelayPeerSet
	peersLock sync.RWMutex

	// Message relay queue
	relayQueue chan *RelayMessage
	quit chan struct{}
}

// RelayMessage represents a message to be forwarded.
type RelayMessage struct {
	From enode.ID
	ToPeers []enode.ID // Empty = broadcast to all except sender
	MsgCode uint64
	Payload []byte
	RequestID uint64 // For request-response
}

// NewBackend creates a new relay backend.
func NewBackend(config *Config, p2pServer *p2p.Server) *Backend {
	return &Backend{
		config: config,
		p2pServer: p2pServer,
		networkID: config.NetworkID,
		genesisHash: config.GenesisHash,
		chainConfig: config.ChainConfig,
		forkID: config.ForkID,
		blockRange: config.BlockRange,
		peers: NewRelayPeerSet(),
		relayQueue: make(chan *RelayMessage, 1000),
		quit: make(chan struct{}),
	}
}

// GetNetworkID returns the network ID.
func (b *Backend) GetNetworkID() uint64 {
	return b.networkID
}

// GetGenesisHash returns the genesis hash.
func (b *Backend) GetGenesisHash() common.Hash {
	return b.genesisHash
}

// GetChainConfig returns the chain configuration.
func (b *Backend) GetChainConfig() *params.ChainConfig {
	return b.chainConfig
}

// GetForkID returns the fork ID.
func (b *Backend) GetForkID() forkid.ID {
	return b.forkID
}

// GetBlockRange returns the block range.
func (b *Backend) GetBlockRange() BlockRange {
	return b.blockRange
}

// sendToPeer sends a raw message to a peer via P2P.
// This will be connected to actual protocol handlers through the relay service.
func (b *Backend) sendToPeer(peerID enode.ID, msgCode uint64, payload []byte) error {
	// Queue message for forwarding through protocol handlers
	select {
	case b.relayQueue <- &RelayMessage{
		From:    peerID,
		MsgCode: msgCode,
		Payload: payload,
	}:
		return nil
	case <-b.quit:
		return ErrPeerDisconnected
	}
}

// GetRelayQueue returns the relay message queue (for use by relay service).
func (b *Backend) GetRelayQueue() <-chan *RelayMessage {
	return b.relayQueue
}

// Stop stops the backend.
func (b *Backend) Stop() {
	close(b.quit)
}

