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

package eth

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/relay"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// RelayBackend implements eth.Backend for relay mode.
type RelayBackend struct {
	relay *relay.Backend
}

// NewRelayBackend creates a new relay backend.
func NewRelayBackend(r *relay.Backend) *RelayBackend {
	return &RelayBackend{relay: r}
}

// SetBackend sets the relay backend (for delayed initialization).
func (rb *RelayBackend) SetBackend(r *relay.Backend) {
	rb.relay = r
}

// RegisterRelayProtocols implements relay.ProtocolRegistrar
func (rb *RelayBackend) RegisterRelayProtocols(backend *relay.Backend, networkID uint64, discCandidates enode.Iterator) []p2p.Protocol {
	return MakeRelayProtocols(rb, networkID, discCandidates)
}

// Chain returns nil - relay doesn't have a blockchain.
func (rb *RelayBackend) Chain() *core.BlockChain {
	return nil
}

// TxPool returns nil - relay doesn't have a transaction pool.
func (rb *RelayBackend) TxPool() TxPool {
	return nil
}

// AcceptTxs returns false - relay doesn't accept transactions.
func (rb *RelayBackend) AcceptTxs() bool {
	return false
}

// RunPeer is invoked when a peer joins on the eth protocol.
func (rb *RelayBackend) RunPeer(peer *Peer, handler Handler) error {
	// Execute relay handshake
	blockRange := rb.relay.GetBlockRange()
		rangePacket := BlockRangeUpdatePacket{
		EarliestBlock:   blockRange.EarliestBlock,
		LatestBlock:     blockRange.LatestBlock,
		LatestBlockHash: blockRange.LatestBlockHash,
	}
	
	// Use relay handshake
	if err := peer.RelayHandshake(
		rb.relay.GetNetworkID(),
		rb.relay.GetGenesisHash(),
		rb.relay.GetChainConfig(),
		rb.relay.GetForkID(),
		rangePacket,
	); err != nil {
		return err
	}
	
	// Run the handler
	return handler(peer)
}

// PeerInfo retrieves relay peer information.
func (rb *RelayBackend) PeerInfo(id enode.ID) interface{} {
	// Return minimal peer info for relay mode
	return struct {
		Network uint64 `json:"network"`
		Relay   bool   `json:"relay"`
	}{
		Network: rb.relay.GetNetworkID(),
		Relay:   true,
	}
}

// Handle is invoked when a data packet is received from the remote peer.
func (rb *RelayBackend) Handle(peer *Peer, packet Packet) error {
	// Relay mode: packets are handled by the protocol handlers
	// This method is called for unhandled packets - in relay mode we forward them
	// TODO: Implement packet forwarding through message router
	return nil
}

// IsRelay returns true - this is a relay backend.
func (rb *RelayBackend) IsRelay() bool {
	return true
}

// GetRelayBackend returns the underlying relay backend.
func (rb *RelayBackend) GetRelayBackend() *relay.Backend {
	return rb.relay
}

