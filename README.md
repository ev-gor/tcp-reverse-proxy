# TCP Reverse Proxy

A lightweight, high-performance TCP reverse proxy written in Go. This proxy forwards TCP connections from clients to a specified backend server, making it useful for load balancing, port forwarding, and basic network traffic management.

## Features

- Simple TCP connection forwarding
- Graceful shutdown on signal interruption
- Bidirectional data transfer with proper cleanup
- Configurable listen and backend addresses
- Optional TLS support for secure connections
- Multiple configuration methods (flags, environment variables, JSON file)
- Proper connection management and error handling
- Low memory footprint

## Installation

### Prerequisites

- Go 1.24 or higher

### Using `go install`

```bash
go install github.com/ev-gor/tcp-reverse-proxy/cmd/proxy@latest
```

### Building from source

```bash
# Clone the repository
git clone https://github.com/ev-gor/tcp-reverse-proxy.git
cd tcp-reverse-proxy

# Build the application
go build -o tcp-proxy ./cmd/proxy

# Optional: install to your GOPATH/bin
go install ./cmd/proxy
```

## Configuration

The proxy can be configured using various methods:

### Default Configuration

The default configuration is:

```
Listen Address: 127.0.0.1:8080
Backend Address: 127.0.0.1:9000
Buffer Size: 32 KB
TLS Enabled: false
```

### Command-line Flags
Call `proxy.WithFlags` function to use command-line flags.
```
Usage: tcp-proxy [flags]

Flags:
  -listen string
        Address on which the proxy listens (default "127.0.0.1:8080")
  -backend string
        Address of the backend server (default "127.0.0.1:9000")
  -buffer-size int
        Buffer size for data transfer in KB (default 32)
  -tls-enabled
        Enable TLS (default false)
  -cert-file-path string
        Path to TLS certificate file (absolute path required)
  -key-file-path string
        Path to TLS key file (absolute path required)
```

### Environment Variables

You can configure the proxy using environment variables with a custom prefix (e.g., "PROXY"). Pass the prefix as an argument to `proxy.FromEnv` function:

```bash
export PROXY_LISTEN_ADDR=0.0.0.0:8443
export PROXY_BACKEND_ADDR=192.168.1.100:5432
export PROXY_BUFFER_SIZE=64
export PROXY_TLS_ENABLED=true
export PROXY_CERT_FILE_PATH=/absolute/path/to/cert.pem
export PROXY_KEY_FILE_PATH=/absolute/path/to/key.pem
```

### JSON Configuration File

You can provide a JSON configuration file (absolute path required) as an argument to `proxy.WithConfigFile` function:

```json
{
  "listen_addr": "0.0.0.0:8443",
  "backend_addr": "192.168.1.100:5432",
  "buffer_size": 64,
  "tls_enabled": true,
  "cert_file_path": "/absolute/path/to/cert.pem",
  "key_file_path": "/absolute/path/to/key.pem"
}
```

### Programmatic Configuration

When using the proxy as a library, you can configure it using functional options:

```go
proxy, err := proxy.CreateProxy(
    proxy.WithListenAddr("0.0.0.0:8443"),
    proxy.WithBackendAddr("192.168.1.100:5432"),
    proxy.WithBufferSize(64),
    proxy.WithTlsEnabled(true),
    proxy.WithCertFilePath("/absolute/path/to/cert.pem"),
    proxy.WithKeyFilePath("/absolute/path/to/key.pem"),
)
```

You can also load configuration from different sources:

```go
// From environment variables
proxy, err := proxy.CreateProxy(proxy.FromEnv("<your_prefix>"))

// From a JSON file (absolute path required)
proxy, err := proxy.CreateProxy(proxy.WithConfigFile("/absolute/path/to/config.json"))

// From command-line flags
proxy, err := proxy.CreateProxy(proxy.WithFlags())
```

## Usage

### Basic Example

```bash
# Start the proxy listening on port 8080 and forwarding to a backend on port 9000
tcp-proxy -listen localhost:8080 -backend localhost:9000
```

### Secure TLS Example

```bash
# Start the proxy with TLS enabled (requires certificate and key files)
tcp-proxy -listen 0.0.0.0:8443 -backend localhost:9000 \
  -tls-enabled \
  -cert-file-path="/absolute/path/to/cert.pem" \
  -key-file-path="/absolute/path/to/key.pem"
```

### Custom Port Example

```bash
# Listen on all interfaces on port 8888 and forward to 192.168.1.100:5432
tcp-proxy -listen 0.0.0.0:8888 -backend 192.168.1.100:5432
```

## TLS Support

The proxy supports TLS for securing connections. **TLS is disabled by default**.

### Enabling TLS

To enable TLS, you need to:

1. Set the `tls-enabled` flag to `true` (or use any other config option)
2. Provide valid certificate and private key files (absolute paths required)

```bash
tcp-proxy -tls-enabled -cert-file-path="/absolute/path/to/cert.pem" -key-file-path="/absolute/path/to/key.pem" -listen 0.0.0.0:8443 -backend localhost:9000
```

### Generating a Self-Signed Certificate for Testing

For testing purposes, you can generate a self-signed certificate using OpenSSL:

```bash
# Create directory for certificates
mkdir -p /path/to/certs
cd /path/to/certs

# Generate private key
openssl genrsa -out key.pem 2048

# Generate self-signed certificate (valid for 365 days)
openssl req -new -x509 -key key.pem -out cert.pem -days 365 -subj "/CN=localhost"

# Verify the certificate
openssl x509 -in cert.pem -text -noout
```

Then use the generated files with the proxy:

```bash
tcp-proxy -tls-enabled -cert-file-path="/path/to/certs/cert.pem" -key-file-path="/path/to/certs/key.pem" -listen 0.0.0.0:8443 -backend localhost:9000
```

**Important**: Always use absolute paths for certificate and key files to avoid runtime errors.

## Example Scenarios

### Database Connection Proxy

Suppose you have a PostgreSQL database server running on a private network at `10.0.0.5:5432` and you want to access it from your local machine:

```bash
# On a server with access to both networks
tcp-proxy -listen 0.0.0.0:5432 -backend 10.0.0.5:5432
```

Then connect your PostgreSQL client to the proxy server's address.

### Secure Database Connection Proxy

To encrypt database connections over untrusted networks:

```bash
# Generate certificates (as shown in the TLS section)
# Then start the proxy with TLS enabled
tcp-proxy -tls-enabled \
  -cert-file-path="/absolute/path/to/cert.pem" \
  -key-file-path="/absolute/path/to/key.pem" \
  -listen 0.0.0.0:5432 \
  -backend 10.0.0.5:5432
```

Then configure your PostgreSQL client to use SSL mode when connecting to the proxy.

### Simple Load Testing Setup

```bash
# Terminal 1: Start a simple echo server on port 9000
nc -l 9000 -k

# Terminal 2: Start the TCP proxy
tcp-proxy -listen localhost:8080 -backend localhost:9000

# Terminal 3: Connect to the proxy with a client
nc localhost 8080
```

Now you can type messages in Terminal 3, and they will be forwarded through the proxy to the echo server and back.

## Error Handling

The proxy handles various error conditions gracefully:

| Error Type                     | Behavior |
|--------------------------------|----------|
| Listener startup failure       | Logs error and exits |
| Custom config errors           | Logs error and exits |
| TLS certificate errors         | Logs error and exits |
| TLS configuration errors       | Logs error and exits |
| Backend connection failure     | Logs error and closes client connection |
| Client read/write errors       | Logs error, closes affected connection |
| Backend read/write errors      | Logs error, closes affected connection |
| Graceful shutdown (SIGINT/SIGTERM) | Stops accepting new connections, completes existing transfers, then exits |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the terms of the license included in the repository.
