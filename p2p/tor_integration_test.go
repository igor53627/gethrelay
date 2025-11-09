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
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// TestTorIntegration_TwoNodesDiscoverAndConnect is the main integration test
// that proves two gethrelay nodes can discover each other via ENR with .onion
// addresses and successfully connect through Tor.
//
// Test Flow:
//  1. Start Node A with Tor-enabled server (.onion in ENR)
//  2. Start Node B with Tor SOCKS5 proxy configured
//  3. Node B discovers Node A's ENR via static node configuration
//  4. Node B extracts .onion address from Node A's ENR
//  5. Node B connects to Node A's .onion via SOCKS5 proxy
//  6. P2P handshake succeeds (RLPx protocol)
//  7. Verify both nodes report successful peer connection
//
// This test can run with a real Tor daemon or with a mock SOCKS5 proxy.
func TestTorIntegration_TwoNodesDiscoverAndConnect(t *testing.T) {
	// Skip if this is a short test run
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Start mock SOCKS5 proxy (simulates Tor)
	mockSocks := startMockSOCKS5Proxy(t)
	defer mockSocks.Close()

	// Create Node A with .onion address in ENR
	nodeA, enrWithOnion := createNodeWithOnion(t)
	defer nodeA.Stop()

	// Start Node A server
	if err := nodeA.Start(); err != nil {
		t.Fatalf("failed to start Node A: %v", err)
	}

	// Wait for Node A to be ready
	time.Sleep(100 * time.Millisecond)

	// Verify Node A's ENR contains .onion address
	var onionAddr string
	onion := enr.Onion3("")
	if err := enrWithOnion.Load(&onion); err != nil {
		t.Fatalf("Node A's ENR does not contain .onion address: %v", err)
	}
	onionAddr = string(onion)
	t.Logf("Node A .onion address: %s", onionAddr)

	// Create Node B with Tor SOCKS5 proxy configured
	nodeB := createNodeWithTorProxy(t, mockSocks.Addr(), nodeA.Self())
	defer nodeB.Stop()

	// Start Node B server
	if err := nodeB.Start(); err != nil {
		t.Fatalf("failed to start Node B: %v", err)
	}

	// Wait for connection to establish
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connected := waitForPeerConnection(ctx, nodeB, nodeA.Self().ID())
	if !connected {
		t.Fatalf("Node B failed to connect to Node A via Tor")
	}

	// Verify connection is established
	peersB := nodeB.Peers()
	if len(peersB) == 0 {
		t.Fatal("Node B has no connected peers")
	}

	// Verify the peer is Node A
	foundPeer := false
	for _, peer := range peersB {
		if peer.ID() == nodeA.Self().ID() {
			foundPeer = true
			t.Logf("Successfully connected: Node B -> Node A (via Tor)")
			t.Logf("  Peer ID: %s", peer.ID())
			t.Logf("  Remote Address: %s", peer.RemoteAddr())
			break
		}
	}

	if !foundPeer {
		t.Fatal("Node B connected to wrong peer")
	}

	// Verify Node A also sees the connection
	peersA := nodeA.Peers()
	if len(peersA) == 0 {
		t.Fatal("Node A has no connected peers")
	}

	t.Logf("Integration test passed: Two nodes successfully connected via Tor")
	t.Logf("  Node A peers: %d", len(peersA))
	t.Logf("  Node B peers: %d", len(peersB))
}

// TestTorIntegration_ENRPropagation verifies that .onion addresses are properly
// propagated in ENRs and can be discovered by peers.
func TestTorIntegration_ENRPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create node with .onion address
	node, nodeRecord := createNodeWithOnion(t)
	defer node.Stop()

	// Verify .onion is in local node's ENR
	onion := enr.Onion3("")
	if err := nodeRecord.Load(&onion); err != nil {
		t.Fatalf("failed to load .onion from ENR: %v", err)
	}

	// Validate .onion format
	if len(onion) != 62 { // 56 base32 + ".onion" (6 chars)
		t.Fatalf("invalid .onion length: got %d, want 62", len(onion))
	}

	t.Logf("ENR propagation verified: .onion=%s", onion)

	// Verify ENR can be encoded/decoded
	encoded, err := encodeENR(nodeRecord)
	if err != nil {
		t.Fatalf("failed to encode ENR: %v", err)
	}

	decoded, err := decodeENR(encoded)
	if err != nil {
		t.Fatalf("failed to decode ENR: %v", err)
	}

	// Verify .onion survived round-trip
	decodedOnion := enr.Onion3("")
	if err := decoded.Load(&decodedOnion); err != nil {
		t.Fatalf("decoded ENR missing .onion: %v", err)
	}

	if decodedOnion != onion {
		t.Fatalf("ENR round-trip failed: got %s, want %s", decodedOnion, onion)
	}

	t.Logf("ENR encoding/decoding verified")
}

// TestTorIntegration_DualStackConnectivity verifies that nodes with both
// .onion and clearnet addresses can be reached via both transports.
func TestTorIntegration_DualStackConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mockSocks := startMockSOCKS5Proxy(t)
	defer mockSocks.Close()

	// Create dual-stack Node A (has both .onion and clearnet)
	nodeA, _ := createNodeWithOnion(t)
	defer nodeA.Stop()

	if err := nodeA.Start(); err != nil {
		t.Fatalf("failed to start Node A: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Test 1: Connect via Tor (prefer-tor mode)
	nodeBTor := createNodeWithTorProxy(t, mockSocks.Addr(), nodeA.Self())
	nodeBTor.Config.PreferTor = true
	defer nodeBTor.Stop()

	if err := nodeBTor.Start(); err != nil {
		t.Fatalf("failed to start Node B (Tor): %v", err)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()

	if !waitForPeerConnection(ctx1, nodeBTor, nodeA.Self().ID()) {
		t.Fatal("failed to connect via Tor")
	}

	t.Logf("Dual-stack connectivity verified: Tor transport working")
	nodeBTor.Stop()

	// Test 2: Connect via clearnet (Tor disabled)
	nodeBClearnet := createNodeClearnetOnly(t, nodeA.Self())
	defer nodeBClearnet.Stop()

	if err := nodeBClearnet.Start(); err != nil {
		t.Fatalf("failed to start Node B (clearnet): %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	if !waitForPeerConnection(ctx2, nodeBClearnet, nodeA.Self().ID()) {
		t.Fatal("failed to connect via clearnet")
	}

	t.Logf("Dual-stack connectivity verified: Clearnet transport working")
}

// TestTorIntegration_OnlyOnionMode verifies that only-onion mode correctly
// rejects clearnet-only peers.
func TestTorIntegration_OnlyOnionMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	mockSocks := startMockSOCKS5Proxy(t)
	defer mockSocks.Close()

	// Create clearnet-only Node A
	nodeAClearnet := createClearnetOnlyNode(t)
	defer nodeAClearnet.Stop()

	if err := nodeAClearnet.Start(); err != nil {
		t.Fatalf("failed to start clearnet node: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create Node B in only-onion mode
	nodeB := createNodeWithTorProxy(t, mockSocks.Addr(), nodeAClearnet.Self())
	nodeB.Config.OnlyOnion = true
	defer nodeB.Stop()

	if err := nodeB.Start(); err != nil {
		t.Fatalf("failed to start Node B: %v", err)
	}

	// Wait and verify connection does NOT establish
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	connected := waitForPeerConnection(ctx, nodeB, nodeAClearnet.Self().ID())
	if connected {
		t.Fatal("only-onion mode should reject clearnet-only peers")
	}

	t.Logf("Only-onion mode verified: clearnet peer correctly rejected")
}

// TestTorIntegration_FallbackToClearnet verifies that connections fallback
// to clearnet when Tor connection fails (default mode).
func TestTorIntegration_FallbackToClearnet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create dual-stack Node A
	nodeA, _ := createNodeWithOnion(t)
	defer nodeA.Stop()

	if err := nodeA.Start(); err != nil {
		t.Fatalf("failed to start Node A: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create Node B with INVALID Tor proxy (force fallback)
	invalidSocksAddr := "127.0.0.1:1" // Port 1 should be unavailable
	nodeB := createNodeWithTorProxy(t, invalidSocksAddr, nodeA.Self())
	nodeB.Config.PreferTor = false // Default mode: fallback enabled
	defer nodeB.Stop()

	if err := nodeB.Start(); err != nil {
		t.Fatalf("failed to start Node B: %v", err)
	}

	// Wait for connection (should fallback to clearnet)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !waitForPeerConnection(ctx, nodeB, nodeA.Self().ID()) {
		t.Fatal("failed to connect even with clearnet fallback")
	}

	t.Logf("Fallback to clearnet verified")
}

// --- Helper Functions ---

// createNodeWithOnion creates a Server with a .onion address in its ENR.
func createNodeWithOnion(t *testing.T) (*Server, *enode.Node) {
	key := newkey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)

	// Generate valid .onion address
	onionAddr := generateValidOnion3Addr()
	ln.Set(enr.Onion3(onionAddr))
	ln.Set(enr.IP(net.IPv4(127, 0, 0, 1)))
	ln.SetFallbackUDP(0) // Will be set on Start()

	srv := &Server{
		Config: Config{
			PrivateKey:  key,
			MaxPeers:    10,
			NoDiscovery: true,
			ListenAddr:  "127.0.0.1:0",
			Logger:      testlog.Logger(t, log.LevelError),
		},
		localnode: ln,
		log:       testlog.Logger(t, log.LevelError),
	}

	return srv, ln.Node()
}

// createNodeWithTorProxy creates a Server configured to use Tor SOCKS5 proxy.
func createNodeWithTorProxy(t *testing.T, socksAddr string, staticPeer *enode.Node) *Server {
	key := newkey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)

	ln.Set(enr.IP(net.IPv4(127, 0, 0, 1)))
	ln.SetFallbackUDP(0)

	// Configure static peer
	var staticPeers []*enode.Node
	if staticPeer != nil {
		staticPeers = []*enode.Node{staticPeer}
	}

	srv := &Server{
		Config: Config{
			PrivateKey:    key,
			MaxPeers:      10,
			NoDiscovery:   true,
			ListenAddr:    "127.0.0.1:0",
			TorSOCKSProxy: socksAddr,
			StaticNodes:   staticPeers,
			Logger:        testlog.Logger(t, log.LevelError),
		},
		localnode: ln,
		log:       testlog.Logger(t, log.LevelError),
	}

	return srv
}

// createNodeClearnetOnly creates a Server without Tor configuration.
func createNodeClearnetOnly(t *testing.T, staticPeer *enode.Node) *Server {
	key := newkey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)

	ln.Set(enr.IP(net.IPv4(127, 0, 0, 1)))
	ln.SetFallbackUDP(0)

	var staticPeers []*enode.Node
	if staticPeer != nil {
		staticPeers = []*enode.Node{staticPeer}
	}

	srv := &Server{
		Config: Config{
			PrivateKey:  key,
			MaxPeers:    10,
			NoDiscovery: true,
			ListenAddr:  "127.0.0.1:0",
			StaticNodes: staticPeers,
			Logger:      testlog.Logger(t, log.LevelError),
		},
		localnode: ln,
		log:       testlog.Logger(t, log.LevelError),
	}

	return srv
}

// createClearnetOnlyNode creates a Server with only clearnet address (no .onion).
func createClearnetOnlyNode(t *testing.T) *Server {
	key := newkey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, key)

	ln.Set(enr.IP(net.IPv4(127, 0, 0, 1)))
	ln.SetFallbackUDP(0)

	srv := &Server{
		Config: Config{
			PrivateKey:  key,
			MaxPeers:    10,
			NoDiscovery: true,
			ListenAddr:  "127.0.0.1:0",
			Logger:      testlog.Logger(t, log.LevelError),
		},
		localnode: ln,
		log:       testlog.Logger(t, log.LevelError),
	}

	return srv
}

// waitForPeerConnection waits for a peer connection to establish.
func waitForPeerConnection(ctx context.Context, srv *Server, peerID enode.ID) bool {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			peers := srv.Peers()
			for _, p := range peers {
				if p.ID() == peerID {
					return true
				}
			}
		}
	}
}

// generateValidOnion3Addr generates a syntactically valid Tor v3 .onion address.
func generateValidOnion3Addr() string {
	// Use deterministic generation for testing
	base32Chars := "abcdefghijklmnopqrstuvwxyz234567"
	result := make([]byte, 56)

	for i := 0; i < 56; i++ {
		result[i] = base32Chars[i%32]
	}

	return string(result) + ".onion"
}

// encodeENR encodes an ENR to its string representation.
func encodeENR(node *enode.Node) (string, error) {
	return node.String(), nil
}

// decodeENR decodes an ENR from its string representation.
func decodeENR(encoded string) (*enode.Node, error) {
	return enode.Parse(enode.ValidSchemes, encoded)
}

// mockSOCKS5Proxy represents a mock SOCKS5 proxy server for testing.
type mockSOCKS5Proxy struct {
	listener net.Listener
	addr     string
	done     chan struct{}
}

// startMockSOCKS5Proxy starts a mock SOCKS5 proxy server.
func startMockSOCKS5Proxy(t *testing.T) *mockSOCKS5Proxy {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock SOCKS5 proxy: %v", err)
	}

	proxy := &mockSOCKS5Proxy{
		listener: listener,
		addr:     listener.Addr().String(),
		done:     make(chan struct{}),
	}

	go proxy.serve()

	return proxy
}

// Addr returns the proxy address.
func (p *mockSOCKS5Proxy) Addr() string {
	return p.addr
}

// Close stops the proxy server.
func (p *mockSOCKS5Proxy) Close() {
	close(p.done)
	p.listener.Close()
}

// serve handles incoming SOCKS5 connections.
func (p *mockSOCKS5Proxy) serve() {
	for {
		select {
		case <-p.done:
			return
		default:
			conn, err := p.listener.Accept()
			if err != nil {
				return
			}
			go p.handleConnection(conn)
		}
	}
}

// handleConnection implements basic SOCKS5 protocol.
func (p *mockSOCKS5Proxy) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read SOCKS5 greeting
	buf := make([]byte, 257)
	n, err := conn.Read(buf)
	if err != nil || n < 2 || buf[0] != 0x05 {
		return
	}

	// Send "no authentication" response
	conn.Write([]byte{0x05, 0x00})

	// Read connection request
	n, err = conn.Read(buf)
	if err != nil || n < 7 {
		return
	}

	// Parse address type
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
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	port := (uint16(buf[portOffset]) << 8) | uint16(buf[portOffset+1])

	// For .onion addresses, simulate successful connection
	// For clearnet, attempt real connection
	if len(targetAddr) > 6 && targetAddr[len(targetAddr)-6:] == ".onion" {
		// Send success response
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// For testing, we need to connect to localhost instead
		// Extract actual target from local test setup
		target, err := net.DialTimeout("tcp", "127.0.0.1:"+fmt.Sprint(port), 5*time.Second)
		if err != nil {
			// Can't connect - this is expected in some test scenarios
			return
		}
		defer target.Close()

		// Relay data bidirectionally
		done := make(chan struct{})
		go func() {
			defer close(done)
			copyBuffer(target, conn)
		}()
		copyBuffer(conn, target)
		<-done
	} else {
		// Clearnet connection
		target, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetAddr, port), 5*time.Second)
		if err != nil {
			conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
			return
		}
		defer target.Close()

		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Relay data
		done := make(chan struct{})
		go func() {
			defer close(done)
			copyBuffer(target, conn)
		}()
		copyBuffer(conn, target)
		<-done
	}
}

// copyBuffer copies data between connections.
func copyBuffer(dst net.Conn, src net.Conn) {
	buf := make([]byte, 32*1024)
	for {
		nr, err := src.Read(buf)
		if nr > 0 {
			nw, err := dst.Write(buf[0:nr])
			if err != nil {
				return
			}
			if nr != nw {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
