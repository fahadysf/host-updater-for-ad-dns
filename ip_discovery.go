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

		// Find the interface with this address and collect all global unicast IPv6
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				addrsList, err := iface.Addrs()
				if err != nil {
					continue
				}

				hasDialedAddr := false

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
						break
					}
				}

				// If this interface has the dialed address, collect all global unicast IPv6 from it
				if hasDialedAddr {
					debugLog.Printf("Found interface %s with dialed IPv6 address\n", iface.Name)
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

						if ip.IsGlobalUnicast() {
							addrs.IPV6 = append(addrs.IPV6, ip.String())
							debugLog.Printf("Found IPv6 address: %s\n", ip.String())
						}
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
