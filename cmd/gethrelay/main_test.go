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
	"flag"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// TestStaticNodesFlag verifies that the --staticnodes CLI flag is defined
func TestStaticNodesFlag(t *testing.T) {
	// Verify flag exists in relayFlags
	var foundFlag bool
	for _, flag := range relayFlags {
		if stringFlag, ok := flag.(*cli.StringFlag); ok {
			if stringFlag.Name == "staticnodes" {
				foundFlag = true
				if stringFlag.Usage == "" {
					t.Error("staticnodes flag should have a usage description")
				}
				break
			}
		}
	}

	if !foundFlag {
		t.Error("staticnodes flag not found in relayFlags")
	}
}

// TestMustParseBootnodes verifies that enode URL parsing works
func TestMustParseBootnodes(t *testing.T) {
	testCases := []struct {
		name        string
		urls        []string
		expectCount int
	}{
		{
			name: "valid clearnet enode",
			urls: []string{
				"enode://d860a01f9722d78051619d1e2351aba3f43f943f6f00718d1b9baa4101932a1f5011f16bb2b1bb35db20d6fe28fa0bf09636d26a87d31de9ec6203eeedb1f666@127.0.0.1:30303",
			},
			expectCount: 1,
		},
		{
			name: "valid onion enode",
			urls: []string{
				"enode://d860a01f9722d78051619d1e2351aba3f43f943f6f00718d1b9baa4101932a1f5011f16bb2b1bb35db20d6fe28fa0bf09636d26a87d31de9ec6203eeedb1f666@55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion:30303",
			},
			expectCount: 1,
		},
		{
			name: "multiple valid enodes",
			urls: []string{
				"enode://d860a01f9722d78051619d1e2351aba3f43f943f6f00718d1b9baa4101932a1f5011f16bb2b1bb35db20d6fe28fa0bf09636d26a87d31de9ec6203eeedb1f666@127.0.0.1:30303",
				"enode://22a8232c3abc76a16ae9d6c3b164f98775fe226f0917b0ca871128a74a8e9630b458460865bab457221f1d448dd9791d24c4e5d88786180ac185df813a68d4de@55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion:30303",
			},
			expectCount: 2,
		},
		{
			name:        "empty list",
			urls:        []string{},
			expectCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodes := mustParseBootnodes(tc.urls)
			if len(nodes) != tc.expectCount {
				t.Errorf("expected %d nodes, got %d", tc.expectCount, len(nodes))
			}
		})
	}
}

// TestSplitAndTrim verifies CSV parsing works correctly
func TestSplitAndTrim(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single value",
			input:    "value1",
			expected: []string{"value1"},
		},
		{
			name:     "multiple values",
			input:    "value1,value2,value3",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "values with spaces",
			input:    " value1 , value2 , value3 ",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "trailing comma",
			input:    "value1,value2,",
			expected: []string{"value1", "value2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := splitAndTrim(tc.input)
			if len(result) != len(tc.expected) {
				t.Errorf("expected %d values, got %d", len(tc.expected), len(result))
				return
			}
			for i := range result {
				if result[i] != tc.expected[i] {
					t.Errorf("expected result[%d]=%q, got %q", i, tc.expected[i], result[i])
				}
			}
		})
	}
}

// TestStaticNodesIntegration verifies that staticnodes CLI flag sets P2P config correctly
func TestStaticNodesIntegration(t *testing.T) {
	// Create a test CLI context with staticnodes flag
	set := flag.NewFlagSet("test", 0)
	set.String("staticnodes", "", "doc")
	set.String("chain", "mainnet", "doc")
	set.String("rpc.upstream", "https://ethereum-rpc.publicnode.com", "doc")

	staticNodesValue := "enode://d860a01f9722d78051619d1e2351aba3f43f943f6f00718d1b9baa4101932a1f5011f16bb2b1bb35db20d6fe28fa0bf09636d26a87d31de9ec6203eeedb1f666@55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion:30303"

	set.Parse([]string{
		"--staticnodes", staticNodesValue,
		"--chain", "mainnet",
	})

	ctx := cli.NewContext(app, set, nil)

	// Test parsing logic directly
	if ctx.IsSet("staticnodes") {
		urls := splitAndTrim(ctx.String("staticnodes"))
		nodes := mustParseBootnodes(urls)

		if len(nodes) != 1 {
			t.Errorf("expected 1 static node, got %d", len(nodes))
		}

		if len(nodes) > 0 {
			// Verify the node was parsed successfully
			nodeStr := nodes[0].URLv4()
			if nodeStr == "" {
				t.Error("node URL should not be empty")
			}
			// Verify it contains the onion domain
			if !strings.Contains(nodeStr, "55tgupvg5jo4zvatazrr75zflsx6jx36qz3wzu6mb4rrmdbsvxvw54yd.onion") {
				t.Errorf("expected onion address in node URL, got %s", nodeStr)
			}
		}
	} else {
		t.Error("staticnodes flag should be set")
	}
}
