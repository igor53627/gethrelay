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

	if n.config.HTTPHost == "" && n.config.WSHost == "" {
		return errors.New("tor hidden service requires HTTP or WS endpoint")
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
