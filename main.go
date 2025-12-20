package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Result struct {
	Server             string        `json:"server"`
	ARecordsFound      []string      `json:"a_records_found"`
	ARecordUpdates     []UpdateResult `json:"a_record_updates"`
	AAAARecordsFound   []string      `json:"aaaa_records_found"`
	AAAARecordUpdates  []UpdateResult `json:"aaaa_record_updates"`
}

type UpdateResult struct {
	IP      string `json:"ip"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Output struct {
	Hostname           string   `json:"hostname"`
	Domain             string   `json:"domain"`
	FQDN               string   `json:"fqdn"`
	SourceUPs          IPAddrs  `json:"source_ips"`
	DNSServersQueried  []string `json:"dns_servers_queried"`
	Results            []Result `json:"results"`
}

type IPAddrs struct {
	IPV4 string `json:"ipv4"`
	IPV6 string `json:"ipv6"`
}

func main() {
	// Define command-line flags
	domain := flag.String("domain", "", "Domain to check (e.g., fy.loc).")
	nameservers := flag.String("nameservers", "", "Comma-separated list of DNS servers to use.")
	hostname := flag.String("hostname", "", "Hostname to check. Defaults to the local hostname.")
	updateDNS := flag.Bool("update-dns", false, "Enable DNS update functionality.")
	adUser := flag.String("ad-user", "", "Active Directory username for DNS update.")
	adPassword := flag.String("ad-password", "", "Active Directory password for DNS update. If not provided, will be prompted.")
	manualIP := flag.String("ip", "", "Manual IPv4 address to check/update. Skips auto-detection.")
	debug := flag.Bool("debug", false, "Enable debug logging.")

	flag.Parse()

	initLogger(*debug)

	// Validate required flags
	if *domain == "" || *nameservers == "" {
		fmt.Println("Error: --domain and --nameservers are required.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get hostname if not provided
	if *hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			fmt.Printf("Error getting hostname: %v\n", err)
			os.Exit(1)
		}
		*hostname = strings.Split(h, ".")[0]
	}

	// Process nameservers
	serverList := strings.Split(*nameservers, ",")

	// Create the output structure
	output := Output {
		Hostname: *hostname,
		Domain: *domain,
		FQDN: fmt.Sprintf("%s.%s", *hostname, *domain),
		DNSServersQueried: serverList,
	}

	// Determine IPs to check
	var sourceIPs IPAddrs
	if *manualIP != "" {
		sourceIPs.IPV4 = *manualIP
	} else {
		var err error
		sourceIPs, err = getDefaultInterfaceAddresses()
		if err != nil {
			fmt.Printf("Error getting local IP addresses: %v\n", err)
			os.Exit(1)
		}
	}
	output.SourceUPs = sourceIPs

	// Handle password prompt
	if *updateDNS && *adUser != "" && *adPassword == "" {
		fmt.Printf("Enter AD password for user %s: ", *adUser)
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Printf("\nPassword entry cancelled: %v\n", err)
			os.Exit(1)
		}
		*adPassword = string(bytePassword)
		fmt.Println()
	}

	// Find live servers
	var liveServers []string
	for _, server := range serverList {
		server = strings.TrimSpace(server)
		if checkDNSServerLiveness(server) {
			liveServers = append(liveServers, server)
		} else {
			output.Results = append(output.Results, Result{
				Server: server,
				ARecordUpdates: []UpdateResult{
					{Status: "skipped", Message: "DNS server is not responding."},
				},
				AAAARecordUpdates: []UpdateResult{
					{Status: "skipped", Message: "DNS server is not responding."},
				},
			})
		}
	}

	// Perform DNS lookups and updates on live servers
	for _, server := range liveServers {
		result := Result{Server: server}

		// A Record
		aRecords, err := performDNSLookup(server, output.FQDN, "A")
		if err != nil {
			fmt.Printf("Error looking up A records on %s: %v\n", server, err)
		}
		result.ARecordsFound = aRecords

		if *updateDNS {
			found := false
			for _, r := range aRecords {
				if r == sourceIPs.IPV4 {
					found = true
					break
				}
			}
			if !found {
				updateResult, err := updateWindowsDNSRecord(server, *adUser, *adPassword, *domain, *hostname, sourceIPs.IPV4, "A")
				if err != nil {
					fmt.Printf("Error updating A record on %s: %v\n", server, err)
				}
				result.ARecordUpdates = append(result.ARecordUpdates, updateResult)
			} else {
				result.ARecordUpdates = append(result.ARecordUpdates, UpdateResult{IP: sourceIPs.IPV4, Status: "skipped", Message: "Record already correct."})
			}
		}

		// AAAA Record
		aaaaRecords, err := performDNSLookup(server, output.FQDN, "AAAA")
		if err != nil {
			fmt.Printf("Error looking up AAAA records on %s: %v\n", server, err)
		}
		result.AAAARecordsFound = aaaaRecords

		if *updateDNS && sourceIPs.IPV6 != "" {
			found := false
			for _, r := range aaaaRecords {
				if r == sourceIPs.IPV6 {
					found = true
					break
				}
			}
			if !found {
				updateResult, err := updateWindowsDNSRecord(server, *adUser, *adPassword, *domain, *hostname, sourceIPs.IPV6, "AAAA")
				if err != nil {
					fmt.Printf("Error updating AAAA record on %s: %v\n", server, err)
				}
				result.AAAARecordUpdates = append(result.AAAARecordUpdates, updateResult)
			} else {
				result.AAAARecordUpdates = append(result.AAAARecordUpdates, UpdateResult{IP: sourceIPs.IPV6, Status: "skipped", Message: "Record already correct."})
			}
		}
		output.Results = append(output.Results, result)
	}

	// Print the JSON output
	jsonOutput, err := json.MarshalIndent(output, "", "    ")
	if err != nil {
		fmt.Printf("Error marshalling JSON: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonOutput))
}
