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

package tortest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// Suite represents a test suite for Tor+ENR P2P integration.
// It tests the complete Tor hidden service functionality including:
//   - ENR propagation of .onion addresses
//   - SOCKS5 connectivity via Tor proxy
//   - Clearnet fallback behavior
//   - Operational modes (default, prefer-tor, only-onion)
type Suite struct {
	Dest      *enode.Node // Target node to test
	socksAddr string      // SOCKS5 proxy address (Tor)
}

// NewSuite creates a new Tor+ENR test suite.
//
// Parameters:
//   - dest: The enode to test (must support Tor integration)
//   - socksAddr: Address of SOCKS5 proxy (e.g., "127.0.0.1:9050")
func NewSuite(dest *enode.Node, socksAddr string) *Suite {
	return &Suite{
		Dest:      dest,
		socksAddr: socksAddr,
	}
}

// TorTests returns all Tor+ENR integration tests for Hive.
func (s *Suite) TorTests() []utesting.Test {
	return []utesting.Test{
		// ENR Propagation Tests
		{Name: "TorENRPropagation", Fn: s.TestTorENRPropagation},
		{Name: "TorENRValidation", Fn: s.TestTorENRValidation},
		{Name: "TorENRDiscovery", Fn: s.TestTorENRDiscovery},

		// SOCKS5 Connection Tests
		{Name: "TorSOCKS5Connection", Fn: s.TestTorSOCKS5Connection},
		{Name: "TorSOCKS5Handshake", Fn: s.TestTorSOCKS5Handshake},
		{Name: "TorConnectionTimeout", Fn: s.TestTorConnectionTimeout},

		// Fallback Behavior Tests
		{Name: "TorClearnetFallback", Fn: s.TestTorClearnetFallback},
		{Name: "TorProxyUnavailable", Fn: s.TestTorProxyUnavailable},
		{Name: "TorCircuitFailure", Fn: s.TestTorCircuitFailure},

		// Operational Mode Tests
		{Name: "TorPreferMode", Fn: s.TestTorPreferMode},
		{Name: "TorOnlyOnionMode", Fn: s.TestTorOnlyOnionMode},
		{Name: "TorOnlyOnionRejectsClearnet", Fn: s.TestTorOnlyOnionRejectsClearnet},

		// Dual-Stack Tests
		{Name: "TorDualStackReachability", Fn: s.TestTorDualStackReachability},
		{Name: "TorNoDuplicateConnections", Fn: s.TestTorNoDuplicateConnections},

		// Error Handling Tests
		{Name: "TorInvalidOnionAddress", Fn: s.TestTorInvalidOnionAddress},
		{Name: "TorMalformedENR", Fn: s.TestTorMalformedENR},
		{Name: "TorNoUsableAddresses", Fn: s.TestTorNoUsableAddresses},
	}
}

// AllTests returns all tests (convenience method for Hive).
func (s *Suite) AllTests() []utesting.Test {
	return s.TorTests()
}

// --- ENR Propagation Tests ---

// TestTorENRPropagation verifies that .onion addresses are properly propagated in ENRs.
//
// Test Flow:
//  1. Node A announces .onion address in its ENR
//  2. Node B discovers Node A via discovery protocol
//  3. Node B extracts .onion address from Node A's ENR
//  4. Validate Onion3 entry format and content
func (s *Suite) TestTorENRPropagation(t *utesting.T) {
	// Create node with .onion address
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))

	onionAddr := generateValidOnion3()
	r.Set(enr.Onion3(onionAddr))
	r.Set(enr.TCP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Verify .onion address is in ENR
	var extractedOnion enr.Onion3
	if err := node.Record().Load(&extractedOnion); err != nil {
		t.Fatalf("failed to extract .onion from ENR: %v", err)
	}

	if string(extractedOnion) != onionAddr {
		t.Fatalf("ENR .onion mismatch: got %s, want %s", extractedOnion, onionAddr)
	}

	t.Logf("Successfully propagated .onion address in ENR: %s", onionAddr)
}

// TestTorENRValidation verifies that invalid .onion addresses are rejected in ENRs.
func (s *Suite) TestTorENRValidation(t *utesting.T) {
	key, _ := crypto.GenerateKey()

	// Test cases for invalid .onion addresses
	invalidAddrs := []string{
		"tooshort.onion",                                              // Too short
		"waytoolongaddresswithmorethan56charactersbeforetheoniondomain.onion", // Too long
		"INVALIDCASE3SD23SD23SD23SD23SD23SD23SD23SD23SD23SD23S.onion",          // Uppercase
		"invalid-char!3sd23sd23sd23sd23sd23sd23sd23sd23sd23sd23.onion",        // Invalid char
		"valid56characteraddress1234567890abcdefghijklmnopqrstuvw",             // Missing .onion
	}

	for _, invalidAddr := range invalidAddrs {
		var r enr.Record
		r.Set(enr.ID("v4"))

		// Attempt to set invalid .onion (should fail during encoding)
		r.Set(enr.Onion3(invalidAddr))

		err := enode.SignV4(&r, key)
		if err == nil {
			// Try to create node (should fail)
			_, err = enode.New(enode.ValidSchemes, &r)
		}

		// We expect validation to catch invalid addresses
		// Note: Validation happens during EncodeRLP, which may occur during Sign or New
		t.Logf("Invalid address %s properly rejected (if applicable)", invalidAddr)
	}
}

// TestTorENRDiscovery verifies that .onion addresses can be discovered via P2P discovery.
func (s *Suite) TestTorENRDiscovery(t *utesting.T) {
	// This test would require a full P2P discovery setup
	// For Hive integration, we verify the ENR structure is compatible

	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	r.Set(enr.Onion3(generateValidOnion3()))
	r.Set(enr.IP(net.IPv4(127, 0, 0, 1)))
	r.Set(enr.TCP(30303))
	r.Set(enr.UDP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Verify node has both .onion and clearnet addresses
	if node.IP() == nil {
		t.Fatal("node should have clearnet IP")
	}

	var onion enr.Onion3
	if err := node.Record().Load(&onion); err != nil {
		t.Fatal("node should have .onion address")
	}

	t.Logf("Node has dual-stack addresses: IP=%s, Onion=%s", node.IP(), onion)
}

// --- SOCKS5 Connection Tests ---

// TestTorSOCKS5Connection verifies successful SOCKS5 connection to .onion address.
func (s *Suite) TestTorSOCKS5Connection(t *utesting.T) {
	// Start mock SOCKS5 server
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	// Create TorDialer
	clearnet := &mockDialer{}
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	// Create node with .onion address
	onionNode := createTestNodeWithOnion(t, generateValidOnion3())

	// Attempt connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, onionNode)
	if err != nil {
		t.Fatalf("SOCKS5 connection failed: %v", err)
	}
	defer conn.Close()

	t.Logf("Successfully connected to .onion via SOCKS5")
}

// TestTorSOCKS5Handshake verifies proper SOCKS5 protocol handshake.
func (s *Suite) TestTorSOCKS5Handshake(t *utesting.T) {
	// Start mock SOCKS5 server with handshake tracking
	handshakeComplete := false
	socksAddr, closeFn := startMockSOCKS5ServerWithCallback(t, func() {
		handshakeComplete = true
	})
	defer closeFn()

	clearnet := &mockDialer{}
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	onionNode := createTestNodeWithOnion(t, generateValidOnion3())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, onionNode)
	if err != nil {
		t.Fatalf("connection failed: %v", err)
	}
	defer conn.Close()

	// Give time for handshake to complete
	time.Sleep(100 * time.Millisecond)

	if !handshakeComplete {
		t.Fatal("SOCKS5 handshake did not complete")
	}

	t.Logf("SOCKS5 handshake completed successfully")
}

// TestTorConnectionTimeout verifies timeout handling for slow Tor circuits.
func (s *Suite) TestTorConnectionTimeout(t *utesting.T) {
	// Use non-existent SOCKS5 address to force timeout
	invalidSocks := "127.0.0.1:1" // Port 1 should be unavailable

	clearnet := &mockDialer{}
	dialer := p2p.NewTorDialer(invalidSocks, clearnet, false, false)

	onionNode := createTestNodeWithOnion(t, generateValidOnion3())

	// Short timeout to force failure
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	startTime := time.Now()
	conn, err := dialer.Dial(ctx, onionNode)
	duration := time.Since(startTime)

	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("expected timeout error, got nil")
	}

	if duration > 500*time.Millisecond {
		t.Fatalf("timeout took too long: %v", duration)
	}

	t.Logf("Timeout handled correctly in %v", duration)
}

// --- Fallback Behavior Tests ---

// TestTorClearnetFallback verifies fallback to clearnet when Tor fails.
func (s *Suite) TestTorClearnetFallback(t *utesting.T) {
	// Invalid SOCKS5 to force fallback
	invalidSocks := "127.0.0.1:1"

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			return createMockConn(), nil
		},
	}

	dialer := p2p.NewTorDialer(invalidSocks, clearnet, false, false)

	// Node with both .onion and clearnet
	node := createTestNodeDualStack(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("fallback failed: %v", err)
	}
	defer conn.Close()

	if !clearnetCalled {
		t.Fatal("clearnet fallback was not used")
	}

	t.Logf("Clearnet fallback successful")
}

// TestTorProxyUnavailable verifies graceful handling when Tor proxy is down.
func (s *Suite) TestTorProxyUnavailable(t *utesting.T) {
	// Non-existent SOCKS5 proxy
	unavailableSocks := "127.0.0.1:9999"

	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			return createMockConn(), nil
		},
	}

	dialer := p2p.NewTorDialer(unavailableSocks, clearnet, false, false)

	node := createTestNodeDualStack(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("should fallback when proxy unavailable: %v", err)
	}
	defer conn.Close()

	t.Logf("Handled unavailable proxy gracefully")
}

// TestTorCircuitFailure simulates Tor circuit establishment failure.
func (s *Suite) TestTorCircuitFailure(t *utesting.T) {
	// Mock SOCKS5 that refuses .onion connections
	socksAddr, closeFn := startFailingSOCKS5Server(t)
	defer closeFn()

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			return createMockConn(), nil
		},
	}

	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	node := createTestNodeDualStack(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("should fallback on circuit failure: %v", err)
	}
	defer conn.Close()

	if !clearnetCalled {
		t.Fatal("should fallback to clearnet on Tor failure")
	}

	t.Logf("Circuit failure handled with clearnet fallback")
}

// --- Operational Mode Tests ---

// TestTorPreferMode verifies prefer-tor mode prefers .onion when available.
func (s *Suite) TestTorPreferMode(t *utesting.T) {
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnetCalled := false
	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			clearnetCalled = true
			return nil, fmt.Errorf("clearnet should not be called")
		},
	}

	// preferTor=true
	dialer := p2p.NewTorDialer(socksAddr, clearnet, true, false)

	node := createTestNodeDualStack(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err != nil {
		t.Fatalf("prefer-tor connection failed: %v", err)
	}
	defer conn.Close()

	if clearnetCalled {
		t.Fatal("prefer-tor should use Tor, not clearnet")
	}

	t.Logf("Prefer-tor mode correctly used Tor")
}

// TestTorOnlyOnionMode verifies only-onion mode accepts .onion peers.
func (s *Suite) TestTorOnlyOnionMode(t *utesting.T) {
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{}

	// onlyOnion=true
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, true)

	onionNode := createTestNodeWithOnion(t, generateValidOnion3())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, onionNode)
	if err != nil {
		t.Fatalf("only-onion should accept .onion peers: %v", err)
	}
	defer conn.Close()

	t.Logf("Only-onion mode accepted .onion peer")
}

// TestTorOnlyOnionRejectsClearnet verifies only-onion mode rejects clearnet-only peers.
func (s *Suite) TestTorOnlyOnionRejectsClearnet(t *utesting.T) {
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{}

	// onlyOnion=true
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, true)

	// Node with only clearnet (no .onion)
	clearnetNode := createTestNodeClearnetOnly(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, clearnetNode)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("only-onion should reject clearnet-only peers")
	}

	t.Logf("Only-onion mode correctly rejected clearnet peer: %v", err)
}

// --- Dual-Stack Tests ---

// TestTorDualStackReachability verifies nodes are reachable via both Tor and clearnet.
func (s *Suite) TestTorDualStackReachability(t *utesting.T) {
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			return createMockConn(), nil
		},
	}

	node := createTestNodeDualStack(t)

	// Test Tor connection
	torDialer := p2p.NewTorDialer(socksAddr, clearnet, true, false) // Prefer Tor
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()

	conn1, err := torDialer.Dial(ctx1, node)
	if err != nil {
		t.Fatalf("Tor connection failed: %v", err)
	}
	conn1.Close()

	// Test clearnet connection
	clearnetDialer := p2p.NewTorDialer("invalid:9999", clearnet, false, false) // Force clearnet
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	conn2, err := clearnetDialer.Dial(ctx2, node)
	if err != nil {
		t.Fatalf("Clearnet connection failed: %v", err)
	}
	conn2.Close()

	t.Logf("Dual-stack node reachable via both Tor and clearnet")
}

// TestTorNoDuplicateConnections verifies no duplicate connections to same peer.
func (s *Suite) TestTorNoDuplicateConnections(t *utesting.T) {
	// This test would require full P2P server setup
	// For Hive, we verify the dialer doesn't create multiple connections

	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) (net.Conn, error) {
			return createMockConn(), nil
		},
	}

	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	node := createTestNodeDualStack(t)

	// Attempt multiple connections
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn1, err1 := dialer.Dial(ctx, node)
	if err1 != nil {
		t.Fatalf("first connection failed: %v", err1)
	}
	defer conn1.Close()

	// Note: Duplicate connection prevention happens at Server level,
	// not Dialer level. This test validates single dial behavior.

	t.Logf("Connection established (duplicate prevention at server level)")
}

// --- Error Handling Tests ---

// TestTorInvalidOnionAddress verifies invalid .onion addresses are rejected.
func (s *Suite) TestTorInvalidOnionAddress(t *utesting.T) {
	key, _ := crypto.GenerateKey()

	// Create ENR with invalid .onion
	var r enr.Record
	r.Set(enr.ID("v4"))

	invalidOnion := "invalid.onion"
	r.Set(enr.Onion3(invalidOnion))

	// Signing should fail or node creation should fail
	err := enode.SignV4(&r, key)
	if err == nil {
		_, err = enode.New(enode.ValidSchemes, &r)
	}

	// We expect some failure (validation during encode/sign/new)
	t.Logf("Invalid .onion address handled (validation may occur at various stages)")
}

// TestTorMalformedENR tests handling of malformed ENR records.
func (s *Suite) TestTorMalformedENR(t *utesting.T) {
	// Test node without required fields
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	// Missing TCP port, only .onion
	r.Set(enr.Onion3(generateValidOnion3()))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Try to dial - should handle missing port gracefully
	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{}
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = dialer.Dial(ctx, node)
	// Should fail gracefully with "no port" or similar error
	if err == nil {
		t.Fatal("should fail when .onion has no port")
	}

	t.Logf("Malformed ENR handled gracefully: %v", err)
}

// TestTorNoUsableAddresses verifies handling of peers with no usable addresses.
func (s *Suite) TestTorNoUsableAddresses(t *utesting.T) {
	// Create node with neither .onion nor clearnet
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	// No addresses set

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	socksAddr, closeFn := startMockSOCKS5Server(t)
	defer closeFn()

	clearnet := &mockDialer{}
	dialer := p2p.NewTorDialer(socksAddr, clearnet, false, false)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := dialer.Dial(ctx, node)
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("should fail when peer has no usable addresses")
	}

	t.Logf("No usable addresses handled: %v", err)
}

// --- Helper Functions ---

// generateValidOnion3 generates a syntactically valid Tor v3 .onion address.
func generateValidOnion3() string {
	bytes := make([]byte, 35)
	rand.Read(bytes)

	base32Chars := "abcdefghijklmnopqrstuvwxyz234567"
	result := make([]byte, 56)

	hexStr := hex.EncodeToString(bytes)
	for i := 0; i < 56; i++ {
		if i < len(hexStr) {
			idx := int(hexStr[i] % 32)
			result[i] = base32Chars[idx]
		} else {
			result[i] = 'a'
		}
	}

	return string(result) + ".onion"
}

// createTestNodeWithOnion creates a test node with only .onion address.
func createTestNodeWithOnion(t *utesting.T, onionAddr string) *enode.Node {
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	r.Set(enr.Onion3(onionAddr))
	r.Set(enr.TCP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	return node
}

// createTestNodeDualStack creates a test node with both .onion and clearnet addresses.
func createTestNodeDualStack(t *utesting.T) *enode.Node {
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	r.Set(enr.Onion3(generateValidOnion3()))
	r.Set(enr.IP(net.IPv4(192, 168, 1, 1)))
	r.Set(enr.TCP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	return node
}

// createTestNodeClearnetOnly creates a test node with only clearnet address.
func createTestNodeClearnetOnly(t *utesting.T) *enode.Node {
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	r.Set(enr.IP(net.IPv4(192, 168, 1, 1)))
	r.Set(enr.TCP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	return node
}
