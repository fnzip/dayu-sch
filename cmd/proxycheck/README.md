# Proxy Check CLI

A CLI application that monitors blocked proxies and automatically unblocks them when their IP changes or when they become accessible again.

## Features

- Fetches blocked proxies from yarun API (limit 32 by default)
- Checks if proxy IP has changed using `https://api.ipify.org/?format=json`
- If IP changed: automatically unblocks the proxy with new IP
- If IP same: tests accessibility on multiple URLs:
  - `https://jktjkt48.com`
  - `https://idrok5.com`
  - `https://idrgamerp.com`
  - `https://test.1gvdjbxcw.com`
- Detects blocking by checking for:
  - HTTP status code != 200
  - Response body containing "errorOccurPath"
- Updates last check time for still-blocked proxies
- Runs continuously with configurable interval (default 5s)

## Usage

```bash
./proxycheck-cli -yarun-url <API_URL> -yarun-token <TOKEN> [options]
```

### Required Flags

- `-yarun-url`: Base URL of the yarun API
- `-yarun-token`: Authentication token for yarun API

### Optional Flags

- `-interval`: Check interval duration (default: 5s)
- `-limit`: Maximum number of blocked proxies to check per cycle (default: 32)

### Example

```bash
./proxycheck-cli -yarun-url "https://api.yarun.com" -yarun-token "your-token-here" -interval 10s -limit 50
```

## Build

```bash
go build -o proxycheck-cli ./cmd/proxycheck
```

## Process Flow

1. Fetch blocked proxies from yarun API
2. For each proxy:
   - Check current IP through the proxy
   - If IP changed → unblock with new IP
   - If IP same → test accessibility on test URLs
   - If accessible → unblock proxy
   - If still blocked → update last check time
3. Sleep for configured interval
4. Repeat

## Graceful Shutdown

The application handles SIGINT and SIGTERM signals for graceful shutdown.
