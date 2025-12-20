package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

func checkDNSServerLiveness(server string) bool {
	debugLog.Printf("Checking liveness of DNS server %s\n", server)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := resolver.LookupHost(ctx, "_gemini_test.")
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			debugLog.Printf("DNS server %s is not responding: %v\n", server, err)
			return false
		}
	}
	debugLog.Printf("DNS server %s is alive\n", server)
	return true
}

func performDNSLookup(server, fqdn, recordType string) ([]string, error) {
	debugLog.Printf("Performing DNS lookup on server %s for %s (%s)\n", server, fqdn, recordType)
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 5,
			}
			return d.DialContext(ctx, "udp", net.JoinHostPort(server, "53"))
		},
	}

	var results []string
	var err error

	switch recordType {
	case "A":
		ips, err := resolver.LookupIP(context.Background(), "ip4", fqdn)
		if err == nil {
			for _, ip := range ips {
				results = append(results, ip.String())
			}
		}
	case "AAAA":
		ips, err := resolver.LookupIP(context.Background(), "ip6", fqdn)
		if err == nil {
			for _, ip := range ips {
				results = append(results, ip.String())
			}
		}
	default:
		err = fmt.Errorf("unsupported record type: %s", recordType)
	}

	if e, ok := err.(*net.DNSError); ok && e.IsNotFound {
		debugLog.Printf("DNS lookup on server %s for %s (%s) returned no records\n", server, fqdn, recordType)
		return []string{}, nil
	}

	debugLog.Printf("DNS lookup on server %s for %s (%s) returned: %v\n", server, fqdn, recordType, results)
	return results, err
}