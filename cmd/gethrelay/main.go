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

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/forkid"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/eth/relay"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/nat"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/urfave/cli/v2"
)

const (
	clientIdentifier = "gethrelay"
)

var (
	relayFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  "identity",
			Usage: "Custom node name",
		},
		&cli.StringFlag{
			Name:  "bootnodes",
			Usage: "Comma separated list of bootstrap nodes",
		},
		&cli.StringFlag{
			Name:  "staticnodes",
			Usage: "Comma separated list of static peer nodes (always maintained)",
		},
		&cli.IntFlag{
			Name:  "port",
			Usage: "Network listening port",
			Value: 30303,
		},
		&cli.IntFlag{
			Name:  "maxpeers",
			Usage: "Maximum number of network peers (default: 200, no hard maximum)",
			Value: 200,
		},
		&cli.Uint64Flag{
			Name:  "networkid",
			Usage: "Network identifier (1=Mainnet, 17000=Holesky, 11155111=Sepolia)",
		},
		&cli.BoolFlag{
			Name:  "v4disc",
			Usage: "Enable discv4 discovery",
		},
		&cli.BoolFlag{
			Name:  "v5disc",
			Usage: "Enable discv5 discovery",
		},
		&cli.BoolFlag{
			Name:  "nodiscover",
			Usage: "Disable peer discovery",
		},
		&cli.StringFlag{
			Name:  "nat",
			Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		},
		&cli.StringFlag{
			Name:  "netrestrict",
			Usage: "Restrict network communication to the given IP networks (CIDR masks)",
		},
		&cli.StringFlag{
			Name:  "genesis",
			Usage: "Genesis block hash (required)",
		},
		&cli.StringFlag{
			Name:  "chain",
			Usage: "Chain preset (mainnet, holesky, sepolia)",
			Value: "mainnet",
		},
		&cli.Uint64Flag{
			Name:  "earliest-block",
			Usage: "Earliest available block number",
			Value: 0,
		},
		&cli.Uint64Flag{
			Name:  "latest-block",
			Usage: "Latest available block number",
			Value: 0,
		},
		&cli.StringFlag{
			Name:  "latest-hash",
			Usage: "Latest block hash (hex)",
		},
		&cli.BoolFlag{
			Name:  "log.debug",
			Usage: "Prepends log messages with call-site location (deprecated, use --log.backtrace)",
		},
		&cli.StringFlag{
			Name:  "log.backtrace",
			Usage: "Request a stack trace at a specific logging statement (e.g. \"block.go:271\")",
		},
		&cli.StringFlag{
			Name:  "rpc.upstream",
			Usage: "Upstream RPC endpoint URL for proxying requests (default: https://ethereum-rpc.publicnode.com)",
			Value: "https://ethereum-rpc.publicnode.com",
		},
		// Tor configuration flags
		&cli.StringFlag{
			Name:  "tor-proxy",
			Usage: "SOCKS5 proxy address for Tor connections (e.g., 127.0.0.1:9050)",
		},
		&cli.BoolFlag{
			Name:  "prefer-tor",
			Usage: "Prefer .onion addresses when both Tor and clearnet available",
		},
		&cli.BoolFlag{
			Name:  "only-onion",
			Usage: "Restrict to .onion addresses only (requires --tor-proxy)",
		},
		// HTTP RPC configuration flags
		&cli.BoolFlag{
			Name:  "http",
			Usage: "Enable the HTTP-RPC server",
		},
		&cli.StringFlag{
			Name:  "http.addr",
			Usage: "HTTP-RPC server listening interface",
			Value: node.DefaultHTTPHost,
		},
		&cli.IntFlag{
			Name:  "http.port",
			Usage: "HTTP-RPC server listening port",
			Value: node.DefaultHTTPPort,
		},
		&cli.StringFlag{
			Name:  "http.api",
			Usage: "API's offered over the HTTP-RPC interface (comma separated)",
			Value: "eth,net,web3",
		},
		// Admin API configuration flags (separate endpoint for p2p management)
		&cli.BoolFlag{
			Name:  "admin",
			Usage: "Enable the admin API server for p2p management",
		},
		&cli.StringFlag{
			Name:  "admin.addr",
			Usage: "Admin API server listening interface",
			Value: "127.0.0.1",
		},
		&cli.IntFlag{
			Name:  "admin.port",
			Usage: "Admin API server listening port",
			Value: 8546,
		},
	}

	app = flags.NewApp("lightweight Ethereum P2P relay node")
)

func init() {
	app.Action = runRelay
	app.Flags = append(relayFlags, debug.Flags...)
	flags.AutoEnvVars(app.Flags, "GETHRELAY")

	app.Before = func(ctx *cli.Context) error {
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runRelay(ctx *cli.Context) error {
	if args := ctx.Args().Slice(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	// Validate Tor configuration
	if ctx.Bool("only-onion") && !ctx.IsSet("tor-proxy") {
		return fmt.Errorf("--only-onion requires --tor-proxy to be set")
	}

	// Load chain configuration
	chainPreset := ctx.String("chain")
	var chainConfig *params.ChainConfig
	var genesisHash common.Hash
	var networkID uint64

	switch chainPreset {
	case "mainnet":
		chainConfig = params.MainnetChainConfig
		genesisHash = params.MainnetGenesisHash
		networkID = 1
	case "holesky":
		chainConfig = params.HoleskyChainConfig
		genesisHash = params.HoleskyGenesisHash
		networkID = 17000
	case "sepolia":
		chainConfig = params.SepoliaChainConfig
		genesisHash = params.SepoliaGenesisHash
		networkID = 11155111
	default:
		return fmt.Errorf("unknown chain preset: %s", chainPreset)
	}

	// Override genesis hash if provided
	if ctx.IsSet("genesis") {
		genesisHash = common.HexToHash(ctx.String("genesis"))
	}

	// Override network ID if provided
	if ctx.IsSet("networkid") {
		networkID = ctx.Uint64("networkid")
	}

	// Calculate fork ID
	// For relay, we use current block = latest block or 0
	latestBlock := ctx.Uint64("latest-block")
	if latestBlock == 0 {
		latestBlock = 0 // Start from genesis
	}
	
	// Get current timestamp for fork ID calculation
	currentTime := uint64(time.Now().Unix())
	
	// Create a minimal genesis block structure for fork ID calculation
	// We only need the hash and timestamp, which we already have
	// For forkid.NewID, we need a types.Block, so we'll use a simplified approach
	// Create genesis from preset
	var genesis *core.Genesis
	switch chainPreset {
	case "mainnet":
		genesis = core.DefaultGenesisBlock()
	case "holesky":
		genesis = core.DefaultHoleskyGenesisBlock()
	case "sepolia":
		genesis = core.DefaultSepoliaGenesisBlock()
	default:
		genesis = core.DefaultGenesisBlock()
	}
	
	// Commit genesis to get block for fork ID calculation
	// Use in-memory database to avoid any disk writes
	db := rawdb.NewMemoryDatabase()
	genesisBlock, _ := genesis.Commit(db, triedb.NewDatabase(db, nil))
	
	// Calculate fork ID (using latest block or 0)
	forkID := forkid.NewID(chainConfig, genesisBlock, latestBlock, currentTime)

	// Get block range
	latestHash := common.Hash{}
	if ctx.IsSet("latest-hash") {
		latestHash = common.HexToHash(ctx.String("latest-hash"))
	} else if latestBlock == 0 {
		// If no latest block specified, use genesis block hash for ETH69 compatibility
		// ETH69 requires a non-empty LatestBlockHash in the status packet
		latestHash = genesisHash
	}

	blockRange := relay.BlockRange{
		EarliestBlock:   ctx.Uint64("earliest-block"),
		LatestBlock:     latestBlock,
		LatestBlockHash: latestHash,
	}

	// Create relay configuration
	relayConfig := &relay.Config{
		NetworkID:   networkID,
		GenesisHash: genesisHash,
		ChainConfig: chainConfig,
		ForkID:      forkID,
		BlockRange:  blockRange,
	}

	// Create minimal node (no database)
	nodeConfig := &node.Config{
		P2P: p2p.Config{
			MaxPeers:      ctx.Int("maxpeers"),
			ListenAddr:    fmt.Sprintf(":%d", ctx.Int("port")),
			DiscoveryV4:   ctx.Bool("v4disc"),
			DiscoveryV5:   ctx.Bool("v5disc"),
			NoDiscovery:   ctx.Bool("nodiscover"),
			// Tor configuration
			TorSOCKSProxy: ctx.String("tor-proxy"),
			PreferTor:     ctx.Bool("prefer-tor"),
			OnlyOnion:     ctx.Bool("only-onion"),
		},
		UserIdent: ctx.String("identity"),
	}

	// Enable admin API server if requested
	if ctx.Bool("admin") {
		nodeConfig.HTTPHost = ctx.String("admin.addr")
		nodeConfig.HTTPPort = ctx.Int("admin.port")
		nodeConfig.HTTPModules = []string{"admin"}
		log.Info("Admin API server enabled", "addr", nodeConfig.HTTPHost, "port", nodeConfig.HTTPPort)
	}

	// Set NAT
	if ctx.IsSet("nat") {
		natif, err := nat.Parse(ctx.String("nat"))
		if err != nil {
			return fmt.Errorf("invalid NAT option: %v", err)
		}
		nodeConfig.P2P.NAT = natif
	}
	
	// Set bootstrap nodes
	if ctx.IsSet("bootnodes") {
		urls := splitAndTrim(ctx.String("bootnodes"))
		nodeConfig.P2P.BootstrapNodes = mustParseBootnodes(urls)
	} else {
		// Use default bootnodes based on chain
		var urls []string
		switch chainPreset {
		case "holesky":
			urls = params.HoleskyBootnodes
		case "sepolia":
			urls = params.SepoliaBootnodes
		default:
			urls = params.MainnetBootnodes
		}
		nodeConfig.P2P.BootstrapNodes = mustParseBootnodes(urls)
	}

	// Set static nodes (for persistent .onion connections)
	if ctx.IsSet("staticnodes") {
		urls := splitAndTrim(ctx.String("staticnodes"))
		nodeConfig.P2P.StaticNodes = mustParseBootnodes(urls)
		log.Info("Configured static peer nodes", "count", len(nodeConfig.P2P.StaticNodes))
	}

	stack, err := node.New(nodeConfig)
	if err != nil {
		return fmt.Errorf("failed to create node: %v", err)
	}
	defer stack.Close()

	// Setup RPC proxy with configured HTTP settings
	httpAddr := ctx.String("http.addr")
	httpPort := ctx.Int("http.port")
	upstreamURL := ctx.String("rpc.upstream")

	if err := setupRPCProxy(stack, upstreamURL, httpAddr, httpPort); err != nil {
		return fmt.Errorf("failed to setup RPC proxy: %v", err)
	}

	// Create relay service
	relayService, err := relay.NewRelay(stack, relayConfig, networkID, nil)
	if err != nil {
		return fmt.Errorf("failed to create relay: %v", err)
	}

	// Register relay service
	stack.RegisterLifecycle(relayService)

	// Register relay protocols BEFORE starting the stack
	// Protocols must be registered before the node starts
	if err := relayService.RegisterProtocols(stack); err != nil {
		return fmt.Errorf("failed to register protocols: %v", err)
	}

	log.Info("Starting Ethereum P2P relay", 
		"network", networkID,
		"genesis", genesisHash.Hex(),
		"chain", chainPreset)

	// Start the node
	if err := stack.Start(); err != nil {
		return fmt.Errorf("failed to start node: %v", err)
	}

	log.Info("Ethereum P2P relay started")
	stack.Wait()
	return nil
}

// Helper functions to avoid importing cmd/utils

func splitAndTrim(input string) []string {
	var ret []string
	for _, r := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(r); trimmed != "" {
			ret = append(ret, trimmed)
		}
	}
	return ret
}

func mustParseBootnodes(urls []string) []*enode.Node {
	nodes := make([]*enode.Node, 0, len(urls))
	for _, url := range urls {
		if url != "" {
			node, err := enode.Parse(enode.ValidSchemes, url)
			if err != nil {
				log.Error("Bootstrap URL invalid", "enode", url, "err", err)
				continue
			}
			nodes = append(nodes, node)
		}
	}
	return nodes
}

