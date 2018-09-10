// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"net"

	ma "github.com/multiformats/go-multiaddr"
)

// Config defines the configurations to run commands.
type Config struct {
	// p2p
	ListenAddr      net.IP         // p2p message listen address
	ListenPort      uint           // p2p message listen port
	AddPeers        []ma.Multiaddr // peers to be connected when the p2p service gets on
	DefaultPeerPort uint           // default p2p listen port of peers
	// rpc client
	// logger
	LogLevel int
}

// // PeerAddr represents the listening address of peer, including ip/port
// type PeerAddr struct {
// 	IP      net.IP
// 	Port    uint
// 	address string
// }

// func (pa *PeerAddr) String() string {
// 	return pa.address
// }

// // IsIPv4 checks if the address is IPV4 address
// func IsIPv4(address string) bool {
// 	return strings.Count(address, ":") < 2
// }

// // IsIPv6 checks if the address is IPV6 address
// func IsIPv6(address string) bool {
// 	return strings.Count(address, ":") >= 2
// }

// // NewPeerAddr creates a PeerAddr object or nil if the parameter is invalid
// func NewPeerAddr(addr string, defaultPort uint) *PeerAddr {
// 	if IsIPv4(addr) {
// 		var s = strings.Split(addr, ":")
// 		if len(s) == 1 { // without port info
// 			return &PeerAddr{
// 				IP:      net.ParseIP(s[0]),
// 				Port:    defaultPort,
// 				address: addr,
// 			}
// 		}
// 		// with port info
// 		port, err := strconv.Atoi(s[1])
// 		if err == nil {
// 			return &PeerAddr{
// 				IP:      net.ParseIP(s[0]),
// 				Port:    uint(port),
// 				address: addr,
// 			}
// 		}
// 	} else if IsIPv6(addr) {
// 		var s = strings.Split(addr, "]:") // "[::FFFF:C0A8:1%1]:80"
// 		if len(s) == 1 {
// 			return &PeerAddr{
// 				IP:      net.ParseIP(s[0]),
// 				Port:    defaultPort,
// 				address: addr,
// 			}
// 		} else if len(s) == 2 {
// 			if s[0][0] == byte('[') {
// 				// with port info
// 				port, err := strconv.Atoi(s[1])
// 				if err == nil {
// 					return &PeerAddr{
// 						IP:      net.ParseIP(s[0][1:]),
// 						Port:    uint(port),
// 						address: addr,
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return nil
// }
