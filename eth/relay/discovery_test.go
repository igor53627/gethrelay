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
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetOnionAddress_Valid tests setting a valid Tor v3 address in local node ENR.
func TestSetOnionAddress_Valid(t *testing.T) {
	// Setup
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)

	// Test with valid onion address
	validOnion := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	err := SetOnionAddress(ln, validOnion)
	require.NoError(t, err, "SetOnionAddress should succeed with valid address")

	// Verify it was set in ENR
	node := ln.Node()
	var onion enr.Onion3
	err = node.Load(&onion)
	require.NoError(t, err, "Should be able to load onion3 entry from ENR")
	assert.Equal(t, validOnion, string(onion), "Loaded onion address should match what was set")
}

// TestSetOnionAddress_Invalid tests that invalid addresses are rejected.
func TestSetOnionAddress_Invalid(t *testing.T) {
	// Setup
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)

	testCases := []struct {
		name    string
		address string
	}{
		{"too short", "short.onion"},
		{"wrong suffix", "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.invalid"},
		{"invalid characters", "VWWW6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"}, // uppercase invalid
		{"no suffix", "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd"},
		{"empty string", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetOnionAddress(ln, tc.address)
			assert.Error(t, err, "SetOnionAddress should reject invalid address: %s", tc.address)
		})
	}
}

// TestSetOnionAddress_NilLocalNode tests that nil LocalNode is handled.
func TestSetOnionAddress_NilLocalNode(t *testing.T) {
	err := SetOnionAddress(nil, "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	assert.Error(t, err, "SetOnionAddress should return error for nil LocalNode")
}

// TestGetPeerOnionAddress_Present tests retrieving an onion address from peer ENR.
func TestGetPeerOnionAddress_Present(t *testing.T) {
	// Setup: Create a node with onion address
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)

	// Set onion address
	validOnion := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	err := SetOnionAddress(ln, validOnion)
	require.NoError(t, err)

	// Get the node (peer)
	peer := ln.Node()

	// Test retrieval
	onionAddr, ok := GetPeerOnionAddress(peer)
	assert.True(t, ok, "GetPeerOnionAddress should return true when onion address exists")
	assert.Equal(t, validOnion, onionAddr, "Retrieved onion address should match")
}

// TestGetPeerOnionAddress_Missing tests that false is returned when no onion address exists.
func TestGetPeerOnionAddress_Missing(t *testing.T) {
	// Setup: Create a node WITHOUT onion address
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)
	peer := ln.Node()

	// Test retrieval
	onionAddr, ok := GetPeerOnionAddress(peer)
	assert.False(t, ok, "GetPeerOnionAddress should return false when no onion address exists")
	assert.Empty(t, onionAddr, "Returned address should be empty string when not present")
}

// TestGetPeerOnionAddress_NilPeer tests that nil peer is handled.
func TestGetPeerOnionAddress_NilPeer(t *testing.T) {
	onionAddr, ok := GetPeerOnionAddress(nil)
	assert.False(t, ok, "GetPeerOnionAddress should return false for nil peer")
	assert.Empty(t, onionAddr, "Returned address should be empty string for nil peer")
}

// TestOnionAddress_RoundTrip tests complete ENR round-trip: set → retrieve → verify.
func TestOnionAddress_RoundTrip(t *testing.T) {
	// Create two nodes: one local, one to simulate a peer
	db1, _ := enode.OpenDB("")
	defer db1.Close()
	key1, _ := crypto.GenerateKey()
	localNode := enode.NewLocalNode(db1, key1)

	// Set onion address on local node
	onionAddr := "3g2upl4pq6kufc4m3ptl6old3zc7rgslvizlt2aot3xnflwm66qrd2id.onion"
	err := SetOnionAddress(localNode, onionAddr)
	require.NoError(t, err, "Setting onion address should succeed")

	// Get the node record (simulating peer discovery)
	node := localNode.Node()

	// Retrieve the onion address as if we discovered this peer
	retrievedAddr, ok := GetPeerOnionAddress(node)
	require.True(t, ok, "Should be able to retrieve onion address")
	assert.Equal(t, onionAddr, retrievedAddr, "Round-trip should preserve onion address")

	// Verify ENR signature is valid (implicit in Node() call)
	// The fact that Node() succeeded means the ENR is valid and properly signed
	t.Logf("ENR record valid and signed, seq=%d", node.Seq())
}

// TestOnionAddress_Update tests that updating onion address works correctly.
func TestOnionAddress_Update(t *testing.T) {
	// Setup
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)

	// Set initial onion address
	onion1 := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	err := SetOnionAddress(ln, onion1)
	require.NoError(t, err)

	// Verify first address
	addr1, ok := GetPeerOnionAddress(ln.Node())
	require.True(t, ok)
	assert.Equal(t, onion1, addr1)

	// Update to new onion address (e.g., Tor service restarted)
	onion2 := "3g2upl4pq6kufc4m3ptl6old3zc7rgslvizlt2aot3xnflwm66qrd2id.onion"
	err = SetOnionAddress(ln, onion2)
	require.NoError(t, err)

	// Verify updated address
	addr2, ok := GetPeerOnionAddress(ln.Node())
	require.True(t, ok)
	assert.Equal(t, onion2, addr2, "Updated onion address should be reflected in ENR")
	assert.NotEqual(t, addr1, addr2, "Address should have changed")
}

// TestOnionAddress_MultipleEntries tests that onion address coexists with other ENR entries.
func TestOnionAddress_MultipleEntries(t *testing.T) {
	// Setup
	db, _ := enode.OpenDB("")
	defer db.Close()
	key, _ := crypto.GenerateKey()
	ln := enode.NewLocalNode(db, key)

	// Set various ENR entries
	ln.Set(enr.IPv4{127, 0, 0, 1})
	ln.Set(enr.UDP(30303))
	ln.Set(enr.TCP(30303))

	onionAddr := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"
	err := SetOnionAddress(ln, onionAddr)
	require.NoError(t, err)

	// Verify all entries are present
	node := ln.Node()

	var ip enr.IPv4
	require.NoError(t, node.Load(&ip))
	assert.Equal(t, enr.IPv4{127, 0, 0, 1}, ip)

	var udp enr.UDP
	require.NoError(t, node.Load(&udp))
	assert.Equal(t, enr.UDP(30303), udp)

	var tcp enr.TCP
	require.NoError(t, node.Load(&tcp))
	assert.Equal(t, enr.TCP(30303), tcp)

	retrievedOnion, ok := GetPeerOnionAddress(node)
	require.True(t, ok)
	assert.Equal(t, onionAddr, retrievedOnion)
}

// testNodeFromRecord creates a Node from a manually crafted ENR record (for advanced testing).
func testNodeFromRecord(t *testing.T, key *ecdsa.PrivateKey, entries ...enr.Entry) *enode.Node {
	var r enr.Record
	for _, e := range entries {
		r.Set(e)
	}
	if err := enode.SignV4(&r, key); err != nil {
		t.Fatal(err)
	}
	n, err := enode.New(enode.ValidSchemes, &r)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestGetPeerOnionAddress_DirectLoad tests loading from a manually crafted node.
func TestGetPeerOnionAddress_DirectLoad(t *testing.T) {
	key, _ := crypto.GenerateKey()
	onionAddr := "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion"

	// Create node with onion address entry
	node := testNodeFromRecord(t, key, enr.Onion3(onionAddr))

	// Retrieve it
	retrieved, ok := GetPeerOnionAddress(node)
	require.True(t, ok)
	assert.Equal(t, onionAddr, retrieved)
}
