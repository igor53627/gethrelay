package torhidden

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
)

type torTestAPI struct{}

func (torTestAPI) Ping() string { return "pong" }

func TestHiddenServiceIntegration(t *testing.T) {
	if os.Getenv("TOR_INTEGRATION_TEST") == "" {
		t.Skip("set TOR_INTEGRATION_TEST=1 to enable tor integration tests")
	}
	image := os.Getenv("GETH_TOR_IMAGE")
	if image == "" {
		t.Fatal("GETH_TOR_IMAGE not set")
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tempDir := t.TempDir()
	conf := node.DefaultConfig
	conf.Name = "tor-integration"
	conf.DataDir = tempDir
	conf.P2P.NoDiscovery = true
	conf.P2P.ListenAddr = ":0"
	conf.P2P.MaxPeers = 0
	conf.HTTPHost = "127.0.0.1"
	conf.WSHost = "127.0.0.1"

	httpPort := mustReservePort(t)
	wsPort := mustReservePort(t)
	if httpPort == wsPort {
		wsPort = mustReservePort(t)
	}
	conf.HTTPPort = httpPort
	conf.WSPort = wsPort

	instanceDir := filepath.Join(conf.DataDir, conf.Name)
	torSharedDir := filepath.Join(instanceDir, "tor")
	if err := os.MkdirAll(torSharedDir, 0o777); err != nil {
		t.Fatalf("create tor share dir: %v", err)
	}

	socksPort := mustReservePort(t)
	controlPort := mustReservePort(t)

	containerName := fmt.Sprintf("geth-tor-test-%d", time.Now().UnixNano())
	runArgs := []string{
		"run", "-d", "--rm",
		"--name", containerName,
		"--user", "root",
		"-v", fmt.Sprintf("%s:/data", torSharedDir),
		"-p", fmt.Sprintf("127.0.0.1:%d:9150", socksPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:9051", controlPort),
		image,
	}
	cmd := exec.CommandContext(ctx, "docker", runArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("start tor container: %v", err)
	}
	defer exec.Command("docker", "rm", "-f", containerName).Run()

	if err := waitForPort(ctx, fmt.Sprintf("127.0.0.1:%d", controlPort)); err != nil {
		t.Fatalf("tor control port not ready: %v", err)
	}

	// Wait for Tor to create the cookie file
	cookiePath := filepath.Join(torSharedDir, "control_auth_cookie")
	if err := waitForFile(ctx, cookiePath); err != nil {
		t.Fatalf("tor cookie file not created: %v", err)
	}

	// Fix permissions on Tor files so the test can read them
	// Tor runs as root in the container and creates root-owned files
	// Use Docker to fix permissions as root since host user may not have permission
	fixPermsCmd := exec.CommandContext(ctx, "docker", "exec", containerName, "chmod", "-R", "755", "/data")
	if err := fixPermsCmd.Run(); err != nil {
		t.Fatalf("fix tor directory permissions: %v", err)
	}

	conf.Tor.Enabled = true
	conf.Tor.ControlAddress = fmt.Sprintf("127.0.0.1:%d", controlPort)
	conf.Tor.CookiePath = filepath.Join("tor", "control_auth_cookie")
	conf.Tor.HiddenServiceDir = filepath.Join("tor", "hidden_service")
	conf.Tor.HTTPPort = httpPort
	conf.Tor.WSPort = wsPort

	stack, err := node.New(&conf)
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	defer stack.Close()

	stack.RegisterAPIs([]rpc.API{{
		Namespace: "torTest",
		Service:   torTestAPI{},
		Public:    true,
	}})

	if err := stack.Start(); err != nil {
		t.Fatalf("start node: %v", err)
	}

	onion := os.Getenv("GETH_TOR_ONION")
	if onion == "" {
		t.Fatal("tor hidden service onion not set")
	}

	// Give Tor time to bootstrap and publish the hidden service descriptor
	// In CI environments, this can take significantly longer than locally
	t.Logf("Waiting for Tor bootstrap and hidden service descriptor publication for %s", onion)
	time.Sleep(30 * time.Second)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "torTest_ping",
		"params":  []interface{}{},
		"id":      1,
	}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("http://%s:%d", onion, httpPort)

	// Retry up to 20 times with exponentially increasing delays
	// Hidden services can take time to be reachable through the Tor network
	if err := retry(ctx, 20, func() error {
		return curlViaTor(ctx, containerName, socksPort, url, string(body))
	}); err != nil {
		t.Fatalf("hidden service unreachable: %v", err)
	}
}

func mustReservePort(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func waitForPort(ctx context.Context, address string) error {
	for {
		conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func retry(ctx context.Context, attempts int, fn func() error) error {
	var last error
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			last = err
			select {
			case <-time.After(time.Duration(i+1) * 300 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
	return last
}

func curlViaTor(ctx context.Context, container string, socksPort int, url, body string) error {
	// Note: socksPort parameter is the host-mapped port, but we're running curl
	// inside the container via docker exec, so we use the container's internal port (9150)
	args := []string{
		"exec", container,
		"curl", "--socks5-hostname", "localhost:9150",
		"-sS", "-X", "POST",
		"-H", "Content-Type: application/json",
		"--data", body,
		url,
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("curl error: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "\"result\":\"pong\"") {
		return fmt.Errorf("unexpected response: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
