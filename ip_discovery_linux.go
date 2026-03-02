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

	// Get IPv6 addresses from the interface with default route
	// Use netlink to get address flags so we can skip deprecated addresses
	if ipv6Iface != nil {
		link, err := netlink.LinkByIndex(ipv6Iface.Index)
		if err != nil {
			debugLog.Printf("Error getting link for %s: %v\n", ipv6Iface.Name, err)
		} else {
			nlAddrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
			if err != nil {
				debugLog.Printf("Error listing IPv6 addrs on %s: %v\n", ipv6Iface.Name, err)
			} else {
				for _, nlAddr := range nlAddrs {
					ip := nlAddr.IP
					if ip == nil || ip.To4() != nil || !ip.IsGlobalUnicast() {
						continue
					}
					// Skip deprecated addresses (preferred lifetime expired)
					if nlAddr.PreferedLft == 0 {
						debugLog.Printf("Skipping deprecated IPv6 address: %s\n", ip.String())
						continue
					}
					addrs.IPV6 = append(addrs.IPV6, ip.String())
					debugLog.Printf("Found IPv6 address from %s: %s (preferred_lft=%d)\n", ipv6Iface.Name, ip.String(), nlAddr.PreferedLft)
				}
			}
		}
	}

	if addrs.IPV4 == "" {
		return addrs, fmt.Errorf("could not determine source IPv4 address from default route")
	}

	return addrs, nil
}
