# dayu-sch

## batch
```bash
go build -o batch-cli cmd/cfbatch/main.go
```

## batchproxy
```bash
go build -o batchproxy-cli cmd/batchproxy/main.go
```

The batchproxy application now integrates with yarun API for intelligent proxy management:

### Features:
- **Intelligent Proxy Management**: Uses yarun API to get available proxies with round-robin functionality
- **Automatic Proxy Blocking**: Blocks failed proxies for 1 hour cooldown period
- **Thread-safe Operations**: Proper concurrency control and error handling

### Configuration:
You can configure batchproxy using either a YAML file or environment variables:

#### YAML Configuration (recommended):
```yaml
base_url: "https://your-cfbatch-api.com"
token: "your-cfbatch-token"
proxy_username: "your-proxy-username"
proxy_password: "your-proxy-password"
yarun_base_url: "https://your-yarun-api.com"
yarun_token: "your-yarun-api-token"
```

#### Environment Variables:
```bash
export CFBATCH_URL="https://your-cfbatch-api.com"
export CFBATCH_TOKEN="your-cfbatch-token"
export PROXY_USERNAME="your-proxy-username"
export PROXY_PASSWORD="your-proxy-password"
export YARUN_BASE_URL="https://your-yarun-api.com"
export YARUN_TOKEN="your-yarun-api-token"
```

### Usage:
```bash
# Using config file
./batchproxy-cli -max-concurrent 10 -batch-limit 50 -delay 5 -config pxy.yml

# Using environment variables
./batchproxy-cli -max-concurrent 10 -batch-limit 50 -delay 5
```

### Yarun API Integration:
- **GET /proxy?limit={n}**: Fetches available proxies starting from port 10000 with round-robin
- **POST /proxy/blocked**: Blocks proxies that fail for 1-hour cooldown

## stat
```bash
go build -o stat-cli cmd/schstat/main.go
```