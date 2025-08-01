<p align="center">
  <a href="https://github.com/Skaronator/lancache-dns-sync/actions/workflows/build.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/Skaronator/lancache-dns-sync/build.yml?branch=main&label=CI%2FCD&logo=github" alt="CI/CD">
  </a>
  <a href="https://github.com/Skaronator/lancache-dns-sync/blob/main/go.mod">
    <img src="https://img.shields.io/github/go-mod/go-version/Skaronator/lancache-dns-sync?logo=go" alt="Go Version">
  </a>
  <a href="https://github.com/Skaronator/lancache-dns-sync/actions/workflows/codeql.yml">
    <img src="https://img.shields.io/github/actions/workflow/status/Skaronator/lancache-dns-sync/build.yml?branch=main&label=Security%20Scan" alt="Security Scan">
  </a>
  <a href="https://github.com/Skaronator/lancache-dns-sync/releases/latest">
    <img src="https://img.shields.io/github/v/release/Skaronator/lancache-dns-sync?sort=semver" alt="Latest Release">
  </a>
  <a href="https://github.com/Skaronator/lancache-dns-sync/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/License-AGLP-yellow.svg" alt="License: AGLP">
  </a>
</p>

# Lancache DNS Sync

Lancache DNS Sync is a synchronization tool designed to update DNS entries for specific services for the [LanCache server](https://github.com/lancachenet/monolithic) as rewrites to the [AdGuardHome DNS server](https://github.com/AdguardTeam/AdGuardHome).

It serves users who already have a running local DNS server (AdGuard Home) in their LAN and wish to avoid replacing it with the lancache-dns container.
This project simplifies the integration of lancache server benefits while keeping your existing AdGuard Home setup.

## Installation and Configuration

### Requirements

- Docker installed on your system
- An existing [AdGuard Home](https://github.com/AdguardTeam/AdGuardHome) setup within your LAN
- An existing [LanCache](https://lancache.net) setup within your lan

### Setup

To start using Lancache DNS Sync, use the following docker-compose configuration:

#### Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  lancache-dns-sync:
    image: ghcr.io/skaronator/lancache-dns-sync:latest
    container_name: lancache-dns-sync
    restart: unless-stopped
    environment:
      ADGUARD_USERNAME: admin
      ADGUARD_PASSWORD: password
      LANCACHE_SERVER: 192.168.1.100
      ADGUARD_API: http://adguard.local:3000
      SYNC_INTERVAL: 24h
      SERVICE_NAMES: '*'

  # Copied from: https://github.com/lancachenet/docker-compose/tree/master
  monolithic:
    image: lancachenet/monolithic:latest
    restart: unless-stopped
    ports:
      - 80:80/tcp
      - 443:443/tcp
    environment:
      LANCACHE_IP: 192.168.1.100
      CACHE_MAX_AGE: 365d
      CACHE_INDEX_SIZE: 500m
      MIN_FREE_DISK: 100g
      CACHE_DISK_SIZE: 1000g
    volumes:
      - ./cache:/data/cache
      - ./logs:/data/logs
```

Then run:
```bash
docker-compose up -d
```

#### Environment Variables

| Variable         | Description                                    | Required | Default | Example                                                                      |
|------------------|------------------------------------------------|----------|---------|------------------------------------------------------------------------------|
| ADGUARD_USERNAME | Username for AdGuard Home                      | Yes      |         | `ADGUARD_USERNAME=admin`                                                     |
| ADGUARD_PASSWORD | Password for AdGuard Home                      | Yes      |         | `ADGUARD_PASSWORD=admin`                                                     |
| LANCACHE_SERVER  | IP address of your lancache server             | Yes      |         | `LANCACHE_SERVER=192.168.1.1`                                                |
| ADGUARD_API      | API URL for AdGuard Home                       | Yes      |         | `ADGUARD_API=http://fw.home:8080`                                            |
| SYNC_INTERVAL    | Duration between syncs (Go duration format)   | No       | `24h`   | `SYNC_INTERVAL="1h"` or `SYNC_INTERVAL="30m"` or `SYNC_INTERVAL="2h30m"`     |
| RUN_ONCE         | Run sync once and exit                         | No       | `false` | `RUN_ONCE="true"` or `RUN_ONCE="1"` or `RUN_ONCE="yes"`                      |
| SERVICE_NAMES    | Services to sync DNS entries for               | Yes      |         | `SERVICE_NAMES='*'` or `SERVICE_NAMES='wsus,epicgames,steam'`                |

*Note: Use `SERVICE_NAMES='*'` to sync all services, or specify comma-separated service names.

### Running without Docker

You can also run the application directly by downloading the pre-built binary from GitHub releases:

#### Download and Run Binary

```bash
# Download latest release for Linux (replace with your OS/architecture)
curl -L https://github.com/Skaronator/lancache-dns-sync/releases/latest/download/lancache-dns-sync_Linux_x86_64.tar.gz -o lancache-dns-sync.tar.gz

# Extract the binary
tar -xzf lancache-dns-sync.tar.gz

# Make it executable
chmod +x lancache-dns-sync

# Set environment variables
export ADGUARD_USERNAME="admin"
export ADGUARD_PASSWORD="password"
export LANCACHE_SERVER="192.168.1.100"
export ADGUARD_API="http://adguard.local:3000"
export SERVICE_NAMES="steam,epicgames"
export SYNC_INTERVAL="2h"  # Optional: sync every 2 hours (default is 24h)

# Run the application
./lancache-dns-sync

# Or run once and exit
./lancache-dns-sync -once
# Or using environment variable
RUN_ONCE=true ./lancache-dns-sync
```

### How It Works

Upon configuring and initiating the container with the required environment variables, Lancache DNS Sync will automatically sync DNS entries for the designated services to your AdGuard Home installation. This process is governed by the `SYNC_INTERVAL` setting, allowing for periodic updates without manual intervention.

The application downloads domain files concurrently for better performance and uses minimal system resources. It runs as a native daemon with built-in duration-based scheduling.

## Contributing

We welcome contributions! For enhancements or fixes, please submit an issue or pull request on GitHub. Your contributions help improve Lancache DNS Sync for everyone.

## License

This project is available under the [MIT License](LICENSE). You are free to fork, modify, and use it in any way you see fit.
