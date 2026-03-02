# Host Updater for AD DNS

A cross-platform Go tool that automatically updates Windows Active Directory DNS records based on the local machine's IP addresses. Perfect for dynamic environments where hosts need to keep their DNS records current.

## Features

- **Automatic IP Detection** - Discovers local IPv4 and IPv6 addresses from network interfaces
- **DNS Record Verification** - Queries AD DNS servers to check existing A and AAAA records
- **Smart Updates** - Only updates records when they don't match the current IP
- **Kerberos Authentication** - Secure authentication using Kerberos over HTTPS
- **Stale Record Cleanup** - Removes AAAA records that no longer exist on the host
- **Cross-Platform** - Compiles for Windows, Linux, and macOS (Intel & Apple Silicon)
- **Multiple Output Formats** - Pretty interactive CLI, JSON, or YAML output
- **Self-Update** - Optional `--auto-update` flag to download and install new releases from GitHub with SHA256 verification

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/fahadysf/host-updater-for-ad-dns/releases) page.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/fahadysf/host-updater-for-ad-dns.git
cd host-updater-for-ad-dns

# Build with version embedding (recommended)
make build

# Or manually
go build -o dns-updater .
```

### Cross-Platform Compilation

```bash
# All platforms at once
make build-all

# Or individually
make build-windows    # Windows amd64
make build-linux      # Linux amd64
make build-darwin     # macOS Intel
make build-darwin-arm64  # macOS Apple Silicon
```

## Usage

```bash
./dns-updater \
  --domain "example.com" \
  --nameservers "192.168.1.10,192.168.1.11" \
  --ad-user "user@example.com" \
  --ad-password "password" \
  --update-dns
```

### Command Line Options

| Flag | Description | Required |
|------|-------------|----------|
| `--domain` | AD domain name (e.g., example.com) | Yes |
| `--nameservers` | Comma-separated list of DNS server IPs | Yes |
| `--hostname` | Hostname to update (defaults to local hostname) | No |
| `--update-dns` | Enable DNS record updates | No |
| `--ad-user` | AD username in UPN format (user@domain or DOMAIN\user) | For updates |
| `--ad-password` | AD password (prompted securely if not provided) | For updates |
| `--ip` | Manual IPv4 address (skips IPv4 auto-detection) | No |
| `--ipv6` | Comma-separated manual IPv6 addresses (skips IPv6 auto-detection) | No |
| `-o` | Output format: `pretty` (default), `json`, or `yaml` | No |
| `--auto-update` | Check for and install updates from GitHub after execution | No |
| `--debug` | Enable verbose debug logging | No |
| `--version` | Show version and exit | No |

### Output Formats

**Pretty** (default) — Interactive CLI output with Unicode progress indicators and spinners:

```
DNS Updater - workstation1.example.com

✓ Discover local IP addresses found [192.168.1.100, fd00::100]
✓ Check DNS server 192.168.1.10 online
✓ Lookup records on 192.168.1.10 found [A:1 AAAA:1]
✓ Update records on 192.168.1.10 updated [1 records]
```

**JSON** (`-o json`) — Structured output for programmatic consumption:

```json
{
    "hostname": "workstation1",
    "domain": "example.com",
    "fqdn": "workstation1.example.com",
    "source_ips": {
        "ipv4": "192.168.1.100",
        "ipv6": ["fd00::100"]
    },
    "dns_servers_queried": ["192.168.1.10"],
    "results": [
        {
            "server": "192.168.1.10",
            "a_records_found": ["192.168.1.50"],
            "a_record_updates": [
                {
                    "ip": "192.168.1.100",
                    "status": "success",
                    "message": "Created new A record"
                }
            ],
            "aaaa_records_found": ["fd00::100"],
            "aaaa_record_updates": [
                {
                    "ip": "fd00::100",
                    "status": "skipped",
                    "message": "Record already correct."
                }
            ]
        }
    ]
}
```

**YAML** (`-o yaml`) — Structured output for configuration and scripting.

## Server Requirements

Each Windows DNS server must have an HTTPS WinRM listener configured:

```powershell
# Run as Administrator on each DNS server
$cert = New-SelfSignedCertificate -DnsName "dc.example.com" -CertStoreLocation Cert:\LocalMachine\My
winrm create winrm/config/Listener?Address=*+Transport=HTTPS "@{Hostname=`"dc.example.com`"; CertificateThumbprint=`"$($cert.Thumbprint)`"}"
New-NetFirewallRule -DisplayName "WinRM HTTPS" -Direction Inbound -LocalPort 5986 -Protocol TCP -Action Allow
```

## How It Works

1. **IP Discovery** - Scans local network interfaces for IPv4 and IPv6 addresses
2. **DNS Query** - Queries each specified DNS server for existing A/AAAA records
3. **Comparison** - Compares discovered IPs with DNS records
4. **Authentication** - If updates needed, authenticates via Kerberos:
   - Extracts realm from username (user@domain.com → DOMAIN.COM)
   - Creates temporary Kerberos configuration
   - Obtains TGT from KDC
   - Gets service ticket for HTTP/hostname SPN
5. **Update** - Executes PowerShell commands via WinRM over HTTPS to add missing records and remove stale ones

## Self-Update

When `--auto-update` is passed, the tool checks for newer releases after normal execution completes. The update process:

1. Queries the GitHub Releases API for the latest version
2. Compares the release timestamp against the current build version
3. Downloads the correct binary for the current OS/architecture
4. Verifies the SHA256 checksum against the published `checksums.txt`
5. Atomically replaces the running binary (on Windows, renames the old binary to `.old` first)

If any step fails, the original binary is left untouched. Dev builds (`--version` shows `dev`) skip the update check entirely.

```bash
# Check and update after normal operation
./dns-updater --domain example.com --nameservers 192.168.1.10 --auto-update

# Combine with --debug to see update details
./dns-updater --domain example.com --nameservers 192.168.1.10 --auto-update --debug
```

## Architecture

```
main.go           - CLI entry point, argument parsing, orchestration
dns.go            - DNS server liveness checks and record lookups
ip_discovery.go   - Local network interface IP address discovery
winrm.go          - WinRM client with Kerberos/HTTPS authentication
updater.go        - Self-update logic (GitHub releases, SHA256 verification)
output.go         - Interactive progress display and output formatting
logger.go         - Debug logging infrastructure
version.go        - Build version (set via ldflags)
```

## Dependencies

- [github.com/masterzen/winrm](https://github.com/masterzen/winrm) - WinRM client
- [github.com/jcmturner/gokrb5](https://github.com/jcmturner/gokrb5) - Kerberos authentication
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) - Secure password input and terminal detection
- [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3) - YAML output formatting

## Versioning

Versions follow the format `YYYYmmdd.HHMM.<commit-id>` and are embedded at build time via ldflags. Use `make build` to automatically set the version. The `--version` flag displays the embedded version.

## Use Cases

- **DHCP Environments** - Keep DNS records updated when IPs change
- **Laptop Users** - Update DNS when moving between networks
- **DevOps Automation** - Integrate into provisioning scripts
- **Home Labs** - Maintain DNS for homelab machines
- **Scheduled Tasks / Cron** - Run periodically with `-o json` for structured logging

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
