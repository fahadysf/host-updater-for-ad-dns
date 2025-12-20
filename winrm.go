package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/masterzen/winrm"
)

// isIPAddress checks if the given string is an IP address
func isIPAddress(s string) bool {
	return net.ParseIP(s) != nil
}

// createKerberosConfig creates a minimal krb5.conf for the given realm
func createKerberosConfig(realm, kdc string) (string, error) {
	krbConf := fmt.Sprintf(`[libdefaults]
    default_realm = %s
    dns_lookup_realm = false
    dns_lookup_kdc = false
    forwardable = true

[realms]
    %s = {
        kdc = %s
        admin_server = %s
    }

[domain_realm]
    .%s = %s
    %s = %s
`, strings.ToUpper(realm), strings.ToUpper(realm), kdc, kdc,
		strings.ToLower(realm), strings.ToUpper(realm),
		strings.ToLower(realm), strings.ToUpper(realm))

	tmpDir := os.TempDir()
	krbPath := filepath.Join(tmpDir, "dns-updater-krb5.conf")
	if err := os.WriteFile(krbPath, []byte(krbConf), 0600); err != nil {
		return "", err
	}
	return krbPath, nil
}

func updateWindowsDNSRecord(server, username, password, zone, name, ip, recordType string) (UpdateResult, error) {
	endpoint := winrm.NewEndpoint(server, 5985, false, true, nil, nil, nil, 0)

	// Extract realm and username from UPN or domain\user format
	var user, realm string
	if strings.Contains(username, "@") {
		parts := strings.Split(username, "@")
		user = parts[0]
		realm = parts[1]
	} else if strings.Contains(username, "\\") {
		parts := strings.Split(username, "\\")
		realm = parts[0]
		user = parts[1]
	} else {
		// Assume zone as realm
		user = username
		realm = zone
	}

	// Create Kerberos config
	krbPath, err := createKerberosConfig(realm, server)
	if err != nil {
		debugLog.Printf("Failed to create Kerberos config: %v, falling back to NTLM\n", err)
		// Fall back to NTLM
		params := winrm.DefaultParameters
		params.TransportDecorator = func() winrm.Transporter {
			return &winrm.ClientNTLM{}
		}
		client, err := winrm.NewClientWithParameters(endpoint, username, password, params)
		if err != nil {
			return UpdateResult{Status: "error"}, err
		}
		return executeWinRMCommand(client, server, zone, name, ip, recordType)
	}
	defer os.Remove(krbPath)

	debugLog.Printf("Using Kerberos authentication with realm %s\n", strings.ToUpper(realm))

	// Determine the SPN - try to use hostname instead of IP for SPN
	spnHost := server
	// If server looks like an IP, try reverse DNS to get hostname for SPN
	if isIPAddress(server) {
		names, err := net.LookupAddr(server)
		if err == nil && len(names) > 0 {
			// Use the first name, removing trailing dot
			spnHost = strings.TrimSuffix(names[0], ".")
			debugLog.Printf("Resolved IP %s to hostname %s for SPN\n", server, spnHost)
		}
	}

	// For WinRM, try HTTP/hostname SPN format
	spn := fmt.Sprintf("HTTP/%s", spnHost)
	debugLog.Printf("Using SPN: %s\n", spn)

	// Use HTTPS with Kerberos for secure encrypted transport
	// Use hostname for both connection and SPN (required for TLS cert validation)
	settings := &winrm.Settings{
		WinRMUsername: user,
		WinRMPassword: password,
		WinRMHost:     spnHost, // Use hostname for HTTPS (cert is issued to hostname)
		WinRMPort:     5986,
		WinRMProto:    "https",
		WinRMInsecure: true, // Skip cert verification for self-signed certs
		KrbRealm:      strings.ToUpper(realm),
		KrbConfig:     krbPath,
		KrbSpn:        spn,
	}

	// Update endpoint to use HTTPS
	endpoint = winrm.NewEndpoint(spnHost, 5986, true, true, nil, nil, nil, 0)

	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter {
		return winrm.NewClientKerberos(settings)
	}

	client, err := winrm.NewClientWithParameters(endpoint, user, password, params)
	if err != nil {
		return UpdateResult{Status: "error"}, err
	}

	return executeWinRMCommand(client, server, zone, name, ip, recordType)
}

// executeWinRMCommand runs the PowerShell DNS update command
func executeWinRMCommand(client *winrm.Client, server, zone, name, ip, recordType string) (UpdateResult, error) {
	recordType = strings.ToUpper(recordType)
	ipAddressProperty := "IPv4Address"
	if recordType == "AAAA" {
		ipAddressProperty = "IPv6Address"
	}

	psScript := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
try {
    $records = Get-DnsServerResourceRecord -ZoneName "%s" -Name "%s" -RRType %s -ErrorAction SilentlyContinue
    $existing_ips = @($records | ForEach-Object { $_.RecordData.%s })

    if ('%s' -in $existing_ips) {
        Write-Host "Record %s already exists with the correct IP"
    } else {
        Add-DnsServerResourceRecord -%s -ZoneName "%s" -Name "%s" -%s "%s" -TimeToLive (New-TimeSpan -Hours 1) -PassThru
        Write-Host "Created new %s record for %s"
    }
} catch {
    Write-Error $_.Exception.Message
}`, zone, name, recordType, ipAddressProperty, ip, ip, recordType, zone, name, ipAddressProperty, ip, recordType, ip)

	debugLog.Printf("Executing PowerShell script on %s:\n%s\n", server, psScript)

	// Use winrm.Powershell to properly encode the script for PowerShell execution
	psCmd := winrm.Powershell(psScript)

	var stdOut, stdErr bytes.Buffer
	_, err := client.Run(psCmd, &stdOut, &stdErr)

	debugLog.Printf("WinRM stdout:\n%s\n", stdOut.String())
	debugLog.Printf("WinRM stderr:\n%s\n", stdErr.String())

	// Parse the output to create human-readable messages
	stdOutStr := strings.TrimSpace(stdOut.String())
	stdErrStr := strings.TrimSpace(stdErr.String())

	// First, check for success indicators in stdout
	// This takes priority over error checking because PowerShell may write to stderr
	// even on successful operations (verbose output, warnings, etc.)
	hasSuccessIndicator := false
	var message string

	if strings.Contains(stdOutStr, "already exists with the correct IP") {
		hasSuccessIndicator = true
		message = "Record already exists"
	} else if strings.Contains(stdOutStr, "Created new A record for") {
		hasSuccessIndicator = true
		message = "Record created successfully"
	} else if strings.Contains(stdOutStr, "Created new AAAA record for") {
		hasSuccessIndicator = true
		message = "Record created successfully"
	} else if strings.Contains(stdOutStr, "Created new") {
		hasSuccessIndicator = true
		message = "Record created successfully"
	}

	// If we found success indicators, return success regardless of stderr
	if hasSuccessIndicator {
		return UpdateResult{Status: "success", Message: message}, nil
	}

	// If no success indicators, check for errors
	if err != nil || stdErrStr != "" {
		// Extract error message from stderr
		errMsg := "Failed to update DNS record"
		if stdErrStr != "" {
			// Try to extract meaningful error from PowerShell error output
			lines := strings.Split(stdErrStr, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "At line:") && !strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "CategoryInfo") && !strings.HasPrefix(line, "FullyQualifiedErrorId") {
					errMsg = line
					break
				}
			}
		}
		return UpdateResult{Status: "error", Message: errMsg}, err
	}

	// Fallback for unexpected success case (no error, no known success message)
	return UpdateResult{Status: "success", Message: "Record updated successfully"}, nil
}