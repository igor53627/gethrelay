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

package p2p

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"golang.org/x/net/proxy"
)

// TorDialer wraps a standard NodeDialer and routes connections to .onion
// addresses through a SOCKS5 proxy (Tor), with configurable fallback behavior.
//
// The TorDialer supports three operational modes:
//
//  1. Default mode (preferTor=false, onlyOnion=false):
//     - Connects to .onion addresses via SOCKS5 proxy if available
//     - Falls back to clearnet on Tor connection failure
//     - Uses clearnet for peers without .onion addresses
//
//  2. Prefer Tor mode (preferTor=true, onlyOnion=false):
//     - Prefers .onion addresses when both .onion and clearnet are available
//     - Falls back to clearnet on Tor connection failure
//     - Uses clearnet for peers without .onion addresses
//
//  3. Tor Only mode (onlyOnion=true):
//     - Only connects to peers with .onion addresses
//     - Rejects peers without .onion addresses
//     - No clearnet fallback
type TorDialer struct {
	socksAddr string     // SOCKS5 proxy address (e.g., "127.0.0.1:9050")
	clearnet  NodeDialer // Fallback dialer for clearnet connections
	preferTor bool       // Prefer Tor when both .onion and clearnet available
	onlyOnion bool       // Reject clearnet connections (Tor-only mode)
}

// NewTorDialer creates a TorDialer with the specified configuration.
//
// Parameters:
//   - socksAddr: Address of the SOCKS5 proxy (e.g., "127.0.0.1:9050" for Tor)
//   - clearnet: Fallback NodeDialer for clearnet connections
//   - preferTor: If true, prefer .onion addresses when both .onion and clearnet are available
//   - onlyOnion: If true, reject peers without .onion addresses (Tor-only mode)
//
// Example:
//
//	// Default mode: Tor with clearnet fallback
//	dialer := NewTorDialer("127.0.0.1:9050", tcpDialer, false, false)
//
//	// Prefer Tor mode
//	dialer := NewTorDialer("127.0.0.1:9050", tcpDialer, true, false)
//
//	// Tor only mode (no clearnet)
//	dialer := NewTorDialer("127.0.0.1:9050", tcpDialer, false, true)
func NewTorDialer(socksAddr string, clearnet NodeDialer, preferTor, onlyOnion bool) *TorDialer {
	return &TorDialer{
		socksAddr: socksAddr,
		clearnet:  clearnet,
		preferTor: preferTor,
		onlyOnion: onlyOnion,
	}
}

// Dial implements the NodeDialer interface.
//
// It determines the appropriate transport (Tor vs clearnet) based on:
//  1. Whether the peer has a .onion address (enr.Onion3 entry or .onion hostname)
//  2. Whether the peer has a clearnet address (TCP endpoint)
//  3. The configured operational mode (preferTor, onlyOnion)
//
// Connection priority:
//   - In only-onion mode: Reject peers without .onion addresses
//   - If peer has .onion and (preferTor OR no clearnet): Use Tor
//   - If Tor fails and not only-onion mode: Fallback to clearnet
//   - If peer has only clearnet: Use clearnet (unless only-onion mode)
func (t *TorDialer) Dial(ctx context.Context, dest *enode.Node) (net.Conn, error) {
	if dest == nil {
		return nil, fmt.Errorf("cannot dial nil peer")
	}

	// Extract .onion address from ENR or hostname
	var onion enr.Onion3
	var onionAddr string
	hasOnion := false

	// Check ENR first
	if dest.Load(&onion) == nil && onion != "" {
		onionAddr = string(onion)
		hasOnion = true
	} else if hostname := dest.Hostname(); hostname != "" && strings.HasSuffix(strings.ToLower(hostname), ".onion") {
		// Check hostname for .onion address (common for static nodes)
		onionAddr = hostname
		hasOnion = true
	}

	// Check clearnet availability
	_, hasClearnet := dest.TCPEndpoint()

	// Only-onion mode: reject clearnet-only peers
	if t.onlyOnion && !hasOnion {
		return nil, fmt.Errorf("only-onion mode: peer %s has no .onion address", dest.ID())
	}

	// Determine whether to use Tor
	useTor := hasOnion && (t.preferTor || !hasClearnet)

	if useTor {
		// Attempt connection via Tor
		conn, err := t.dialViaTor(ctx, dest, onionAddr)
		if err != nil {
			// In only-onion mode, don't fallback to clearnet
			if t.onlyOnion {
				return nil, fmt.Errorf("failed to connect via Tor in only-onion mode: %w", err)
			}
			// Fallback to clearnet if available
			if hasClearnet {
				return t.clearnet.Dial(ctx, dest)
			}
			return nil, fmt.Errorf("Tor connection failed and no clearnet fallback available: %w", err)
		}
		return conn, nil
	}

	// Use clearnet
	if hasClearnet {
		return t.clearnet.Dial(ctx, dest)
	}

	// No usable addresses
	return nil, fmt.Errorf("peer %s has no usable addresses", dest.ID())
}

// dialViaTor attempts to connect to a .onion address through the SOCKS5 proxy.
//
// It extracts the TCP port from the peer's ENR (defaulting to 30303 if not specified),
// creates a SOCKS5 dialer, and establishes a connection to the .onion address.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - dest: The destination peer node
//   - onionAddr: The .onion address (e.g., "abc...xyz.onion")
//
// Returns:
//   - net.Conn: Established connection to the .onion address
//   - error: Any error encountered during SOCKS5 setup or connection
func (t *TorDialer) dialViaTor(ctx context.Context, dest *enode.Node, onionAddr string) (net.Conn, error) {
	// Extract TCP port from ENR
	var tcpPort enr.TCP
	if err := dest.Load(&tcpPort); err != nil || tcpPort == 0 {
		tcpPort = 30303 // Default geth P2P port
	}

	// Create SOCKS5 dialer with timeout from context
	var baseDialer net.Dialer
	if deadline, ok := ctx.Deadline(); ok {
		baseDialer.Deadline = deadline
	} else {
		baseDialer.Timeout = 30 * time.Second
	}

	// Create SOCKS5 proxy dialer
	socksDialer, err := proxy.SOCKS5("tcp", t.socksAddr, nil, &baseDialer)
	if err != nil {
		return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}

	// Dial through SOCKS5 proxy
	target := fmt.Sprintf("%s:%d", onionAddr, tcpPort)
	conn, err := socksDialer.Dial("tcp", target)
	if err != nil {
		return nil, fmt.Errorf("SOCKS5 dial to %s failed: %w", target, err)
	}

	return conn, nil
}
