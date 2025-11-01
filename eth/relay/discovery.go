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
	"context"

	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	discoveryPrefetchBuffer = 32
	maxParallelENRRequests  = 16
)

// enrEntry is the ENR entry for relay mode.
type enrEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124
	Rest   []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e enrEntry) ENRKey() string {
	return "eth"
}

// setupRelayDiscovery configures peer discovery for relay mode.
// It uses the same discovery mechanisms as full nodes but with relay-specific filters.
// The p2p server already manages discv4/discv5, we just add filtered iterators.
func setupRelayDiscovery(
	p2pServer *p2p.Server,
	discmix *enode.FairMix,
	config *Config,
) error {
	// Add eth nodes from DNS discovery (same as original)
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	if len(config.EthDiscoveryURLs) > 0 {
		iter, err := dnsclient.NewIterator(config.EthDiscoveryURLs...)
		if err != nil {
			return err
		}
		iter = enode.Filter(iter, newRelayNodeFilter(config))
		discmix.AddSource(iter)
	}

	// Add snap nodes from DNS (same as original)
	if len(config.SnapDiscoveryURLs) > 0 {
		iter, err := dnsclient.NewIterator(config.SnapDiscoveryURLs...)
		if err != nil {
			return err
		}
		discmix.AddSource(iter)
	}

	// Add DHT nodes from discv4 - USE RELAY FILTER
	if p2pServer.DiscoveryV4() != nil {
		iter := p2pServer.DiscoveryV4().RandomNodes()
		resolverFunc := func(ctx context.Context, enr *enode.Node) *enode.Node {
			nn, _ := p2pServer.DiscoveryV4().RequestENR(enr)
			return nn
		}
		iter = enode.AsyncFilter(iter, resolverFunc, maxParallelENRRequests)
		// KEY CHANGE: Use relay filter instead of blockchain-based filter
		iter = enode.Filter(iter, newRelayNodeFilter(config))
		iter = enode.NewBufferIter(iter, discoveryPrefetchBuffer)
		discmix.AddSource(iter)
	}

	// Add DHT nodes from discv5 - USE RELAY FILTER
	if p2pServer.DiscoveryV5() != nil {
		filter := newRelayNodeFilter(config) // Relay filter instead of eth.NewNodeFilter
		iter := enode.Filter(p2pServer.DiscoveryV5().RandomNodes(), filter)
		iter = enode.NewBufferIter(iter, discoveryPrefetchBuffer)
		discmix.AddSource(iter)
	}

	// Add DNS discovery if available for the network
	if url := params.KnownDNSNetwork(config.GenesisHash, "all"); url != "" {
		log.Info("Adding DNS discovery source", "url", url)
		dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
		iter, err := dnsclient.NewIterator(url)
		if err == nil {
			iter = enode.Filter(iter, newRelayNodeFilter(config))
			discmix.AddSource(iter)
		}
	}

	return nil
}

// StartRelayENRUpdater updates ENR without blockchain dependency.
// Must be called after the node has started (LocalNode() is available).
func StartRelayENRUpdater(localNode *enode.LocalNode, config *Config) {
	if localNode == nil {
		return // LocalNode not available yet
	}
	// Set ENR entry with hard-coded fork ID
	entry := &enrEntry{
		ForkID: config.ForkID,
	}
	localNode.Set(entry)
}

// newRelayNodeFilter creates a node filter for relay mode.
// Replaces eth.NewNodeFilter(blockchain) with hard-coded chain info.
func newRelayNodeFilter(config *Config) func(*enode.Node) bool {
	return func(node *enode.Node) bool {
		// Filter based on hard-coded network ID and fork ID
		// Check ENR entries for eth network compatibility
		var entry enrEntry
		if err := node.Load(&entry); err != nil {
			return false // No eth entry means not an eth node
		}
		// Simple fork ID compatibility check - accept if fork hash matches
		// In relay mode, we're less strict and accept compatible fork IDs
		if entry.ForkID.Hash == config.ForkID.Hash {
			return true
		}
		// Also accept nodes with compatible forks (simplified check)
		// This allows relay to connect to nodes on the same network even if fork state differs slightly
		return entry.ForkID.Next == 0 || config.ForkID.Next == 0 || 
			entry.ForkID.Next >= config.ForkID.Next
	}
}

// MakeRelayDialCandidates creates a discovery iterator for relay mode.
func MakeRelayDialCandidates(
	p2pServer *p2p.Server,
	config *Config,
) enode.Iterator {
	discmix := enode.NewFairMix(0)
	if err := setupRelayDiscovery(p2pServer, discmix, config); err != nil {
		return enode.Filter(nil, func(*enode.Node) bool { return false })
	}
	return discmix
}

