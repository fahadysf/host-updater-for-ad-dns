package main

import (
	"fmt"
	"net"
)

func getDefaultInterfaceAddressesFallback() (IPAddrs, error) {
	addrs := IPAddrs{}

	// Use UDP dial to determine which local address would be used to reach the internet
	// This works cross-platform without parsing route tables

	// Get IPv4 address by dialing Google DNS
	conn, err := net.Dial("udp4", "8.8.8.8:53")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		addrs.IPV4 = localAddr.IP.String()
		debugLog.Printf("Detected IPv4 via dial: %s\n", addrs.IPV4)
	} else {
		debugLog.Printf("Error detecting IPv4 via dial: %v\n", err)
	}

	// Get IPv6 address by dialing Google DNS over IPv6
	conn6, err := net.Dial("udp6", "[2001:4860:4860::8888]:53")
	if err == nil {
		defer conn6.Close()
		localAddr := conn6.LocalAddr().(*net.UDPAddr)
		// Get the interface for this address to find all addresses on it
		ipv6FromDial := localAddr.IP.String()
		debugLog.Printf("Detected IPv6 via dial: %s\n", ipv6FromDial)

		// Find the interface with this address
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				addrsList, err := iface.Addrs()
				if err != nil {
					continue
				}

				hasDialedAddr := false
				var candidates []net.IP

				for _, addr := range addrsList {
					var ip net.IP
					switch v := addr.(type) {
					case *net.IPNet:
						ip = v.IP
					case *net.IPAddr:
						ip = v.IP
					}

					if ip == nil || ip.To4() != nil {
						continue
					}

					if ip.String() == ipv6FromDial {
						hasDialedAddr = true
					}

					if ip.IsGlobalUnicast() {
						candidates = append(candidates, ip)
					}
				}

				// If this interface has the dialed address, choose the best IPv6 from it
				if hasDialedAddr {
					debugLog.Printf("Found interface %s with dialed IPv6 address\n", iface.Name)
					// Prefer shorter addresses (non-temporary)
					for _, ip := range candidates {
						ipStr := ip.String()
						if len(ipStr) < 30 {
							addrs.IPV6 = ipStr
							debugLog.Printf("Using IPv6 address: %s\n", ipStr)
							break
						}
					}

					if addrs.IPV6 == "" && len(candidates) > 0 {
						addrs.IPV6 = candidates[0].String()
						debugLog.Printf("Using IPv6 address: %s\n", candidates[0].String())
					}
					break
				}
			}
		}
	} else {
		debugLog.Printf("Error detecting IPv6 via dial: %v\n", err)
	}

	if addrs.IPV4 == "" {
		return addrs, fmt.Errorf("could not determine source IPv4 address")
	}

	return addrs, nil
}