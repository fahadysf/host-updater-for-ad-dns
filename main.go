package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Result struct {
	Server             string         `json:"server" yaml:"server"`
	ARecordsFound      []string       `json:"a_records_found" yaml:"a_records_found"`
	ARecordUpdates     []UpdateResult `json:"a_record_updates" yaml:"a_record_updates"`
	AAAARecordsFound   []string       `json:"aaaa_records_found" yaml:"aaaa_records_found"`
	AAAARecordUpdates  []UpdateResult `json:"aaaa_record_updates" yaml:"aaaa_record_updates"`
	AAAARecordRemovals []UpdateResult `json:"aaaa_record_removals,omitempty" yaml:"aaaa_record_removals,omitempty"`
}

type UpdateResult struct {
	IP      string `json:"ip" yaml:"ip"`
	Status  string `json:"status" yaml:"status"`
	Message string `json:"message" yaml:"message"`
}

type Output struct {
	Hostname          string   `json:"hostname" yaml:"hostname"`
	Domain            string   `json:"domain" yaml:"domain"`
	FQDN              string   `json:"fqdn" yaml:"fqdn"`
	SourceUPs         IPAddrs  `json:"source_ips" yaml:"source_ips"`
	DNSServersQueried []string `json:"dns_servers_queried" yaml:"dns_servers_queried"`
	Results           []Result `json:"results" yaml:"results"`
}

type IPAddrs struct {
	IPV4 string   `json:"ipv4" yaml:"ipv4"`
	IPV6 []string `json:"ipv6" yaml:"ipv6"`
}

func main() {
	// Define command-line flags
	domain := flag.String("domain", "", "Domain to check (e.g., fy.loc).")
	nameservers := flag.String("nameservers", "", "Comma-separated list of DNS servers to use.")
	hostname := flag.String("hostname", "", "Hostname to check. Defaults to the local hostname.")
	updateDNS := flag.Bool("update-dns", false, "Enable DNS update functionality.")
	adUser := flag.String("ad-user", "", "Active Directory username for DNS update.")
	adPassword := flag.String("ad-password", "", "Active Directory password for DNS update. If not provided, will be prompted.")
	manualIP := flag.String("ip", "", "Manual IPv4 address to check/update. Skips IPv4 auto-detection.")
	manualIPv6 := flag.String("ipv6", "", "Comma-separated manual IPv6 addresses. Skips IPv6 auto-detection.")
	debug := flag.Bool("debug", false, "Enable debug logging.")
	outputFormat := flag.String("o", OutputPretty, "Output format: pretty (default), json, or yaml.")
	showVersion := flag.Bool("version", false, "Show version and exit.")
	autoUpdate := flag.Bool("auto-update", false, "Check for and install updates after execution.")

	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Validate output format
	if *outputFormat != OutputPretty && *outputFormat != OutputJSON && *outputFormat != OutputYAML {
		fmt.Fprintf(os.Stderr, "Error: invalid output format '%s'. Valid options: pretty, json, yaml\n", *outputFormat)
		os.Exit(1)
	}

	// Initialize progress display
	progress := NewProgressDisplay(*outputFormat)

	initLogger(*debug)

	// Validate required flags
	if *domain == "" || *nameservers == "" {
		fmt.Fprintln(os.Stderr, "Error: --domain and --nameservers are required.")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Get hostname if not provided
	if *hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting hostname: %v\n", err)
			os.Exit(1)
		}
		*hostname = strings.Split(h, ".")[0]
	}

	// Process nameservers
	serverList := strings.Split(*nameservers, ",")
	for i := range serverList {
		serverList[i] = strings.TrimSpace(serverList[i])
	}

	// Create the output structure
	output := Output{
		Hostname:          *hostname,
		Domain:            *domain,
		FQDN:              fmt.Sprintf("%s.%s", *hostname, *domain),
		DNSServersQueried: serverList,
	}

	// Handle password prompt before starting progress display
	if *updateDNS && *adUser != "" && *adPassword == "" {
		fmt.Printf("Enter AD password for user %s: ", *adUser)
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nPassword entry cancelled: %v\n", err)
			os.Exit(1)
		}
		*adPassword = string(bytePassword)
		fmt.Println()
	}

	// Print header for pretty output
	if *outputFormat == OutputPretty {
		progress.PrintHeader(fmt.Sprintf("DNS Updater - %s", output.FQDN))
		progress.PrintSeparator()
	}

	// Create steps for progress display
	stepDiscover := progress.AddStep("Discover local IP addresses")
	serverSteps := make(map[string]*Step)
	serverLookupSteps := make(map[string]*Step)
	serverUpdateSteps := make(map[string]*Step)

	for _, server := range serverList {
		serverSteps[server] = progress.AddStep(fmt.Sprintf("Check DNS server %s", server))
		serverLookupSteps[server] = progress.AddStep(fmt.Sprintf("Lookup records on %s", server))
		if *updateDNS {
			serverUpdateSteps[server] = progress.AddStep(fmt.Sprintf("Update records on %s", server))
		}
	}

	// Start progress display
	progress.Start()

	// Determine IPs to check
	progress.StartStep(stepDiscover, "detecting network interfaces...")
	var sourceIPs IPAddrs

	// Check if manual IPs are provided
	if *manualIP != "" || *manualIPv6 != "" {
		if *manualIP != "" {
			sourceIPs.IPV4 = *manualIP
		}
		if *manualIPv6 != "" {
			for _, v6 := range strings.Split(*manualIPv6, ",") {
				v6 = strings.TrimSpace(v6)
				if v6 != "" {
					sourceIPs.IPV6 = append(sourceIPs.IPV6, v6)
				}
			}
		}

		// Only auto-detect if not manually specified
		if sourceIPs.IPV4 == "" || len(sourceIPs.IPV6) == 0 {
			autoDetected, err := getDefaultInterfaceAddresses()
			if err != nil && sourceIPs.IPV4 == "" {
				progress.CompleteStep(stepDiscover, StepFailure, "failed", err.Error())
				progress.Stop()
				os.Exit(1)
			}
			if sourceIPs.IPV4 == "" {
				sourceIPs.IPV4 = autoDetected.IPV4
			}
			if len(sourceIPs.IPV6) == 0 {
				sourceIPs.IPV6 = autoDetected.IPV6
			}
		}

		ipInfo := ""
		if sourceIPs.IPV4 != "" {
			ipInfo = sourceIPs.IPV4
		}
		for _, v6 := range sourceIPs.IPV6 {
			if ipInfo != "" {
				ipInfo += ", "
			}
			ipInfo += v6
		}
		progress.CompleteStep(stepDiscover, StepSuccess, "using manual IP", ipInfo)
	} else {
		var err error
		sourceIPs, err = getDefaultInterfaceAddresses()
		if err != nil {
			progress.CompleteStep(stepDiscover, StepFailure, "failed", err.Error())
			progress.Stop()
			os.Exit(1)
		}
		ipInfo := sourceIPs.IPV4
		for _, v6 := range sourceIPs.IPV6 {
			ipInfo += ", " + v6
		}
		progress.CompleteStep(stepDiscover, StepSuccess, "found", ipInfo)
	}
	output.SourceUPs = sourceIPs

	// Find live servers
	var liveServers []string
	for _, server := range serverList {
		step := serverSteps[server]
		progress.StartStep(step, "checking connectivity...")

		if checkDNSServerLiveness(server) {
			liveServers = append(liveServers, server)
			progress.CompleteStep(step, StepSuccess, "online", "")
		} else {
			progress.CompleteStep(step, StepFailure, "not responding", "")
			output.Results = append(output.Results, Result{
				Server: server,
				ARecordUpdates: []UpdateResult{
					{Status: "skipped", Message: "DNS server is not responding."},
				},
				AAAARecordUpdates: []UpdateResult{
					{Status: "skipped", Message: "DNS server is not responding."},
				},
			})
			// Mark subsequent steps as skipped
			progress.CompleteStep(serverLookupSteps[server], StepSkipped, "server offline", "")
			if *updateDNS {
				progress.CompleteStep(serverUpdateSteps[server], StepSkipped, "server offline", "")
			}
		}
	}

	// Perform DNS lookups and updates on live servers
	for _, server := range liveServers {
		result := Result{Server: server}
		lookupStep := serverLookupSteps[server]

		// DNS Lookups
		progress.StartStep(lookupStep, "querying A and AAAA records...")

		// A Record
		aRecords, err := performDNSLookup(server, output.FQDN, "A")
		if err != nil {
			debugLog.Printf("Error looking up A records on %s: %v\n", server, err)
		}
		result.ARecordsFound = aRecords

		// AAAA Record
		aaaaRecords, err := performDNSLookup(server, output.FQDN, "AAAA")
		if err != nil {
			debugLog.Printf("Error looking up AAAA records on %s: %v\n", server, err)
		}
		result.AAAARecordsFound = aaaaRecords

		recordInfo := fmt.Sprintf("A:%d AAAA:%d", len(aRecords), len(aaaaRecords))
		progress.CompleteStep(lookupStep, StepSuccess, "found", recordInfo)

		// Updates
		if *updateDNS {
			updateStep := serverUpdateSteps[server]
			progress.StartStep(updateStep, "checking if updates needed...")

			updatesNeeded := 0
			updatesSuccess := 0
			updatesFailed := 0

			// A Record Update
			aFound := false
			for _, r := range aRecords {
				if r == sourceIPs.IPV4 {
					aFound = true
					break
				}
			}
			if !aFound && sourceIPs.IPV4 != "" {
				updatesNeeded++
				progress.StartStep(updateStep, fmt.Sprintf("updating A record to %s...", sourceIPs.IPV4))
				updateResult, err := updateWindowsDNSRecord(server, *adUser, *adPassword, *domain, *hostname, sourceIPs.IPV4, "A")
				if err != nil {
					debugLog.Printf("Error updating A record on %s: %v\n", server, err)
					updatesFailed++
				} else if updateResult.Status == "success" || updateResult.Status == "updated" {
					updatesSuccess++
				} else {
					updatesFailed++
				}
				result.ARecordUpdates = append(result.ARecordUpdates, updateResult)
			} else {
				result.ARecordUpdates = append(result.ARecordUpdates, UpdateResult{IP: sourceIPs.IPV4, Status: "skipped", Message: "Record already correct."})
			}

			// AAAA Record Updates - add missing and remove stale
			if len(sourceIPs.IPV6) > 0 {
				// Build a set of desired IPv6 addresses
				desiredIPv6 := make(map[string]bool)
				for _, v6 := range sourceIPs.IPV6 {
					desiredIPv6[v6] = true
				}

				// Add missing AAAA records
				for _, v6 := range sourceIPs.IPV6 {
					found := false
					for _, r := range aaaaRecords {
						if r == v6 {
							found = true
							break
						}
					}
					if !found {
						updatesNeeded++
						progress.StartStep(updateStep, fmt.Sprintf("adding AAAA record %s...", v6))
						updateResult, err := updateWindowsDNSRecord(server, *adUser, *adPassword, *domain, *hostname, v6, "AAAA")
						if err != nil {
							debugLog.Printf("Error updating AAAA record on %s: %v\n", server, err)
							updatesFailed++
						} else if updateResult.Status == "success" || updateResult.Status == "updated" {
							updatesSuccess++
						} else {
							updatesFailed++
						}
						result.AAAARecordUpdates = append(result.AAAARecordUpdates, updateResult)
					} else {
						result.AAAARecordUpdates = append(result.AAAARecordUpdates, UpdateResult{IP: v6, Status: "skipped", Message: "Record already correct."})
					}
				}

				// Remove stale AAAA records (in DNS but not on host)
				for _, r := range aaaaRecords {
					if !desiredIPv6[r] {
						updatesNeeded++
						progress.StartStep(updateStep, fmt.Sprintf("removing stale AAAA record %s...", r))
						removeResult, err := removeWindowsDNSRecord(server, *adUser, *adPassword, *domain, *hostname, r, "AAAA")
						if err != nil {
							debugLog.Printf("Error removing stale AAAA record %s on %s: %v\n", r, server, err)
							updatesFailed++
						} else if removeResult.Status == "success" {
							updatesSuccess++
						} else {
							updatesFailed++
						}
						result.AAAARecordRemovals = append(result.AAAARecordRemovals, removeResult)
					}
				}
			}

			// Determine update step status
			if updatesNeeded == 0 {
				progress.CompleteStep(updateStep, StepSkipped, "no updates needed", "")
			} else if updatesFailed > 0 {
				progress.CompleteStep(updateStep, StepFailure, "failed", fmt.Sprintf("%d/%d", updatesFailed, updatesNeeded))
			} else {
				progress.CompleteStep(updateStep, StepSuccess, "updated", fmt.Sprintf("%d records", updatesSuccess))
			}
		}

		output.Results = append(output.Results, result)
	}

	// Stop progress display
	progress.Stop()

	// Print final output
	PrintFinalOutput(output, *outputFormat)

	// Self-update check (non-fatal)
	if *autoUpdate {
		selfUpdate(*outputFormat)
	}
}
