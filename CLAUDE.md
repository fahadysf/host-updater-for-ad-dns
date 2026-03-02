# DNS Updater - Go Port

## Project Goal

Port the DNS updating functionality from Python to Go to create a cross-platform compatible binary that:

1. Discovers local IPv4/IPv6 addresses from network interfaces (all global unicast IPv6)
2. Queries specified DNS servers for A and AAAA records
3. Updates Windows DNS records via WinRM when they don't match the local IP
4. Removes stale AAAA records that no longer exist on the host

## Architecture

```
main.go           - CLI entry point, argument parsing, orchestration
dns.go            - DNS server liveness checks and record lookups
ip_discovery.go   - Local network interface IP address discovery
winrm.go          - Windows Remote Management for DNS record updates/removals (Kerberos/HTTPS)
logger.go         - Debug logging infrastructure
output.go         - Interactive progress display and output formatting (pretty/json/yaml)
version.go        - Build version (set via ldflags)
Makefile          - Build targets with version embedding
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
| `--ipv6` | Comma-separated manual IPv6 addresses (skips IPv6 auto-detection) |
| `-o` | Output format: `pretty` (default), `json`, or `yaml` |
| `--debug` | Enable debug logging |
| `--version` | Show version and exit |

### Output Formats

- **pretty** (default): Interactive CLI output with Unicode progress indicators (✓/✗) and spinners
- **json**: Structured JSON output for programmatic consumption
- **yaml**: Structured YAML output for configuration/scripting use

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
# With version embedding (recommended)
make build

# Or manually
go build -o dns-updater .
```

## Cross-Platform Compilation

```bash
# All platforms at once
make build-all

# Individual targets
make build-windows
make build-linux
make build-darwin
make build-darwin-arm64
```

## Versioning

Versions follow the format `YYYYmmdd.HHMM.<commit-id>` and are embedded at build time via ldflags. Use `make build` to automatically set the version. The `--version` flag displays the embedded version.

## Key Dependencies

- `github.com/masterzen/winrm` - WinRM client with Kerberos support
- `github.com/jcmturner/gokrb5/v8` - Kerberos authentication
- `golang.org/x/term` - Secure password input and terminal detection
- `gopkg.in/yaml.v3` - YAML output formatting

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
