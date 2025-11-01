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
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/params"
)

// Config contains the configuration for a relay node.
type Config struct {
	// Network configuration
	NetworkID   uint64            // Ethereum network ID (1=Mainnet, 17000=Holesky)
	GenesisHash common.Hash       // Hard-coded genesis block hash
	ChainConfig *params.ChainConfig // Hard-coded chain configuration
	ForkID      forkid.ID         // Pre-computed fork ID
	
	// Block range tracking (for handshake)
	BlockRange BlockRange // Initial block range
	
	// Discovery configuration
	EthDiscoveryURLs  []string // DNS discovery URLs for eth protocol
	SnapDiscoveryURLs []string // DNS discovery URLs for snap protocol
}

// BlockRange represents the available block range for the relay.
type BlockRange struct {
	EarliestBlock   uint64
	LatestBlock     uint64
	LatestBlockHash common.Hash
}

// DiscoveryConfig wraps discovery-related configuration.
type DiscoveryConfig struct {
	EthDiscoveryURLs  []string
	SnapDiscoveryURLs []string
}

