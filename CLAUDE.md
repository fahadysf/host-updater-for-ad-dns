# DNS Updater - Go Port

## Project Goal

Port the DNS updating functionality from Python to Go to create a cross-platform compatible binary that:

1. Discovers local IPv4/IPv6 addresses from network interfaces
2. Queries specified DNS servers for A and AAAA records
3. Updates Windows DNS records via WinRM when they don't match the local IP

## Architecture

```
main.go           - CLI entry point, argument parsing, orchestration
dns.go            - DNS server liveness checks and record lookups
ip_discovery.go   - Local network interface IP address discovery
winrm.go          - Windows Remote Management for DNS record updates (Kerberos/HTTPS)
logger.go         - Debug logging infrastructure
```

## CLI Usage

```bash
./dns-updater \
  --domain "fy.loc" \
  --nameservers "192.168.100.85,192.168.100.86" \
  --hostname "myhost" \
  --update-dns \
  --ad-user "fahad@fy.loc" \
  --ad-password "password" \
  --debug
```

### Flags

| Flag | Description |
|------|-------------|
| `--domain` | Domain to check (e.g., fy.loc) - **required** |
| `--nameservers` | Comma-separated list of DNS server IPs - **required** |
| `--hostname` | Hostname to check (defaults to local hostname) |
| `--update-dns` | Enable DNS update functionality |
| `--ad-user` | AD username (UPN format: user@domain or DOMAIN\user) |
| `--ad-password` | AD password (prompted if not provided) |
| `--ip` | Manual IPv4 address (skips auto-detection) |
| `--debug` | Enable debug logging |

## Authentication

The tool uses **Kerberos authentication over HTTPS** (port 5986) for secure WinRM connections.

### Server Requirements

Each Windows DNS server must have HTTPS WinRM listener enabled:

```powershell
# Run as Administrator on each DNS server
$cert = New-SelfSignedCertificate -DnsName "dc.fy.loc" -CertStoreLocation Cert:\LocalMachine\My
winrm create winrm/config/Listener?Address=*+Transport=HTTPS "@{Hostname=`"dc.fy.loc`"; CertificateThumbprint=`"$($cert.Thumbprint)`"}"
New-NetFirewallRule -DisplayName "WinRM HTTPS" -Direction Inbound -LocalPort 5986 -Protocol TCP -Action Allow
```

### How It Works

1. Extracts realm from username (user@domain.loc → FY.LOC)
2. Creates temporary Kerberos configuration
3. Obtains TGT from KDC using password
4. Gets service ticket for HTTP/hostname SPN
5. Authenticates to WinRM over HTTPS
6. Executes PowerShell DNS commands

## Building

```bash
go build -o dns-updater .
```

## Cross-Platform Compilation

```bash
# Windows
GOOS=windows GOARCH=amd64 go build -o dns-updater.exe .

# Linux
GOOS=linux GOARCH=amd64 go build -o dns-updater-linux .

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o dns-updater-darwin .

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o dns-updater-darwin-arm64 .
```

## Key Dependencies

- `github.com/masterzen/winrm` - WinRM client with Kerberos support
- `github.com/jcmturner/gokrb5/v8` - Kerberos authentication
- `golang.org/x/term` - Secure password input

## Issues Fixed

### Original Issue: "invalid content type" with NTLM

**Problem**: The original code used NTLM authentication which was rejected by the Windows server (only Negotiate/Kerberos accepted).

**Solution**: Implemented Kerberos authentication with automatic:
- Realm extraction from UPN
- Reverse DNS lookup for SPN (IP → hostname)
- Dynamic krb5.conf generation
- HTTPS transport for encryption (required by modern Windows Server)

### PowerShell Execution

**Problem**: Commands were executing in cmd.exe instead of PowerShell.

**Solution**: Use `winrm.Powershell()` to Base64-encode scripts for PowerShell execution.
