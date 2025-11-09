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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// mockSOCKS5Server creates a simple SOCKS5 proxy server for testing.
// It handles basic SOCKS5 handshake and connection requests.
func mockSOCKS5Server(t *testing.T) (addr string, close func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create mock SOCKS5 server: %v", err)
	}

	// Start accepting connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Server closed
			}
			go handleSOCKS5Connection(t, conn)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

// handleSOCKS5Connection handles a single SOCKS5 connection.
// Implements minimal SOCKS5 protocol (RFC 1928).
func handleSOCKS5Connection(t *testing.T, conn net.Conn) {
	defer conn.Close()

	// Read version and authentication methods
	buf := make([]byte, 257)
	n, err := conn.Read(buf)
	if err != nil || n < 2 {
		return
	}

	// Expect SOCKS5 version (0x05)
	if buf[0] != 0x05 {
		return
	}

	// Send "no authentication required" response
	conn.Write([]byte{0x05, 0x00})

	// Read connection request
	n, err = conn.Read(buf)
	if err != nil || n < 7 {
		return
	}

	// Expect CONNECT command (0x01)
	if buf[1] != 0x01 {
		return
	}

	// Extract address type and target
	addrType := buf[3]
	var targetAddr string
	var portOffset int

	switch addrType {
	case 0x01: // IPv4
		if n < 10 {
			return
		}
		targetAddr = fmt.Sprintf("%d.%d.%d.%d", buf[4], buf[5], buf[6], buf[7])
		portOffset = 8
	case 0x03: // Domain name
		domainLen := int(buf[4])
		if n < 7+domainLen {
			return
		}
		targetAddr = string(buf[5 : 5+domainLen])
		portOffset = 5 + domainLen
	default:
		// Send unsupported address type error
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	port := (uint16(buf[portOffset]) << 8) | uint16(buf[portOffset+1])

	// For testing, simulate successful connection to .onion addresses
	if strings.HasSuffix(targetAddr, ".onion") {
		// Send success response
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Keep connection open and echo data (for testing purposes)
		io.Copy(conn, conn)
	} else {
		// Try to actually connect for clearnet addresses (testing fallback)
		target, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetAddr, port), 5*time.Second)
		if err != nil {
			// Send connection refused error
			conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer target.Close()

		// Send success response
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Relay data
		go io.Copy(target, conn)
		io.Copy(conn, target)
	}
}

// mockDialer implements NodeDialer for testing clearnet fallback.
type mockDialer struct {
	dialFunc func(context.Context, *enode.Node) (net.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, dest *enode.Node) (net.Conn, error) {
	if m.dialFunc != nil {
		return m.dialFunc(ctx, dest)
	}
	// Default: create a pipe connection for testing
	client, server := net.Pipe()
	go func() {
		io.Copy(io.Discard, server)
		server.Close()
	}()
	return client, nil
}

// createTestNode creates an enode.Node for testing with optional .onion address.
func createTestNode(t *testing.T, onionAddr string, ip net.IP, port uint16) *enode.Node {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	var r enr.Record
	r.Set(enr.ID("v4"))

	if onionAddr != "" {
		r.Set(enr.Onion3(onionAddr))
	}

	if ip != nil {
		r.Set(enr.IP(ip))
		r.Set(enr.TCP(port))
	}

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign record: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	return node
}

// generateValidOnion3 generates a syntactically valid Tor v3 .onion address for testing.
func generateValidOnion3() string {
	// Generate 35 random bytes (will be base32 encoded to 56 chars)
	bytes := make([]byte, 35)
	rand.Read(bytes)

	// Convert to base32 (using lowercase a-z and 2-7)
	base32Chars := "abcdefghijklmnopqrstuvwxyz234567"
	result := make([]byte, 56)

	hexStr := hex.EncodeToString(bytes)
	for i := 0; i < 56; i++ {
		// Use hex chars to index into base32 chars
		if i < len(hexStr) {
			idx := int(hexStr[i] % 32)
			result[i] = base32Chars[idx]
		} else {
			result[i] = 'a'
		}
	}

	return string(result) + ".onion"
}

// TestTorDialer_SuccessfulSOCKS5Connection tests successful connection to .onion address.
func TestTorDialer_SuccessfulSOCKS5Connection(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnet := &mockDialer{}
	dialer := NewTorDialer(socksAddr, clearnet, false, false)

	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, nil, 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful dial to .onion address, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	conn.Close()
}

// TestTorDialer_ClearnetFallback tests fallback to clearnet when Tor fails.
func TestTorDialer_ClearnetFallback(t *testing.T) {
	// Use invalid SOCKS5 address to force Tor failure
	invalidSocksAddr := "127.0.0.1:1" // Port 1 should be unavailable

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			client, server := net.Pipe()
			go func() {
				io.Copy(io.Discard, server)
				server.Close()
			}()
			return client, nil
		},
	}

	dialer := NewTorDialer(invalidSocksAddr, clearnet, false, false)

	// Node with both .onion and clearnet address
	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, net.IPv4(192, 168, 1, 1), 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful fallback to clearnet, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if !clearnetCalled {
		t.Fatal("expected clearnet dialer to be called as fallback")
	}
	conn.Close()
}

// TestTorDialer_PreferTorMode tests prefer-tor mode.
func TestTorDialer_PreferTorMode(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			return nil, errors.New("should not be called")
		},
	}

	dialer := NewTorDialer(socksAddr, clearnet, true, false) // preferTor=true

	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, net.IPv4(192, 168, 1, 1), 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful dial via Tor, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if clearnetCalled {
		t.Fatal("clearnet should not be called in prefer-tor mode when .onion is available")
	}
	conn.Close()
}

// TestTorDialer_OnlyOnionMode_RejectsClearnet tests only-onion mode rejects clearnet-only peers.
func TestTorDialer_OnlyOnionMode_RejectsClearnet(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnet := &mockDialer{}
	dialer := NewTorDialer(socksAddr, clearnet, false, true) // onlyOnion=true

	// Node with only clearnet address (no .onion)
	node := createTestNode(t, "", net.IPv4(192, 168, 1, 1), 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error when dialing clearnet-only peer in only-onion mode")
	}
	if !strings.Contains(err.Error(), "only-onion mode") && !strings.Contains(err.Error(), "no .onion") {
		t.Fatalf("expected 'only-onion mode' error, got: %v", err)
	}
}

// TestTorDialer_OnlyOnionMode_AcceptsOnion tests only-onion mode accepts .onion peers.
func TestTorDialer_OnlyOnionMode_AcceptsOnion(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnet := &mockDialer{}
	dialer := NewTorDialer(socksAddr, clearnet, false, true) // onlyOnion=true

	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, nil, 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful dial to .onion peer in only-onion mode, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	conn.Close()
}

// TestTorDialer_ClearnetOnly tests peer without .onion uses clearnet.
func TestTorDialer_ClearnetOnly(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			client, server := net.Pipe()
			go func() {
				io.Copy(io.Discard, server)
				server.Close()
			}()
			return client, nil
		},
	}

	dialer := NewTorDialer(socksAddr, clearnet, false, false)

	// Node with only clearnet address
	node := createTestNode(t, "", net.IPv4(192, 168, 1, 1), 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful dial via clearnet, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if !clearnetCalled {
		t.Fatal("expected clearnet dialer to be called for clearnet-only peer")
	}
	conn.Close()
}

// TestTorDialer_OnionWithSOCKS5 tests peer with .onion uses SOCKS5.
func TestTorDialer_OnionWithSOCKS5(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			return nil, errors.New("should not be called")
		},
	}

	dialer := NewTorDialer(socksAddr, clearnet, false, false)

	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, nil, 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("expected successful dial via SOCKS5, got error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if clearnetCalled {
		t.Fatal("clearnet should not be called when .onion is available")
	}
	conn.Close()
}

// TestTorDialer_InvalidSOCKS5Address tests handling of invalid SOCKS5 proxy address.
func TestTorDialer_InvalidSOCKS5Address(t *testing.T) {
	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			client, server := net.Pipe()
			go func() {
				io.Copy(io.Discard, server)
				server.Close()
			}()
			return client, nil
		},
	}

	dialer := NewTorDialer("invalid-address:99999", clearnet, false, false)

	onionAddr := generateValidOnion3()
	// Node with both .onion and clearnet for fallback testing
	node := createTestNode(t, onionAddr, net.IPv4(192, 168, 1, 1), 30303)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	// Should fallback to clearnet due to invalid SOCKS5 address
	if err != nil {
		t.Fatalf("expected successful fallback to clearnet, got error: %v", err)
	}
	if !clearnetCalled {
		t.Fatal("expected clearnet fallback when SOCKS5 address is invalid")
	}
	if conn != nil {
		conn.Close()
	}
}

// TestTorDialer_ContextTimeout tests context cancellation handling.
func TestTorDialer_ContextTimeout(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnet := &mockDialer{}
	dialer := NewTorDialer(socksAddr, clearnet, false, false)

	onionAddr := generateValidOnion3()
	node := createTestNode(t, onionAddr, nil, 30303)

	// Very short timeout to force cancellation
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure context is expired

	conn, err := dialer.Dial(ctx, node)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error due to context timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline") && !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout/deadline error, got: %v", err)
	}
}

// TestTorDialer_NoPeerAddresses tests handling of peer with no usable addresses.
func TestTorDialer_NoPeerAddresses(t *testing.T) {
	socksAddr, close := mockSOCKS5Server(t)
	defer close()

	clearnet := &mockDialer{}
	dialer := NewTorDialer(socksAddr, clearnet, false, false)

	// Node with neither .onion nor clearnet address
	node := createTestNode(t, "", nil, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected error when peer has no usable addresses")
	}
	if !strings.Contains(err.Error(), "no usable") && !strings.Contains(err.Error(), "no port") {
		t.Fatalf("expected 'no usable addresses' error, got: %v", err)
	}
}
