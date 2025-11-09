// Copyright 2025 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

// TestTorDialer_EdgeCases tests additional edge cases to increase coverage.
func TestTorDialer_EdgeCases(t *testing.T) {
	t.Run("EmptyOnionString", func(t *testing.T) {
		// Test that empty .onion string is treated as no .onion
		node := createTestNode(t, "", net.IPv4(192, 168, 1, 1), 30303)

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					io.Copy(io.Discard, server)
					server.Close()
				}()
				return client, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9050", mockClearnet, false, false)
		conn, err := torDialer.Dial(context.Background(), node)
		if err != nil {
			t.Fatalf("expected success with clearnet fallback, got error: %v", err)
		}
		if conn == nil {
			t.Fatal("expected connection, got nil")
		}
		conn.Close()
	})

	t.Run("TorFailureWithNoClearnet", func(t *testing.T) {
		// Test Tor failure when peer has no clearnet address
		onionAddr := generateValidOnion3()
		node := createTestNode(t, onionAddr, nil, 30303)

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				t.Fatal("clearnet dialer should not be called")
				return nil, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9999", mockClearnet, false, false) // Unreachable SOCKS5

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		conn, err := torDialer.Dial(ctx, node)
		if err == nil {
			t.Fatal("expected error when Tor fails and no clearnet available")
		}
		if conn != nil {
			t.Fatal("expected nil connection")
		}
	})

	t.Run("PreferTorWithoutOnion", func(t *testing.T) {
		// Test prefer-tor mode with peer that has no .onion (should use clearnet)
		node := createTestNode(t, "", net.IPv4(192, 168, 1, 1), 30303)

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				client, server := net.Pipe()
				go func() {
					io.Copy(io.Discard, server)
					server.Close()
				}()
				return client, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9050", mockClearnet, true, false) // prefer-tor mode

		conn, err := torDialer.Dial(context.Background(), node)
		if err != nil {
			t.Fatalf("expected clearnet fallback in prefer-tor mode, got error: %v", err)
		}
		if conn == nil {
			t.Fatal("expected connection via clearnet")
		}
		conn.Close()
	})

	t.Run("OnlyOnionModeWithClearnetPeer", func(t *testing.T) {
		// Test only-onion mode rejects clearnet-only peer
		node := createTestNode(t, "", net.IPv4(192, 168, 1, 1), 30303)

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				t.Fatal("clearnet dialer should not be called in only-onion mode")
				return nil, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9050", mockClearnet, false, true) // only-onion mode

		conn, err := torDialer.Dial(context.Background(), node)
		if err == nil {
			t.Fatal("expected error in only-onion mode for clearnet-only peer")
		}
		if conn != nil {
			t.Fatal("expected nil connection")
		}
	})

	t.Run("OnlyOnionModeTorFailure", func(t *testing.T) {
		// Test only-onion mode with Tor-only peer (no clearnet) where Tor fails
		onionAddr := generateValidOnion3()
		node := createTestNode(t, onionAddr, nil, 30303) // No clearnet address

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				t.Fatal("clearnet dialer should not be called for Tor-only peer")
				return nil, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9999", mockClearnet, false, true) // only-onion, unreachable proxy

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		conn, err := torDialer.Dial(ctx, node)
		if err == nil {
			t.Fatal("expected error when Tor fails in only-onion mode")
		}
		if conn != nil {
			t.Fatal("expected nil connection")
		}
	})

	t.Run("NilPeer", func(t *testing.T) {
		// Test nil peer handling
		mockClearnet := &mockDialer{}
		torDialer := NewTorDialer("127.0.0.1:9050", mockClearnet, false, false)

		conn, err := torDialer.Dial(context.Background(), nil)
		if err == nil {
			t.Fatal("expected error for nil peer")
		}
		if conn != nil {
			t.Fatal("expected nil connection for nil peer")
		}
	})

	t.Run("PeerWithNoAddresses", func(t *testing.T) {
		// Test peer with neither .onion nor clearnet addresses
		node := createTestNode(t, "", nil, 0)

		mockClearnet := &mockDialer{
			dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
				// This should be called but will fail because node has no address
				return nil, nil
			},
		}

		torDialer := NewTorDialer("127.0.0.1:9050", mockClearnet, false, false)

		_, err := torDialer.Dial(context.Background(), node)
		if err == nil {
			t.Fatal("expected error for peer with no usable addresses")
		}
	})

	t.Run("TCPPortDefaulting", func(t *testing.T) {
		// Test that default port 30303 is used when peer has no TCP port in ENR
		listener := startMockSOCKS5Server(t, func(target string) bool {
			// Verify default port is used
			expectedTarget := "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion:30303"
			if target != expectedTarget {
				t.Errorf("expected target %s, got %s", expectedTarget, target)
			}
			return true
		})
		defer listener.Close()
		socksAddr := listener.Addr().String()

		onionAddr := "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion"
		node := createTestNode(t, onionAddr, nil, 0) // No TCP port

		mockClearnet := &mockDialer{}
		torDialer := NewTorDialer(socksAddr, mockClearnet, false, false)

		conn, err := torDialer.Dial(context.Background(), node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn == nil {
			t.Fatal("expected connection")
		}
		conn.Close()
	})

	t.Run("CustomTCPPort", func(t *testing.T) {
		// Test that custom TCP port from ENR is used
		// Note: We need to provide an IP address to set the TCP port in ENR
		listener := startMockSOCKS5Server(t, func(target string) bool {
			// Verify custom port is used
			expectedTarget := "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion:40404"
			if target != expectedTarget {
				t.Errorf("expected target %s, got %s", expectedTarget, target)
			}
			return true
		})
		defer listener.Close()
		socksAddr := listener.Addr().String()

		onionAddr := "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion"
		// Provide IP so createTestNode will set the TCP port in ENR
		node := createTestNode(t, onionAddr, net.IPv4(192, 168, 1, 1), 40404)

		mockClearnet := &mockDialer{}
		torDialer := NewTorDialer(socksAddr, mockClearnet, true, false) // prefer-tor to use Tor instead of clearnet

		conn, err := torDialer.Dial(context.Background(), node)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn == nil {
			t.Fatal("expected connection")
		}
		conn.Close()
	})
}

// startMockSOCKS5Server starts a mock SOCKS5 server that calls targetValidator on each connect request.
func startMockSOCKS5Server(t *testing.T, targetValidator func(target string) bool) net.Listener {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create mock SOCKS5 server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleMockSOCKS5(t, conn, targetValidator)
		}
	}()

	return listener
}

// handleMockSOCKS5 handles SOCKS5 connection and validates target address.
func handleMockSOCKS5(t *testing.T, conn net.Conn, targetValidator func(target string) bool) {
	defer conn.Close()

	buf := make([]byte, 512)

	// Read version + auth methods
	n, err := conn.Read(buf)
	if err != nil || n < 2 || buf[0] != 0x05 {
		return
	}

	// Send "no auth" response
	conn.Write([]byte{0x05, 0x00})

	// Read connect request
	n, err = conn.Read(buf)
	if err != nil || n < 7 || buf[1] != 0x01 {
		return
	}

	// Parse target address
	addrType := buf[3]
	var target string

	if addrType == 0x03 { // Domain name
		domainLen := int(buf[4])
		if n < 5+domainLen+2 {
			return
		}
		domain := string(buf[5 : 5+domainLen])
		portOffset := 5 + domainLen
		portNum := uint16(buf[portOffset])<<8 | uint16(buf[portOffset+1])
		target = fmt.Sprintf("%s:%d", domain, portNum)
	}

	// Validate target if validator provided
	if targetValidator != nil && !targetValidator(target) {
		// Send failure
		conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Send success
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// Keep connection open briefly
	time.Sleep(10 * time.Millisecond)
}
