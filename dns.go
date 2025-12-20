package main

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/miekg/dns"
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

	// Create DNS message
	m := new(dns.Msg)
	var qtype uint16
	switch recordType {
	case "A":
		qtype = dns.TypeA
	case "AAAA":
		qtype = dns.TypeAAAA
	default:
		return nil, fmt.Errorf("unsupported record type: %s", recordType)
	}
	m.SetQuestion(dns.Fqdn(fqdn), qtype)
	m.RecursionDesired = true

	// Create DNS client
	c := new(dns.Client)
	c.Timeout = 5 * time.Second

	// Query the DNS server
	r, _, err := c.Exchange(m, net.JoinHostPort(server, "53"))
	if err != nil {
		debugLog.Printf("DNS query failed on server %s for %s (%s): %v\n", server, fqdn, recordType, err)
		return nil, err
	}

	// Check response code
	if r.Rcode != dns.RcodeSuccess {
		if r.Rcode == dns.RcodeNameError {
			debugLog.Printf("DNS lookup on server %s for %s (%s) returned NXDOMAIN\n", server, fqdn, recordType)
			return []string{}, nil
		}
		debugLog.Printf("DNS lookup on server %s for %s (%s) returned error code: %d\n", server, fqdn, recordType, r.Rcode)
		return nil, fmt.Errorf("DNS query failed with rcode: %d", r.Rcode)
	}

	// Extract answers
	var results []string
	for _, ans := range r.Answer {
		switch recordType {
		case "A":
			if a, ok := ans.(*dns.A); ok {
				results = append(results, a.A.String())
			}
		case "AAAA":
			if aaaa, ok := ans.(*dns.AAAA); ok {
				results = append(results, aaaa.AAAA.String())
			}
		}
	}

	debugLog.Printf("DNS lookup on server %s for %s (%s) returned: %v\n", server, fqdn, recordType, results)
	return results, nil
}