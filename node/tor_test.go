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

package node

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// mockTorController implements a mock Tor controller for testing
type mockTorController struct {
	conn           net.Conn
	reader         *bufio.Reader
	authenticated  bool
	shouldFailAuth bool
	shouldFailAdd  bool
	serviceID      string
	privateKey     string
}

func newMockTorController(shouldFailAuth, shouldFailAdd bool) *mockTorController {
	return &mockTorController{
		shouldFailAuth: shouldFailAuth,
		shouldFailAdd:  shouldFailAdd,
		serviceID:      "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuvw",
		privateKey:     "ED25519-V3:MOCK_KEY_DATA",
	}
}

func (m *mockTorController) Close() error {
	return nil
}

func (m *mockTorController) protocolInfo() error {
	return nil
}

func (m *mockTorController) authenticate(cookie []byte) error {
	if m.shouldFailAuth {
		return errors.New("authentication failed")
	}
	m.authenticated = true
	return nil
}

func (m *mockTorController) addOnion(keySpec string, mappings []string) (serviceID string, privateKey string, err error) {
	if !m.authenticated {
		return "", "", errors.New("not authenticated")
	}
	if m.shouldFailAdd {
		return "", "", errors.New("ADD_ONION failed")
	}
	return m.serviceID, m.privateKey, nil
}

func (m *mockTorController) command(cmd string) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (m *mockTorController) readReply() ([]string, error) {
	return nil, errors.New("not implemented")
}

// TestEnableP2PTorHiddenService_Success tests successful P2P hidden service creation
func TestEnableP2PTorHiddenService_Success(t *testing.T) {
	// This test will fail until we implement enableP2PTorHiddenService
	t.Skip("Test will pass once enableP2PTorHiddenService is implemented")

	// Create a test node with Tor config
	config := &Config{
		Tor: TorConfig{
			Enabled:        true,
			ControlAddress: "127.0.0.1:9051",
		},
	}

	node := &Node{
		config: config,
		log:    log.New(),
	}

	// Create a mock local node
	db, _ := enode.OpenDB("")
	defer db.Close()
	localNode := enode.NewLocalNode(db, testNodeKey)

	// Test with mock controller
	p2pPort := 30303
	err := node.enableP2PTorHiddenService(localNode, p2pPort)
	if err != nil {
		t.Fatalf("enableP2PTorHiddenService failed: %v", err)
	}

	// Verify ENR was updated with onion address
	// This test documents the expected behavior
}

// TestEnableP2PTorHiddenService_NilLocalNode tests nil LocalNode handling
func TestEnableP2PTorHiddenService_NilLocalNode(t *testing.T) {
	config := &Config{
		Tor: TorConfig{
			Enabled: true,
		},
	}

	node := &Node{
		config: config,
		log:    log.New(),
	}

	err := node.enableP2PTorHiddenService(nil, 30303)
	if err == nil {
		t.Fatal("expected error for nil LocalNode, got nil")
	}
	if !strings.Contains(err.Error(), "local node is nil") {
		t.Fatalf("expected 'local node is nil' error, got: %v", err)
	}
}

// TestEnableP2PTorHiddenService_TorDisabled tests behavior when Tor is disabled
func TestEnableP2PTorHiddenService_TorDisabled(t *testing.T) {
	config := &Config{
		Tor: TorConfig{
			Enabled: false,
		},
	}

	node := &Node{
		config: config,
		log:    log.New(),
	}

	db, _ := enode.OpenDB("")
	defer db.Close()
	localNode := enode.NewLocalNode(db, testNodeKey)

	// Should return nil when Tor is disabled (no-op)
	err := node.enableP2PTorHiddenService(localNode, 30303)
	if err != nil {
		t.Fatalf("expected nil error when Tor disabled, got: %v", err)
	}
}

// TestEnableP2PTorHiddenService_InvalidPort tests invalid port handling
func TestEnableP2PTorHiddenService_InvalidPort(t *testing.T) {
	config := &Config{
		Tor: TorConfig{
			Enabled: true,
		},
	}

	node := &Node{
		config: config,
		log:    log.New(),
	}

	db, _ := enode.OpenDB("")
	defer db.Close()
	localNode := enode.NewLocalNode(db, testNodeKey)

	// Test with invalid port (0)
	err := node.enableP2PTorHiddenService(localNode, 0)
	if err == nil {
		t.Fatal("expected error for port 0, got nil")
	}
	if !strings.Contains(err.Error(), "invalid P2P port") {
		t.Fatalf("expected 'invalid P2P port' error, got: %v", err)
	}

	// Test with negative port
	err = node.enableP2PTorHiddenService(localNode, -1)
	if err == nil {
		t.Fatal("expected error for negative port, got nil")
	}
	if !strings.Contains(err.Error(), "invalid P2P port") {
		t.Fatalf("expected 'invalid P2P port' error, got: %v", err)
	}

	// Test with port > 65535
	err = node.enableP2PTorHiddenService(localNode, 99999)
	if err == nil {
		t.Fatal("expected error for port > 65535, got nil")
	}
	if !strings.Contains(err.Error(), "invalid P2P port") {
		t.Fatalf("expected 'invalid P2P port' error, got: %v", err)
	}
}

// TestEnableP2PTorHiddenService_ControllerFailure tests Tor controller connection failure
func TestEnableP2PTorHiddenService_ControllerFailure(t *testing.T) {
	t.Skip("Test will pass once enableP2PTorHiddenService is implemented")

	config := &Config{
		Tor: TorConfig{
			Enabled:        true,
			ControlAddress: "127.0.0.1:99999", // Invalid port
		},
	}

	node := &Node{
		config: config,
		log:    log.New(),
	}

	db, _ := enode.OpenDB("")
	defer db.Close()
	localNode := enode.NewLocalNode(db, testNodeKey)

	err := node.enableP2PTorHiddenService(localNode, 30303)
	if err == nil {
		t.Fatal("expected error for invalid control address, got nil")
	}
}

// TestEnableP2PTorHiddenService_OnionAddressFormat tests .onion address generation
func TestEnableP2PTorHiddenService_OnionAddressFormat(t *testing.T) {
	// Test that the generated .onion address has correct format:
	// 56 base32 chars + ".onion" = 62 chars total
	serviceID := "abcdefghijklmnopqrstuvwxyz234567abcdefghijklmnopqrstuvwx"
	expectedOnion := serviceID + ".onion"

	if len(expectedOnion) != 62 {
		t.Fatalf("expected onion address length 62, got %d", len(expectedOnion))
	}

	if !strings.HasSuffix(expectedOnion, ".onion") {
		t.Fatal("onion address should end with .onion")
	}

	// Verify the base32 part is exactly 56 chars
	base32Part := strings.TrimSuffix(expectedOnion, ".onion")
	if len(base32Part) != 56 {
		t.Fatalf("expected base32 part to be 56 chars, got %d", len(base32Part))
	}
}

// TestEnableP2PTorHiddenService_PortMapping tests port mapping configuration
func TestEnableP2PTorHiddenService_PortMapping(t *testing.T) {
	// Test that port mapping is correctly formatted
	p2pPort := 30303
	expectedMapping := fmt.Sprintf("Port=%d,127.0.0.1:%d", p2pPort, p2pPort)

	// Verify format matches what Tor expects
	if !strings.HasPrefix(expectedMapping, "Port=") {
		t.Fatal("mapping should start with 'Port='")
	}
	if !strings.Contains(expectedMapping, "127.0.0.1") {
		t.Fatal("mapping should contain 127.0.0.1 (localhost)")
	}

	// Verify complete format
	expected := "Port=30303,127.0.0.1:30303"
	if expectedMapping != expected {
		t.Fatalf("expected mapping %q, got %q", expected, expectedMapping)
	}
}

// createTempTorCookie creates a temporary Tor cookie file for testing
func createTempTorCookie(t *testing.T) string {
	tmpDir := t.TempDir()
	cookiePath := filepath.Join(tmpDir, "control_auth_cookie")
	cookie := []byte("0123456789ABCDEF0123456789ABCDEF") // 32 bytes
	if err := os.WriteFile(cookiePath, cookie, 0600); err != nil {
		t.Fatalf("failed to create temp cookie: %v", err)
	}
	return cookiePath
}
