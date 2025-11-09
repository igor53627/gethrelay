// Copyright 2017 The go-ethereum Authors
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

package enr

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

func randomString(strlen int) string {
	b := make([]byte, strlen)
	rnd.Read(b)
	return string(b)
}

// TestGetSetID tests encoding/decoding and setting/getting of the ID key.
func TestGetSetID(t *testing.T) {
	id := ID("someid")
	var r Record
	r.Set(id)

	var id2 ID
	require.NoError(t, r.Load(&id2))
	assert.Equal(t, id, id2)
}

// TestGetSetIPv4 tests encoding/decoding and setting/getting of the IP key.
func TestGetSetIPv4(t *testing.T) {
	ip := IPv4{192, 168, 0, 3}
	var r Record
	r.Set(ip)

	var ip2 IPv4
	require.NoError(t, r.Load(&ip2))
	assert.Equal(t, ip, ip2)
}

// TestGetSetIPv6 tests encoding/decoding and setting/getting of the IP6 key.
func TestGetSetIPv6(t *testing.T) {
	ip := IPv6{0x20, 0x01, 0x48, 0x60, 0, 0, 0x20, 0x01, 0, 0, 0, 0, 0, 0, 0x00, 0x68}
	var r Record
	r.Set(ip)

	var ip2 IPv6
	require.NoError(t, r.Load(&ip2))
	assert.Equal(t, ip, ip2)
}

// TestGetSetUDP tests encoding/decoding and setting/getting of the UDP key.
func TestGetSetUDP(t *testing.T) {
	port := UDP(30309)
	var r Record
	r.Set(port)

	var port2 UDP
	require.NoError(t, r.Load(&port2))
	assert.Equal(t, port, port2)
}

func TestLoadErrors(t *testing.T) {
	var r Record
	ip4 := IPv4{127, 0, 0, 1}
	r.Set(ip4)

	// Check error for missing keys.
	var udp UDP
	err := r.Load(&udp)
	if !IsNotFound(err) {
		t.Error("IsNotFound should return true for missing key")
	}
	assert.Equal(t, &KeyError{Key: udp.ENRKey(), Err: errNotFound}, err)

	// Check error for invalid keys.
	var list []uint
	err = r.Load(WithEntry(ip4.ENRKey(), &list))
	kerr, ok := err.(*KeyError)
	if !ok {
		t.Fatalf("expected KeyError, got %T", err)
	}
	assert.Equal(t, kerr.Key, ip4.ENRKey())
	assert.Error(t, kerr.Err)
	if IsNotFound(err) {
		t.Error("IsNotFound should return false for decoding errors")
	}
}

// TestSortedGetAndSet tests that Set produced a sorted pairs slice.
func TestSortedGetAndSet(t *testing.T) {
	type pair struct {
		k string
		v uint32
	}

	for _, tt := range []struct {
		input []pair
		want  []pair
	}{
		{
			input: []pair{{"a", 1}, {"c", 2}, {"b", 3}},
			want:  []pair{{"a", 1}, {"b", 3}, {"c", 2}},
		},
		{
			input: []pair{{"a", 1}, {"c", 2}, {"b", 3}, {"d", 4}, {"a", 5}, {"bb", 6}},
			want:  []pair{{"a", 5}, {"b", 3}, {"bb", 6}, {"c", 2}, {"d", 4}},
		},
		{
			input: []pair{{"c", 2}, {"b", 3}, {"d", 4}, {"a", 5}, {"bb", 6}},
			want:  []pair{{"a", 5}, {"b", 3}, {"bb", 6}, {"c", 2}, {"d", 4}},
		},
	} {
		var r Record
		for _, i := range tt.input {
			r.Set(WithEntry(i.k, &i.v))
		}
		for i, w := range tt.want {
			// set got's key from r.pair[i], so that we preserve order of pairs
			got := pair{k: r.pairs[i].k}
			assert.NoError(t, r.Load(WithEntry(w.k, &got.v)))
			assert.Equal(t, w, got)
		}
	}
}

// TestDirty tests record signature removal on setting of new key/value pair in record.
func TestDirty(t *testing.T) {
	var r Record

	if _, err := rlp.EncodeToBytes(r); err != errEncodeUnsigned {
		t.Errorf("expected errEncodeUnsigned, got %#v", err)
	}

	require.NoError(t, signTest([]byte{5}, &r))
	if len(r.signature) == 0 {
		t.Error("record is not signed")
	}
	_, err := rlp.EncodeToBytes(r)
	assert.NoError(t, err)

	r.SetSeq(3)
	if len(r.signature) != 0 {
		t.Error("signature still set after modification")
	}
	if _, err := rlp.EncodeToBytes(r); err != errEncodeUnsigned {
		t.Errorf("expected errEncodeUnsigned, got %#v", err)
	}
}

func TestSize(t *testing.T) {
	var r Record

	// Empty record size is 3 bytes.
	// Unsigned records cannot be encoded, but they could, the encoding
	// would be [ 0, 0 ] -> 0xC28080.
	assert.Equal(t, uint64(3), r.Size())

	// Add one attribute. The size increases to 5, the encoding
	// would be [ 0, 0, "k", "v" ] -> 0xC58080C26B76.
	r.Set(WithEntry("k", "v"))
	assert.Equal(t, uint64(5), r.Size())

	// Now add a signature.
	nodeid := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	signTest(nodeid, &r)
	assert.Equal(t, uint64(45), r.Size())
	enc, _ := rlp.EncodeToBytes(&r)
	if r.Size() != uint64(len(enc)) {
		t.Error("Size() not equal encoded length", len(enc))
	}
	if r.Size() != computeSize(&r) {
		t.Error("Size() not equal computed size", computeSize(&r))
	}
}

func TestSeq(t *testing.T) {
	var r Record

	assert.Equal(t, uint64(0), r.Seq())
	r.Set(UDP(1))
	assert.Equal(t, uint64(0), r.Seq())
	signTest([]byte{5}, &r)
	assert.Equal(t, uint64(0), r.Seq())
	r.Set(UDP(2))
	assert.Equal(t, uint64(1), r.Seq())
}

// TestGetSetOverwrite tests value overwrite when setting a new value with an existing key in record.
func TestGetSetOverwrite(t *testing.T) {
	var r Record

	ip := IPv4{192, 168, 0, 3}
	r.Set(ip)

	ip2 := IPv4{192, 168, 0, 4}
	r.Set(ip2)

	var ip3 IPv4
	require.NoError(t, r.Load(&ip3))
	assert.Equal(t, ip2, ip3)
}

// TestSignEncodeAndDecode tests signing, RLP encoding and RLP decoding of a record.
func TestSignEncodeAndDecode(t *testing.T) {
	var r Record
	r.Set(UDP(30303))
	r.Set(IPv4{127, 0, 0, 1})
	require.NoError(t, signTest([]byte{5}, &r))

	blob, err := rlp.EncodeToBytes(r)
	require.NoError(t, err)

	var r2 Record
	require.NoError(t, rlp.DecodeBytes(blob, &r2))
	assert.Equal(t, r, r2)

	blob2, err := rlp.EncodeToBytes(r2)
	require.NoError(t, err)
	assert.Equal(t, blob, blob2)
}

// TestRecordTooBig tests that records bigger than SizeLimit bytes cannot be signed.
func TestRecordTooBig(t *testing.T) {
	var r Record
	key := randomString(10)

	// set a big value for random key, expect error
	r.Set(WithEntry(key, randomString(SizeLimit)))
	if err := signTest([]byte{5}, &r); err != errTooBig {
		t.Fatalf("expected to get errTooBig, got %#v", err)
	}

	// set an acceptable value for random key, expect no error
	r.Set(WithEntry(key, randomString(100)))
	require.NoError(t, signTest([]byte{5}, &r))
}

// This checks that incomplete RLP inputs are handled correctly.
func TestDecodeIncomplete(t *testing.T) {
	type decTest struct {
		input []byte
		err   error
	}
	tests := []decTest{
		{[]byte{0xC0}, errIncompleteList},
		{[]byte{0xC1, 0x1}, errIncompleteList},
		{[]byte{0xC2, 0x1, 0x2}, nil},
		{[]byte{0xC3, 0x1, 0x2, 0x3}, errIncompletePair},
		{[]byte{0xC4, 0x1, 0x2, 0x3, 0x4}, nil},
		{[]byte{0xC5, 0x1, 0x2, 0x3, 0x4, 0x5}, errIncompletePair},
	}
	for _, test := range tests {
		var r Record
		err := rlp.DecodeBytes(test.input, &r)
		if err != test.err {
			t.Errorf("wrong error for %X: %v", test.input, err)
		}
	}
}

// TestSignEncodeAndDecodeRandom tests encoding/decoding of records containing random key/value pairs.
func TestSignEncodeAndDecodeRandom(t *testing.T) {
	var r Record

	// random key/value pairs for testing
	pairs := map[string]uint32{}
	for i := 0; i < 10; i++ {
		key := randomString(7)
		value := rnd.Uint32()
		pairs[key] = value
		r.Set(WithEntry(key, &value))
	}

	require.NoError(t, signTest([]byte{5}, &r))

	enc, err := rlp.EncodeToBytes(r)
	require.NoError(t, err)
	require.Equal(t, uint64(len(enc)), r.Size())
	require.Equal(t, uint64(len(enc)), computeSize(&r))

	for k, v := range pairs {
		desc := fmt.Sprintf("key %q", k)
		var got uint32
		buf := WithEntry(k, &got)
		require.NoError(t, r.Load(buf), desc)
		require.Equal(t, v, got, desc)
	}
}

type testSig struct{}

type testID []byte

func (id testID) ENRKey() string { return "testid" }

func signTest(id []byte, r *Record) error {
	r.Set(ID("test"))
	r.Set(testID(id))
	return r.SetSig(testSig{}, makeTestSig(id, r.Seq()))
}

func makeTestSig(id []byte, seq uint64) []byte {
	sig := make([]byte, 8, len(id)+8)
	binary.BigEndian.PutUint64(sig[:8], seq)
	sig = append(sig, id...)
	return sig
}

func (testSig) Verify(r *Record, sig []byte) error {
	var id []byte
	if err := r.Load((*testID)(&id)); err != nil {
		return err
	}
	if !bytes.Equal(sig, makeTestSig(id, r.Seq())) {
		return ErrInvalidSig
	}
	return nil
}

func (testSig) NodeAddr(r *Record) []byte {
	var id []byte
	if err := r.Load((*testID)(&id)); err != nil {
		return nil
	}
	return id
}

// TestGetSetOnion3 tests encoding/decoding and setting/getting of the Onion3 key.
func TestGetSetOnion3(t *testing.T) {
	// Valid Tor v3 address (56 base32 chars + .onion)
	onion := Onion3("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	var r Record
	r.Set(onion)

	var onion2 Onion3
	require.NoError(t, r.Load(&onion2))
	assert.Equal(t, onion, onion2)
}

// TestOnion3ENRKey tests that Onion3.ENRKey() returns "onion3".
func TestOnion3ENRKey(t *testing.T) {
	onion := Onion3("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")
	assert.Equal(t, "onion3", onion.ENRKey())
}

// TestOnion3RoundTrip tests that RLP encoding and decoding preserves the Onion3 address.
func TestOnion3RoundTrip(t *testing.T) {
	onion := Onion3("vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion")

	// Encode to RLP
	var buf bytes.Buffer
	err := onion.EncodeRLP(&buf)
	require.NoError(t, err)

	// Decode from RLP
	var decoded Onion3
	err = rlp.DecodeBytes(buf.Bytes(), &decoded)
	require.NoError(t, err)

	// Verify round-trip preserves the address
	assert.Equal(t, onion, decoded)
}

// TestOnion3DecodeErrors tests RLP decoding error handling.
func TestOnion3DecodeErrors(t *testing.T) {
	// Test decoding invalid RLP data
	invalidRLP := []byte{0x00} // Invalid RLP
	var onion Onion3
	err := rlp.DecodeBytes(invalidRLP, &onion)
	assert.Error(t, err)
}

// TestOnion3InvalidFormats tests that invalid Onion3 formats are rejected.
func TestOnion3InvalidFormats(t *testing.T) {
	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "valid v3 address",
			address: "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd.onion",
			wantErr: false,
		},
		{
			name:    "too short (v2 address)",
			address: "3g2upl4pq6kufc4m.onion",
			wantErr: true,
		},
		{
			name:    "missing .onion suffix",
			address: "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyyd",
			wantErr: true,
		},
		{
			name:    "too long",
			address: "vww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyydextra.onion",
			wantErr: true,
		},
		{
			name:    "invalid base32 characters (uppercase)",
			address: "VWW6YBAL4BD7SZMGNCYRUUCPGFKQAHZDDI37KTCEO3AH7NGMCOPNPYYD.onion",
			wantErr: true,
		},
		{
			name:    "invalid base32 characters (numbers 0, 1, 8, 9)",
			address: "0ww6ybal4bd7szmgncyruucpgfkqahzddi37ktceo3ah7ngmcopnpyy1.onion",
			wantErr: true,
		},
		{
			name:    "empty string",
			address: "",
			wantErr: true,
		},
		{
			name:    "just .onion",
			address: ".onion",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onion := Onion3(tt.address)

			// Try encoding - should validate
			var buf bytes.Buffer
			err := onion.EncodeRLP(&buf)

			if tt.wantErr {
				assert.Error(t, err, "expected error for invalid address: %s", tt.address)
			} else {
				assert.NoError(t, err, "expected no error for valid address: %s", tt.address)

				// If encoding succeeded, verify decoding also works
				var decoded Onion3
				err = rlp.DecodeBytes(buf.Bytes(), &decoded)
				assert.NoError(t, err)
				assert.Equal(t, onion, decoded)
			}
		})
	}
}

// TestOnion3StreamDecodeError tests RLP stream decode errors for Onion3.
func TestOnion3StreamDecodeError(t *testing.T) {
	// Test invalid RLP stream (not a string)
	var onion Onion3
	stream := rlp.NewStream(bytes.NewReader([]byte{0xC0}), 0) // List instead of string
	err := onion.DecodeRLP(stream)
	if err == nil {
		t.Fatal("expected error decoding list as Onion3")
	}
}

// TestOnion3InENRRecord tests Onion3 in actual ENR record operations.
func TestOnion3InENRRecord(t *testing.T) {
	validOnion := Onion3("abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion")

	// Test Set in record
	var r Record
	r.Set(validOnion)
	require.NoError(t, signTest([]byte{5}, &r)) // Sign the record

	// Test Load from record
	var loaded Onion3
	if err := r.Load(&loaded); err != nil {
		t.Fatalf("failed to load Onion3 from record: %v", err)
	}

	if loaded != validOnion {
		t.Errorf("loaded value mismatch: got %s, want %s", loaded, validOnion)
	}

	// Test roundtrip through RLP encoding
	blob, err := rlp.EncodeToBytes(r)
	if err != nil {
		t.Fatalf("failed to encode record: %v", err)
	}

	var r2 Record
	if err := rlp.DecodeBytes(blob, &r2); err != nil {
		t.Fatalf("failed to decode record: %v", err)
	}

	var loaded2 Onion3
	if err := r2.Load(&loaded2); err != nil {
		t.Fatalf("failed to load Onion3 from decoded record: %v", err)
	}

	if loaded2 != validOnion {
		t.Errorf("roundtrip value mismatch: got %s, want %s", loaded2, validOnion)
	}
}

// TestOnion3WithOtherENREntries tests Onion3 alongside other ENR entries.
func TestOnion3WithOtherENREntries(t *testing.T) {
	var r Record
	r.Set(Onion3("abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuv22.onion"))
	r.Set(IPv4{192, 168, 1, 1})
	r.Set(TCP(30303))
	r.Set(UDP(30303))

	// Verify all entries can be loaded
	var onion Onion3
	var ip IPv4
	var tcp TCP
	var udp UDP

	if err := r.Load(&onion); err != nil {
		t.Fatalf("failed to load Onion3: %v", err)
	}
	if err := r.Load(&ip); err != nil {
		t.Fatalf("failed to load IPv4: %v", err)
	}
	if err := r.Load(&tcp); err != nil {
		t.Fatalf("failed to load TCP: %v", err)
	}
	if err := r.Load(&udp); err != nil {
		t.Fatalf("failed to load UDP: %v", err)
	}
}
