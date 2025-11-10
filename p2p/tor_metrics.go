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

package p2p

import (
	"strings"

	"github.com/ethereum/go-ethereum/metrics"
)

var (
	// Tor dial metrics
	torDialAttempts = metrics.NewRegisteredCounter("p2p/dials/tor/total", nil)
	torDialSuccesses = metrics.NewRegisteredCounter("p2p/dials/tor/success", nil)

	// Peer count by network type
	peersByNetworkTor = metrics.NewRegisteredGauge("p2p/peers/network/tor", nil)
	peersByNetworkClearnet = metrics.NewRegisteredGauge("p2p/peers/network/clearnet", nil)

	// Traffic by network type and direction
	trafficTorIngress = metrics.NewRegisteredCounter("p2p/traffic/tor/ingress", nil)
	trafficTorEgress = metrics.NewRegisteredCounter("p2p/traffic/tor/egress", nil)
	trafficClearnetIngress = metrics.NewRegisteredCounter("p2p/traffic/clearnet/ingress", nil)
	trafficClearnetEgress = metrics.NewRegisteredCounter("p2p/traffic/clearnet/egress", nil)
)

// IsOnionAddress returns true if the address is a .onion address
func IsOnionAddress(addr string) bool {
	return strings.Contains(addr, ".onion")
}
