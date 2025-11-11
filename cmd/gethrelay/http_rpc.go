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
	"github.com/ethereum/go-ethereum/node"
)

// setupHTTPRPC configures the HTTP RPC server with the admin API
// This enables access to admin_nodeInfo for ENR extraction
func setupHTTPRPC(stack *node.Node, config *node.Config) error {
	// The node.New() function already:
	// 1. Creates the HTTP server (node.http)
	// 2. Registers built-in APIs including admin API
	// 3. Configures the server based on config.HTTPHost, HTTPPort, HTTPModules
	//
	// When stack.Start() is called, it will:
	// 1. Start the HTTP server if HTTPHost is set
	// 2. Enable the APIs specified in HTTPModules
	//
	// So we don't need to do anything extra here - the node package
	// handles everything automatically based on the Config.
	return nil
}
