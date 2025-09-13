# BatchProxy CLI

A command-line tool for batch processing with proxy support using DataImpulse proxy service.

## Usage

### Using Environment Variables (Legacy)

```bash
export CFBATCH_URL="https://your-cfbatch-api-url.com"
export CFBATCH_TOKEN="your-cfbatch-token"
export PROXY_USERNAME="your-dataimpulse-username"
export PROXY_PASSWORD="your-dataimpulse-password"

./batchproxy-cli -c 10 -b 5 -d 2
```

### Using YAML Config File (Recommended)

1. Create a config file (e.g., `config.yaml`):

```yaml
base_url: "https://your-cfbatch-api-url.com"
token: "your-cfbatch-token"
proxy_username: "your-dataimpulse-username"
proxy_password: "your-dataimpulse-password"
```

2. Run with config file:

```bash
./batchproxy-cli -i config.yaml -c 10 -b 5 -d 2
```

## Command Line Options

- `-i <file>`: Input YAML config file (if not provided, falls back to environment variables)
-  `-c <number>`: Number of concurrent workers (default: 10)
- `-b <number>`: Batch limit (default: 10)
- `-d <number>`: Delay between rounds in seconds (default: 1)

## Examples

```bash
# Use config file with default settings
./batchproxy-cli -i config.yaml

# Use config file with 20 concurrent workers, batch size 8, 3 second delay
./batchproxy-cli -i config.yaml -c 20 -b 8 -d 3

# Fallback to environment variables
./batchproxy-cli -c 5 -b 2
```

## Build

```bash
go build -o batchproxy-cli cmd/batchproxy/main.go
```