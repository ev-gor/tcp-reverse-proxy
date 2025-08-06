package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Proxy struct {
	ListenAddr  string
	BackendAddr string
}

func readAndWrite(ctx context.Context, connToRead net.Conn, connToWrite net.Conn, cancelConn context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create buffer for data transfer (32KB)
	buf := make([]byte, 1024*32)

	// Start goroutine to close connections when context is cancelled
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		connToRead.Close()
		connToWrite.Close()
	}()
	for {
		// Read data from source connection
		n, err := connToRead.Read(buf)
		if err != nil {
			// Handle normal connection closure
			if err == io.EOF {
				log.Printf("%v closed connection.", connToRead.RemoteAddr())

				// Ignore already closed connection errors
			} else if !errors.Is(err, net.ErrClosed) {
				log.Printf("Error reading %v: %v", connToRead.RemoteAddr(), err)
			}
			// Gracefully shutdown write side of a TCP connection if the connection has problems with its read side
			if tcpConn, ok := connToRead.(*net.TCPConn); ok {
				tcpConn.CloseWrite() //nolint:errcheck
			}
			// Signal connection termination and exit
			cancelConn()
			return
		}
		// Track how many bytes have been written
		written := 0
		// Ensure all read bytes are fully written (handle partial writes)
		for written < n {
			newWritten, writeErr := connToWrite.Write(buf[written:n])
			if writeErr != nil {
				log.Printf("write to %v error: %v", connToWrite.RemoteAddr(), writeErr)
				// Shutdown read side of the connection if it has problems with its write side
				if tcpConn, ok := connToRead.(*net.TCPConn); ok {
					tcpConn.CloseRead() //nolint:errcheck
				}
				// Signal connection termination and exit
				cancelConn()
				return
			}
			// Update count of bytes written
			written += newWritten
		}
	}
}

func handle(parentCtx context.Context, client net.Conn, backendAddr string, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create a cancellable context for this connection
	connCtx, cancelConn := context.WithCancel(parentCtx)
	defer cancelConn()
	defer client.Close()

	// Configure connection timeout
	dialer := &net.Dialer{Timeout: 5 * time.Second}

	// Connect to the backend server
	backend, err := dialer.DialContext(connCtx, "tcp", backendAddr)
	if err != nil {
		log.Printf("Error connecting to backend: %s\n", err)
		return
	}
	defer backend.Close()

	// Start bidirectional data transfer
	wg.Add(2)
	// Forward client -> backend
	go readAndWrite(connCtx, client, backend, cancelConn, wg)
	// Forward backend -> client
	go readAndWrite(connCtx, backend, client, cancelConn, wg)

	// Wait for connection to be cancelled
	<-connCtx.Done()
}

func (p *Proxy) Run(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()

	// Start TCP listener on the configured address
	listener, err := net.Listen("tcp", p.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen error: %w", err)
	}
	fmt.Printf("Listening on :%v\n", p.ListenAddr)

	// Setup goroutine to close listener when context is cancelled
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		listener.Close()
	}()

	// Accept and handle incoming connections until context is cancelled
	for {
		conn, err := listener.Accept()
		if err != nil {
			// Listener was closed gracefully (expected during shutdown)
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			// Log other accept errors and continue
			log.Printf("accept error: %v", err)
			continue
		}
		log.Printf("Accepting connection from %v", conn.RemoteAddr())

		// Handle each connection in a separate goroutine
		wg.Add(1)
		go handle(ctx, conn, p.BackendAddr, wg)
	}
}
