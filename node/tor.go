package node

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"slices"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	torOKResponse = 250

	torKeyFilename      = "hs_ed25519_secret_key"
	torHostnameFilename = "hostname"

	torCommandTimeout = 10 * time.Second
)

// enableTorHiddenService connects to the Tor control port and provisions a
// hidden service backed by the configured RPC endpoints.
func (n *Node) enableTorHiddenService() error {
	cfg := n.config.Tor
	if !cfg.Enabled {
		return nil
	}

	// Skip RPC hidden service if no RPC endpoints are configured
	// This allows P2P-only nodes to use Tor without requiring RPC endpoints
	if n.config.HTTPHost == "" && n.config.WSHost == "" {
		return nil
	}

	controlAddr := cfg.ControlAddress
	if controlAddr == "" {
		controlAddr = DefaultTorControlAddress
	}

	controller, err := dialTorController(controlAddr)
	if err != nil {
		return fmt.Errorf("tor control connect failed: %w", err)
	}
	defer controller.Close()

	if err := controller.protocolInfo(); err != nil {
		return err
	}

	cookiePath := cfg.CookiePath
	if cookiePath == "" {
		cookiePath = DefaultTorCookiePath
	}
	cookiePath = n.resolveTorPath(cookiePath)
	cookie, err := os.ReadFile(cookiePath)
	if err != nil {
		return fmt.Errorf("tor cookie read failed: %w", err)
	}

	if err := controller.authenticate(cookie); err != nil {
		return err
	}

	mappings, err := n.torPortMappings(cfg)
	if err != nil {
		return err
	}
	if len(mappings) == 0 {
		return errors.New("tor hidden service requires at least one RPC endpoint")
	}

	hsDir := cfg.HiddenServiceDir
	if hsDir == "" {
		hsDir = DefaultTorServiceDir
	}
	hsDir = n.resolveTorPath(hsDir)
	if err := os.MkdirAll(hsDir, 0o700); err != nil {
		return fmt.Errorf("tor hidden service dir create failed: %w", err)
	}

	keyPath := filepath.Join(hsDir, torKeyFilename)
	hostnamePath := filepath.Join(hsDir, torHostnameFilename)

	keySpec, err := loadTorKey(keyPath)
	if err != nil {
		return err
	}

	serviceID, newKey, err := controller.addOnion(keySpec, mappings)
	if err != nil {
		return err
	}

	if newKey != "" {
		if err := os.WriteFile(keyPath, []byte(newKey+"\n"), 0o600); err != nil {
			return fmt.Errorf("write tor key failed: %w", err)
		}
	}
	onionAddress := serviceID + ".onion"
	if err := os.WriteFile(hostnamePath, []byte(onionAddress+"\n"), 0o600); err != nil {
		return fmt.Errorf("write tor hostname failed: %w", err)
	}

	os.Setenv("GETH_TOR_ONION", onionAddress)
	fmt.Println("Tor hidden RPC endpoint:", onionAddress)
	n.log.Info("Tor hidden service ready", "onion", onionAddress, "ports", mappings)
	return nil
}

func (n *Node) resolveTorPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	instanceDir := n.config.instanceDir()
	if instanceDir == "" {
		return path
	}
	return filepath.Join(instanceDir, path)
}

func (n *Node) torPortMappings(cfg TorConfig) ([]string, error) {
	result := make(map[string]struct{})
	addMapping := func(virtual int, endpoint string) error {
		host, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return fmt.Errorf("invalid rpc endpoint %s: %w", endpoint, err)
		}
		switch host {
		case "", "0.0.0.0", "::", "[::]":
			host = "127.0.0.1"
		}
		if virtual == 0 {
			if v, err := strconv.Atoi(port); err == nil {
				virtual = v
			}
		}
		mapping := fmt.Sprintf("Port=%d,%s:%s", virtual, host, port)
		result[mapping] = struct{}{}
		return nil
	}

	if n.config.HTTPHost != "" {
		endpoint := n.http.listenAddr()
		if endpoint == "" {
			return nil, errors.New("http endpoint not listening")
		}
		if err := addMapping(cfg.HTTPPort, endpoint); err != nil {
			return nil, err
		}
	}
	if n.config.WSHost != "" {
		endpoint := n.ws.listenAddr()
		if endpoint == "" {
			return nil, errors.New("ws endpoint not listening")
		}
		if err := addMapping(cfg.WSPort, endpoint); err != nil {
			return nil, err
		}
	}

	mappings := make([]string, 0, len(result))
	for mapping := range result {
		mappings = append(mappings, mapping)
	}
	slices.Sort(mappings)
	return mappings, nil
}

func loadTorKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "NEW:ED25519-V3", nil
		}
		return "", fmt.Errorf("read tor key failed: %w", err)
	}
	key := strings.TrimSpace(string(data))
	if key == "" {
		return "NEW:ED25519-V3", nil
	}
	return key, nil
}

type torController struct {
	conn   net.Conn
	reader *bufio.Reader
}

func dialTorController(addr string) (*torController, error) {
	conn, err := net.DialTimeout("tcp", addr, torCommandTimeout)
	if err != nil {
		return nil, err
	}
	return &torController{conn: conn, reader: bufio.NewReader(conn)}, nil
}

func (c *torController) Close() error {
	return c.conn.Close()
}

func (c *torController) protocolInfo() error {
	_, err := c.command("PROTOCOLINFO 1")
	return err
}

func (c *torController) authenticate(cookie []byte) error {
	cmd := fmt.Sprintf("AUTHENTICATE %s", strings.ToUpper(hex.EncodeToString(cookie)))
	_, err := c.command(cmd)
	return err
}

func (c *torController) addOnion(keySpec string, mappings []string) (serviceID string, privateKey string, err error) {
	parts := []string{"ADD_ONION", keySpec, "Flags=Detach"}
	parts = append(parts, mappings...)
	lines, err := c.command(strings.Join(parts, " "))
	if err != nil {
		return "", "", err
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "ServiceID=") {
			serviceID = strings.TrimPrefix(line, "ServiceID=")
		} else if strings.HasPrefix(line, "PrivateKey=") {
			privateKey = strings.TrimPrefix(line, "PrivateKey=")
		}
	}
	if serviceID == "" {
		return "", "", errors.New("tor did not return ServiceID")
	}
	return serviceID, privateKey, nil
}

func (c *torController) command(cmd string) ([]string, error) {
	if _, err := fmt.Fprintf(c.conn, "%s\r\n", cmd); err != nil {
		return nil, err
	}
	return c.readReply()
}

func (c *torController) readReply() ([]string, error) {
	var lines []string
	deadline := time.Now().Add(torCommandTimeout)
	_ = c.conn.SetDeadline(deadline)
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 4 {
			return nil, fmt.Errorf("invalid tor response: %q", line)
		}
		codePart := line[:3]
		text := line[4:]
		code, err := strconv.Atoi(codePart)
		if err != nil {
			return nil, fmt.Errorf("invalid tor status code: %q", codePart)
		}
		if code != torOKResponse {
			return nil, fmt.Errorf("tor control error %d: %s", code, text)
		}
		lines = append(lines, text)
		if line[3] == ' ' {
			break
		}
	}
	return lines, nil
}

// enableP2PTorHiddenService creates a Tor hidden service for the P2P port
// and updates the local node's ENR with the .onion address.
//
// This method:
//   - Connects to the Tor control port
//   - Creates an ephemeral hidden service for the P2P port
//   - Retrieves the .onion address from the Tor controller
//   - Updates the local node's ENR with the .onion address
//
// The method gracefully handles cases where Tor is disabled or unavailable.
// If localNode is nil, an error is returned.
// If Tor is not enabled in the config, the method returns nil without error.
func (n *Node) enableP2PTorHiddenService(localNode *enode.LocalNode, p2pPort int) error {
	if localNode == nil {
		return errors.New("local node is nil")
	}

	cfg := n.config.Tor
	if !cfg.Enabled {
		return nil // Tor not enabled, skip without error
	}

	// Validate port
	if p2pPort <= 0 || p2pPort > 65535 {
		return fmt.Errorf("invalid P2P port: %d", p2pPort)
	}

	// Check if a persistent hidden service already exists
	// This happens when Tor is running in a container with pre-configured hidden service
	hsDir := cfg.HiddenServiceDir
	if hsDir == "" {
		hsDir = DefaultTorServiceDir
	}
	hsDir = n.resolveTorPath(hsDir)
	hostnamePath := filepath.Join(hsDir, torHostnameFilename)

	var onionAddress string
	// Try to read existing .onion address from persistent hidden service
	if data, err := os.ReadFile(hostnamePath); err == nil {
		onionAddress = strings.TrimSpace(string(data))
		n.log.Info("Using existing P2P Tor hidden service", "onion", onionAddress, "port", p2pPort, "source", "persistent")
	} else if !os.IsNotExist(err) {
		// File exists but couldn't read it - this is an error
		return fmt.Errorf("failed to read Tor hostname file: %w", err)
	} else {
		// No persistent hidden service found, create ephemeral one via control port
		n.log.Info("No persistent hidden service found, creating ephemeral service")

		// Use default control address if not configured
		controlAddr := cfg.ControlAddress
		if controlAddr == "" {
			controlAddr = DefaultTorControlAddress
		}

		// Connect to Tor control port
		controller, err := dialTorController(controlAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to Tor controller: %w", err)
		}
		defer controller.Close()

		// Authenticate with Tor
		if err := controller.protocolInfo(); err != nil {
			return fmt.Errorf("tor protocol info failed: %w", err)
		}

		cookiePath := cfg.CookiePath
		if cookiePath == "" {
			cookiePath = DefaultTorCookiePath
		}
		cookiePath = n.resolveTorPath(cookiePath)
		cookie, err := os.ReadFile(cookiePath)
		if err != nil {
			return fmt.Errorf("failed to read Tor cookie: %w", err)
		}

		if err := controller.authenticate(cookie); err != nil {
			return fmt.Errorf("tor authentication failed: %w", err)
		}

		// Create ephemeral hidden service for P2P port
		// Format: Port=<virtual_port>,<target_address>:<target_port>
		mapping := fmt.Sprintf("Port=%d,127.0.0.1:%d", p2pPort, p2pPort)
		serviceID, _, err := controller.addOnion("NEW:ED25519-V3", []string{mapping})
		if err != nil {
			return fmt.Errorf("failed to create P2P hidden service: %w", err)
		}

		// Construct .onion address (56 base32 chars + ".onion")
		onionAddress = serviceID + ".onion"
		n.log.Info("P2P Tor hidden service ready", "onion", onionAddress, "port", p2pPort, "source", "ephemeral")
	}

	// Update local ENR with .onion address
	// Create the onion entry - this validates the address during RLP encoding
	onion := enr.Onion3(onionAddress)
	// Eagerly validate by attempting to encode
	if _, err := rlp.EncodeToBytes(onion); err != nil {
		return fmt.Errorf("invalid onion address: %w", err)
	}
	// Set in ENR (validation passed)
	localNode.Set(onion)

	n.log.Info("P2P ENR updated with Tor address", "onion", onionAddress)
	return nil
}

