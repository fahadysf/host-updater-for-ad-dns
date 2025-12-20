# Host Updater for AD DNS

A cross-platform Go tool that automatically updates Windows Active Directory DNS records based on the local machine's IP addresses. Perfect for dynamic environments where hosts need to keep their DNS records current.

## Features

- **Automatic IP Detection** - Discovers local IPv4 and IPv6 addresses from network interfaces
- **DNS Record Verification** - Queries AD DNS servers to check existing A and AAAA records
- **Smart Updates** - Only updates records when they don't match the current IP
- **Kerberos Authentication** - Secure authentication using Kerberos over HTTPS
- **Cross-Platform** - Compiles for Windows, Linux, and macOS
- **JSON Output** - Machine-readable output for scripting and automation

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/fahadysf/host-updater-for-ad-dns/releases) page.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/fahadysf/host-updater-for-ad-dns.git
cd host-updater-for-ad-dns

# Build
go build -o dns-updater .
```

### Cross-Platform Compilation

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
| `--ad-user` | AD username in UPN format (user@domain) | For updates |
| `--ad-password` | AD password (prompted if not provided) | For updates |
| `--ip` | Manual IPv4 address (skips auto-detection) | No |
| `--debug` | Enable verbose debug logging | No |

### Example Output

```json
{
    "hostname": "workstation1",
    "domain": "example.com",
    "fqdn": "workstation1.example.com",
    "source_ips": {
        "ipv4": "192.168.1.100",
        "ipv6": "fd00::100"
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
            ]
        }
    ]
}
```

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
5. **Update** - Executes PowerShell commands via WinRM over HTTPS to add missing records

## Architecture

```
main.go           - CLI entry point, argument parsing, orchestration
dns.go            - DNS server liveness checks and record lookups
ip_discovery.go   - Local network interface IP address discovery
winrm.go          - WinRM client with Kerberos/HTTPS authentication
logger.go         - Debug logging infrastructure
```

## Dependencies

- [github.com/masterzen/winrm](https://github.com/masterzen/winrm) - WinRM client
- [github.com/jcmturner/gokrb5](https://github.com/jcmturner/gokrb5) - Kerberos authentication
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) - Secure password input

## Use Cases

- **DHCP Environments** - Keep DNS records updated when IPs change
- **Laptop Users** - Update DNS when moving between networks
- **DevOps Automation** - Integrate into provisioning scripts
- **Home Labs** - Maintain DNS for homelab machines

## License

MIT License - See [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
