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
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/internal/utesting"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

// mockDialer implements p2p.NodeDialer for testing.
type mockDialer struct {
	dialFunc func(context.Context, *enode.Node) (net.Conn, error)
}

func (m *mockDialer) Dial(ctx context.Context, dest *enode.Node) (net.Conn, error) {
	if m.dialFunc != nil {
		return m.dialFunc(ctx, dest)
	}
	// Default: create mock connection
	return createMockConn(), nil
}

// createMockConn creates a mock network connection for testing.
func createMockConn() net.Conn {
	client, server := net.Pipe()
	go func() {
		io.Copy(io.Discard, server)
		server.Close()
	}()
	return client
}

// startMockSOCKS5Server starts a mock SOCKS5 proxy server for testing.
// Returns the server address and a close function.
func startMockSOCKS5Server(t *utesting.T) (string, func()) {
	return startMockSOCKS5ServerWithCallback(t, nil)
}

// startMockSOCKS5ServerWithCallback starts a mock SOCKS5 server with handshake callback.
func startMockSOCKS5ServerWithCallback(t *utesting.T, onHandshake func()) (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create mock SOCKS5 server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // Server closed
			}
			go handleSOCKS5Connection(t, conn, onHandshake)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

// startFailingSOCKS5Server starts a SOCKS5 server that rejects .onion connections.
func startFailingSOCKS5Server(t *utesting.T) (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create failing SOCKS5 server: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleFailingSOCKS5Connection(t, conn)
		}
	}()

	return listener.Addr().String(), func() { listener.Close() }
}

// handleSOCKS5Connection handles a SOCKS5 connection with basic handshake.
// Implements minimal SOCKS5 protocol (RFC 1928).
func handleSOCKS5Connection(t *utesting.T, conn net.Conn, onHandshake func()) {
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

	// Call handshake callback if provided
	if onHandshake != nil {
		onHandshake()
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

	// For .onion addresses, simulate successful connection
	if strings.HasSuffix(targetAddr, ".onion") {
		// Send success response
		conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

		// Keep connection open and echo data
		io.Copy(conn, conn)
	} else {
		// For clearnet, try actual connection
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

// handleFailingSOCKS5Connection handles connections but always fails.
func handleFailingSOCKS5Connection(t *utesting.T, conn net.Conn) {
	defer conn.Close()

	// Read version and auth
	buf := make([]byte, 257)
	n, err := conn.Read(buf)
	if err != nil || n < 2 {
		return
	}

	if buf[0] != 0x05 {
		return
	}

	// Send auth response
	conn.Write([]byte{0x05, 0x00})

	// Read connect request
	n, err = conn.Read(buf)
	if err != nil || n < 7 {
		return
	}

	// Always send connection refused (0x05)
	conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
}

// MockTorProxy represents a mock Tor SOCKS5 proxy for integration testing.
type MockTorProxy struct {
	listener  net.Listener
	addr      string
	closeChan chan struct{}
}

// NewMockTorProxy creates and starts a mock Tor proxy.
func NewMockTorProxy(t *utesting.T) *MockTorProxy {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create mock Tor proxy: %v", err)
	}

	proxy := &MockTorProxy{
		listener:  listener,
		addr:      listener.Addr().String(),
		closeChan: make(chan struct{}),
	}

	go proxy.serve(t)

	return proxy
}

// Addr returns the proxy address.
func (p *MockTorProxy) Addr() string {
	return p.addr
}

// Close stops the proxy server.
func (p *MockTorProxy) Close() {
	close(p.closeChan)
	p.listener.Close()
}

// serve handles incoming SOCKS5 connections.
func (p *MockTorProxy) serve(t *utesting.T) {
	for {
		select {
		case <-p.closeChan:
			return
		default:
			conn, err := p.listener.Accept()
			if err != nil {
				return
			}
			go handleSOCKS5Connection(t, conn, nil)
		}
	}
}

// assertConnection verifies a connection was successfully established.
func assertConnection(t *utesting.T, conn net.Conn, err error, msgAndArgs ...interface{}) {
	if err != nil {
		msg := "connection failed"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		t.Fatalf("%s: %v", msg, err)
	}
	if conn == nil {
		msg := "connection is nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		t.Fatal(msg)
	}
}

// assertError verifies an error occurred.
func assertError(t *utesting.T, err error, expectedSubstring string, msgAndArgs ...interface{}) {
	if err == nil {
		msg := "expected error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		t.Fatalf("%s, got nil", msg)
	}
	if expectedSubstring != "" && !strings.Contains(err.Error(), expectedSubstring) {
		msg := fmt.Sprintf("expected error containing %q", expectedSubstring)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		t.Fatalf("%s, got: %v", msg, err)
	}
}

// waitForCondition waits for a condition to become true.
func waitForCondition(t *utesting.T, timeout time.Duration, condition func() bool, msg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}
