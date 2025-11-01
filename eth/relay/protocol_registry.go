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
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// ProtocolRegistry allows registering protocols dynamically to avoid import cycles.
var protocolRegistry func(*Backend, uint64, enode.Iterator) []p2p.Protocol

// RegisterProtocolRegistry sets the protocol registration function.
// This should be called from a package that can import eth/protocols/eth.
func RegisterProtocolRegistry(fn func(*Backend, uint64, enode.Iterator) []p2p.Protocol) {
	protocolRegistry = fn
}

// RegisterProtocols registers protocols using the registered registry function.
func (r *Relay) RegisterProtocols(stack *node.Node) error {
	if protocolRegistry == nil {
		return nil // No registry set, skip protocol registration
	}
	discCandidates := MakeRelayDialCandidates(r.p2pServer, r.config)
	protocols := protocolRegistry(r.backend, r.networkID, discCandidates)
	stack.RegisterProtocols(protocols)
	return nil
}

