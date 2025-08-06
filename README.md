# TCP Reverse Proxy

A lightweight, high-performance TCP reverse proxy written in Go. This proxy forwards TCP connections from clients to a specified backend server, making it useful for load balancing, port forwarding, and basic network traffic management.

## Features

- Simple TCP connection forwarding
- Graceful shutdown on signal interruption
- Bidirectional data transfer with proper cleanup
- Configurable listen and backend addresses
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

## Usage

### Command-line Options

```
Usage: tcp-proxy [flags]

Flags:
  -listen string
        Address on which the proxy listens (default "localhost:8080")
  -backend string
        Address of the backend server (default "localhost:9000")
```

### Basic Example

```bash
# Start the proxy listening on port 8080 and forwarding to a backend on port 9000
tcp-proxy -listen localhost:8080 -backend localhost:9000
```

### Custom Port Example

```bash
# Listen on all interfaces on port 8888 and forward to 192.168.1.100:5432
tcp-proxy -listen 0.0.0.0:8888 -backend 192.168.1.100:5432
```

## Example Scenarios

### Database Connection Proxy

Suppose you have a PostgreSQL database server running on a private network at `10.0.0.5:5432` and you want to access it from your local machine:

```bash
# On a server with access to both networks
tcp-proxy -listen 0.0.0.0:5432 -backend 10.0.0.5:5432
```

Then connect your PostgreSQL client to the proxy server's address.

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

| Error Type | Behavior |
|------------|----------|
| Listener startup failure | Logs error and exits |
| Backend connection failure | Logs error and closes client connection |
| Client read/write errors | Logs error, closes affected connection |
| Backend read/write errors | Logs error, closes affected connection |
| Graceful shutdown (SIGINT/SIGTERM) | Stops accepting new connections, completes existing transfers, then exits |

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the terms of the license included in the repository.
