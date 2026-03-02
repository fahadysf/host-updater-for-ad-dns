//go:build linux

package main

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

func getDefaultInterfaceAddresses() (IPAddrs, error) {
	return getDefaultInterfaceAddressesLinux()
}

func getDefaultInterfaceAddressesLinux() (IPAddrs, error) {
	addrs := IPAddrs{}

	// Get default IPv4 route
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		debugLog.Printf("Error getting IPv4 routes: %v, falling back\n", err)
		return getDefaultInterfaceAddressesFallback()
	}

	var ipv4Iface *net.Interface
	for _, route := range routes {
		// Default route has Dst=0.0.0.0/0 or Dst=nil
		if route.Dst == nil || (route.Dst != nil && route.Dst.String() == "0.0.0.0/0") {
			iface, err := net.InterfaceByIndex(route.LinkIndex)
			if err == nil {
				ipv4Iface = iface
				debugLog.Printf("Found default IPv4 route on interface: %s\n", iface.Name)
				break
			}
		}
	}

	// Get default IPv6 route
	routesV6, err := netlink.RouteList(nil, netlink.FAMILY_V6)
	if err != nil {
		debugLog.Printf("Error getting IPv6 routes: %v\n", err)
	}

	var ipv6Iface *net.Interface
	for _, route := range routesV6 {
		// Default route has Dst=::/0 or Dst=nil
		if route.Dst == nil || (route.Dst != nil && route.Dst.String() == "::/0") {
			iface, err := net.InterfaceByIndex(route.LinkIndex)
			if err == nil {
				ipv6Iface = iface
				debugLog.Printf("Found default IPv6 route on interface: %s\n", iface.Name)
				break
			}
		}
	}

	// Get IPv4 address from the interface with default route
	if ipv4Iface != nil {
		addrsList, err := ipv4Iface.Addrs()
		if err == nil {
			for _, addr := range addrsList {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}

				if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
					addrs.IPV4 = ip.String()
					debugLog.Printf("Using IPv4 address from %s: %s\n", ipv4Iface.Name, ip.String())
					break
				}
			}
		}
	}

	// Get IPv6 address from the interface with default route
	if ipv6Iface != nil {
		addrsList, err := ipv6Iface.Addrs()
		if err == nil {
			// Collect all global unicast IPv6 addresses
			var candidates []net.IP
			for _, addr := range addrsList {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}

				if ip != nil && ip.To4() == nil && !ip.IsLoopback() {
					// Check if it's a global unicast address (excludes link-local fe80::/10)
					if ip.IsGlobalUnicast() {
						candidates = append(candidates, ip)
					}
				}
			}

			// Collect all non-temporary global unicast IPv6 addresses
			for _, ip := range candidates {
				ipStr := ip.String()
				// Prefer addresses with :: compression (typically static configs)
				// Skip temporary/privacy addresses (heuristic: longer addresses)
				if len(ipStr) < 30 {
					addrs.IPV6 = append(addrs.IPV6, ipStr)
					debugLog.Printf("Found IPv6 address from %s: %s\n", ipv6Iface.Name, ipStr)
				}
			}

			// If no short addresses found, use the first candidate
			if len(addrs.IPV6) == 0 && len(candidates) > 0 {
				addrs.IPV6 = append(addrs.IPV6, candidates[0].String())
				debugLog.Printf("Using IPv6 address from %s: %s\n", ipv6Iface.Name, candidates[0].String())
			}
		}
	}

	if addrs.IPV4 == "" {
		return addrs, fmt.Errorf("could not determine source IPv4 address from default route")
	}

	return addrs, nil
}
