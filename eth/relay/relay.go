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

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// ProtocolRegistrar is an interface for registering protocols.
// This helps avoid import cycles by allowing the relay package
// to request protocol registration without directly importing eth/protocols/eth.
type ProtocolRegistrar interface {
	RegisterRelayProtocols(backend *Backend, networkID uint64, discCandidates enode.Iterator) []p2p.Protocol
}

// Relay is the main relay service.
type Relay struct {
	backend     *Backend
	router     *MessageRouter
	proxy      *RequestProxy
	p2pServer  *p2p.Server
	discmix    *enode.FairMix
	stack      *node.Node
	config     *Config
	networkID  uint64
	protocolRegistrar ProtocolRegistrar
	quit       chan struct{}
	wg         sync.WaitGroup
}

// NewRelay creates a new relay service.
func NewRelay(stack *node.Node, config *Config, networkID uint64, registrar ProtocolRegistrar) (*Relay, error) {
	backend := NewBackend(config, stack.Server())
	
	return &Relay{
		backend:           backend,
		p2pServer:         stack.Server(),
		stack:             stack,
		config:            config,
		networkID:         networkID,
		protocolRegistrar: registrar,
		quit:              make(chan struct{}),
	}, nil
}

// Start implements node.Lifecycle.
func (r *Relay) Start() error {
	// Setup discovery (original discovery mechanisms)
	discmix := enode.NewFairMix(0)
	if err := setupRelayDiscovery(r.p2pServer, discmix, r.backend.config); err != nil {
		return err
	}
	r.discmix = discmix

	// Start message router
	r.router = NewMessageRouter(r.backend)

	// Start request proxy with round-robin selector
	selector := NewRoundRobinSelector(r.backend)
	r.proxy = NewRequestProxy(r.backend, selector, 30*time.Second)

	// Setup ENR updater now that LocalNode() is available
	StartRelayENRUpdater(r.p2pServer.LocalNode(), r.config)

	// Start relay loop
	r.wg.Add(1)
	go r.relayLoop()

	return nil
}

func (r *Relay) relayLoop() {
	defer r.wg.Done()

	for {
		select {
		case msg := <-r.backend.GetRelayQueue():
			if msg == nil {
				continue
			}
			log.Trace("Received message for relaying",
				"from", msg.From.String()[:16]+"...",
				"code", msg.MsgCode,
				"codeName", msgCodeToString(msg.MsgCode),
				"size", len(msg.Payload),
				"isRequest", isRequest(msg.MsgCode))
			
			if isRequest(msg.MsgCode) {
				// Handle as request-response
				log.Trace("Proxying request message",
					"from", msg.From.String()[:16]+"...",
					"code", msgCodeToString(msg.MsgCode))
				go r.proxy.ProxyRequest(msg.From, msg.MsgCode, msg.RequestID, msg.Payload)
			} else {
				// Handle as broadcast/forward
				if err := r.router.ForwardMessage(msg.From, msg.MsgCode, msg.Payload); err != nil {
					log.Debug("Failed to forward message", "err", err, "code", msgCodeToString(msg.MsgCode))
				}
			}
		case <-r.quit:
			return
		}
	}
}

// isRequest checks if a message code is a request message.
func isRequest(msgCode uint64) bool {
	switch msgCode {
	case 0x03: // GetBlockHeadersMsg
		return true
	case 0x05: // GetBlockBodiesMsg
		return true
	case 0x0f: // GetReceiptsMsg
		return true
	case 0x09: // GetPooledTransactionsMsg
		return true
	default:
		return false
	}
}

// msgCodeToString converts message code to human-readable name.
func msgCodeToString(code uint64) string {
	names := map[uint64]string{
		0x00: "Status",
		0x01: "NewBlockHashes",
		0x02: "Transactions",
		0x03: "GetBlockHeaders",
		0x04: "BlockHeaders",
		0x05: "GetBlockBodies",
		0x06: "BlockBodies",
		0x07: "NewBlock",
		0x08: "NewPooledTransactionHashes",
		0x09: "GetPooledTransactions",
		0x0a: "PooledTransactions",
		0x0f: "GetReceipts",
		0x10: "Receipts",
		0x11: "BlockRangeUpdate",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return "Unknown"
}

// Stop implements node.Lifecycle.
func (r *Relay) Stop() error {
	close(r.quit)
	
	// Stop proxy
	if r.proxy != nil {
		r.proxy.Stop()
	}
	
	// Stop router
	if r.router != nil {
		r.router.Stop()
	}
	
	// Close discovery
	if r.discmix != nil {
		r.discmix.Close()
	}
	
	// Stop backend
	r.backend.Stop()
	
	// Wait for goroutines
	r.wg.Wait()
	
	return nil
}

// Backend returns the relay backend.
func (r *Relay) Backend() *Backend {
	return r.backend
}

