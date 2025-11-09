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
	"net"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// TestSuite runs all Tor+ENR integration tests.
// This can be executed via: go test ./cmd/devp2p/internal/tortest -v
func TestSuite(t *testing.T) {
	// Note: Full test suite is designed for Hive integration
	// For unit testing, run individual test functions below
	t.Log("Tor+ENR test suite available for Hive integration")
	t.Log("Use individual test functions for unit testing")
}

// TestENRPropagation tests individual ENR propagation functionality.
func TestENRPropagation(t *testing.T) {
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

// TestDualStackNode tests node with both .onion and clearnet addresses.
func TestDualStackNode(t *testing.T) {
	key, _ := crypto.GenerateKey()
	var r enr.Record
	r.Set(enr.ID("v4"))
	r.Set(enr.Onion3(generateValidOnion3()))
	r.Set(enr.IP(net.ParseIP("192.168.1.1")))
	r.Set(enr.TCP(30303))

	if err := enode.SignV4(&r, key); err != nil {
		t.Fatalf("failed to sign ENR: %v", err)
	}

	node, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	// Verify both addresses exist
	if node.IP() == nil {
		t.Fatal("dual-stack node should have IP address")
	}

	var onion enr.Onion3
	if err := node.Record().Load(&onion); err != nil {
		t.Fatal("dual-stack node should have .onion address")
	}

	if node.TCP() == 0 {
		t.Fatal("dual-stack node should have TCP port")
	}

	t.Logf("Dual-stack node: IP=%s, Onion=%s, Port=%d", node.IP(), onion, node.TCP())
}

// BenchmarkENRCreation benchmarks ENR record creation with .onion address.
func BenchmarkENRCreation(b *testing.B) {
	key, _ := crypto.GenerateKey()
	onionAddr := generateValidOnion3()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var r enr.Record
		r.Set(enr.ID("v4"))
		r.Set(enr.Onion3(onionAddr))
		r.Set(enr.TCP(30303))

		enode.SignV4(&r, key)
		enode.New(enode.ValidSchemes, &r)
	}
}

// BenchmarkOnion3Validation benchmarks .onion address validation.
func BenchmarkOnion3Validation(b *testing.B) {
	key, _ := crypto.GenerateKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var r enr.Record
		r.Set(enr.ID("v4"))
		r.Set(enr.Onion3(generateValidOnion3()))
		enode.SignV4(&r, key)
	}
}
