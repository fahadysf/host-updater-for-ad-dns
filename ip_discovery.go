package main

import (
	"fmt"
	"net"
)

func getDefaultInterfaceAddresses() (IPAddrs, error) {
	addrs := IPAddrs{}
	
	interfaces, err := net.Interfaces()
	if err != nil {
		return addrs, fmt.Errorf("error getting network interfaces: %w", err)
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			// Interface is down or a loopback interface, skip
			continue
		}

		addrsList, err := iface.Addrs()
		if err != nil {
			return addrs, fmt.Errorf("error getting addresses for interface %s: %w", iface.Name, err)
		}

		for _, addr := range addrsList {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			if ip.To4() != nil {
				// IPv4 address
				addrs.IPV4 = ip.String()
			} else {
				// IPv6 address
				// Exclude link-local addresses (fe80::/10)
				if ip.IsGlobalUnicast() { // This checks for global unicast which excludes link-local, site-local, etc.
					addrs.IPV6 = ip.String()
				}
			}
		}
	}

	if addrs.IPV4 == "" {
		return addrs, fmt.Errorf("could not determine source IPv4 address")
	}

	return addrs, nil
}