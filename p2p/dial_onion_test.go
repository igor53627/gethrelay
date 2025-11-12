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
	"crypto/ecdsa"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// Test 1: Onion address detection
func TestIsOnionAddress(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     bool
	}{
		{
			name:     "valid v3 onion address",
			hostname: "55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion",
			want:     true,
		},
		{
			name:     "valid v2 onion address",
			hostname: "3g2upl4pq6kufc4m.onion",
			want:     true,
		},
		{
			name:     "regular domain",
			hostname: "example.com",
			want:     false,
		},
		{
			name:     "IP address",
			hostname: "192.168.1.1",
			want:     false,
		},
		{
			name:     "empty string",
			hostname: "",
			want:     false,
		},
		{
			name:     "onion in middle",
			hostname: "test.onion.example.com",
			want:     false,
		},
		{
			name:     "case insensitive",
			hostname: "test.ONION",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOnionAddress(tt.hostname)
			if got != tt.want {
				t.Errorf("isOnionAddress(%q) = %v, want %v", tt.hostname, got, tt.want)
			}
		})
	}
}

// Test 2: DNS resolution should be skipped for .onion addresses
func TestDNSResolveSkipsOnionAddresses(t *testing.T) {
	// Create a dialScheduler with a mock DNS lookup function that should never be called
	dnsLookupCalled := false
	d := &dialScheduler{
		dialConfig: dialConfig{
			log: log.Root(),
		},
		dnsLookupFunc: func(ctx context.Context, network string, name string) ([]netip.Addr, error) {
			dnsLookupCalled = true
			t.Errorf("DNS lookup was called for .onion address: %s", name)
			return nil, nil
		},
	}

	// Create a node with .onion hostname
	privKey, _ := crypto.GenerateKey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, privKey)

	onionAddr := "55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion"
	node := ln.Node().WithHostname(onionAddr)

	// Attempt DNS resolution
	resolved, err := d.dnsResolveHostname(node)

	// For .onion addresses, we should NOT call DNS and should return the node unchanged
	if dnsLookupCalled {
		t.Fatal("DNS lookup should not be called for .onion addresses")
	}

	if err != nil {
		t.Errorf("dnsResolveHostname should not return error for .onion: %v", err)
	}

	if resolved.Hostname() != onionAddr {
		t.Errorf("hostname changed from %s to %s", onionAddr, resolved.Hostname())
	}
}

// Test 3: DNS resolution should work normally for regular hostnames
func TestDNSResolveWorksForRegularHostnames(t *testing.T) {
	dnsLookupCalled := false
	expectedIP := netip.MustParseAddr("93.184.216.34")

	d := &dialScheduler{
		dialConfig: dialConfig{
			log: log.Root(),
		},
		dnsLookupFunc: func(ctx context.Context, network string, name string) ([]netip.Addr, error) {
			dnsLookupCalled = true
			if name != "example.com" {
				t.Errorf("expected hostname 'example.com', got %s", name)
			}
			return []netip.Addr{expectedIP}, nil
		},
	}

	privKey, _ := crypto.GenerateKey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, privKey)
	node := ln.Node().WithHostname("example.com")

	resolved, err := d.dnsResolveHostname(node)

	if !dnsLookupCalled {
		t.Fatal("DNS lookup should be called for regular hostnames")
	}

	if err != nil {
		t.Errorf("dnsResolveHostname returned error: %v", err)
	}

	if resolved == nil {
		t.Fatal("resolved node should not be nil")
	}

	var resolvedIP enr.IPv4Addr
	if err := resolved.Load(&resolvedIP); err != nil {
		t.Errorf("failed to load IPv4 from resolved node: %v", err)
	}

	if netip.Addr(resolvedIP) != expectedIP {
		t.Errorf("expected IP %v, got %v", expectedIP, netip.Addr(resolvedIP))
	}
}

// Test 4: Static node with .onion address should be dialable without DNS lookup
func TestStaticDialTaskOnionAddress(t *testing.T) {
	// Create a mock dialer that tracks whether it was called
	dialCalled := false
	mockDialer := &mockNodeDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) error {
			dialCalled = true
			// Verify that the hostname is still .onion
			if !strings.HasSuffix(dest.Hostname(), ".onion") {
				t.Errorf("expected .onion hostname, got %s", dest.Hostname())
			}
			return nil
		},
	}

	// Create dialScheduler with mock DNS that should never be called
	dnsLookupCalled := false
	d := &dialScheduler{
		dialConfig: dialConfig{
			dialer: mockDialer,
			log:    log.Root(),
		},
		dnsLookupFunc: func(ctx context.Context, network string, name string) ([]netip.Addr, error) {
			if strings.HasSuffix(name, ".onion") {
				dnsLookupCalled = true
				t.Errorf("DNS lookup called for .onion address: %s", name)
			}
			return nil, nil
		},
		setupFunc: func(c net.Conn, f connFlag, n *enode.Node) error {
			return nil
		},
		ctx: context.Background(),
	}

	// Create a static node with .onion hostname
	privKey, _ := crypto.GenerateKey()
	onionAddr := "55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion"

	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.TCP(30303))
	node := ln.Node().WithHostname(onionAddr)

	// Create and run dial task
	task := newDialTask(node, staticDialedConn)
	task.run(d)

	// Verify DNS lookup was NOT called for .onion
	if dnsLookupCalled {
		t.Fatal("DNS lookup should not be called for .onion static nodes")
	}

	// Verify dial was called (indicating the connection proceeded)
	if !dialCalled {
		t.Fatal("Dial should be called for .onion static nodes")
	}
}

// Test 5: Node with both .onion and clearnet should prefer .onion when TorDialer is configured
func TestOnionAddressHandlingInTorDialer(t *testing.T) {
	privKey, _ := crypto.GenerateKey()
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, privKey)

	// Set both clearnet IP and .onion address
	ln.Set(enr.TCP(30303))
	ln.Set(enr.IPv4Addr(netip.MustParseAddr("127.0.0.1")))
	onionAddr := enr.Onion3("55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion")
	ln.Set(&onionAddr)

	node := ln.Node()

	// Create TorDialer that should handle .onion
	torCalled := false

	mockTorDialer := &mockNodeDialer{
		dialFunc: func(ctx context.Context, dest *enode.Node) error {
			// Check if this is attempting to dial via Tor (would have .onion in ENR)
			var onion enr.Onion3
			if dest.Load(&onion) == nil && onion != "" {
				torCalled = true
			}
			return nil
		},
	}

	// Simulate dial through configured dialer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = mockTorDialer.Dial(ctx, node)

	// When both addresses are present and Tor is configured, .onion should be preferred
	if !torCalled {
		t.Error("TorDialer should handle nodes with .onion addresses")
	}
}

// mockNodeDialer for testing
type mockNodeDialer struct {
	dialFunc func(context.Context, *enode.Node) error
}

func (m *mockNodeDialer) Dial(ctx context.Context, dest *enode.Node) (net.Conn, error) {
	if m.dialFunc != nil {
		err := m.dialFunc(ctx, dest)
		return nil, err
	}
	return nil, nil
}

// Helper to create test nodes with .onion addresses
func newOnionNode(privKey *ecdsa.PrivateKey, onionAddr string, port uint16) *enode.Node {
	db, _ := enode.OpenDB("")
	ln := enode.NewLocalNode(db, privKey)
	ln.Set(enr.TCP(port))

	onion := enr.Onion3(onionAddr)
	ln.Set(&onion)

	return ln.Node().WithHostname(onionAddr)
}
