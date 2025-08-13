package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// TestReadAndWrite tests the readAndWrite function using net.Pipe
func TestReadAndWrite(t *testing.T) {
	t.Run("successful data transfer", func(t *testing.T) {
		// Create pipes for testing
		clientRead, clientWrite := net.Pipe()
		backendRead, backendWrite := net.Pipe()

		defer clientRead.Close()
		defer clientWrite.Close()
		defer backendRead.Close()
		defer backendWrite.Close()

		// Setup context and wait group
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Test data
		testData := []byte("Hello, World!")

		// Start readAndWrite goroutine
		wg.Add(1)
		go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

		// Write test data to client
		go func() {
			clientWrite.Write(testData)
			clientWrite.Close()
		}()

		// Read from backend
		result := make([]byte, len(testData))
		n, err := io.ReadFull(backendRead, result)
		if err != nil {
			t.Fatalf("Failed to read from backend: %v", err)
		}

		if n != len(testData) {
			t.Fatalf("Expected %d bytes, got %d", len(testData), n)
		}

		if !bytes.Equal(result, testData) {
			t.Fatalf("Expected %s, got %s", testData, result)
		}

		// Wait for goroutine to finish
		wg.Wait()
	})

	t.Run("context cancellation", func(t *testing.T) {
		clientRead, clientWrite := net.Pipe()
		backendRead, backendWrite := net.Pipe()

		defer clientRead.Close()
		defer clientWrite.Close()
		defer backendRead.Close()
		defer backendWrite.Close()

		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start readAndWrite goroutine
		wg.Add(1)
		go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

		// Cancel context immediately
		cancel()

		// Wait for goroutine to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed - goroutine finished
		case <-time.After(2 * time.Second):
			t.Fatal("readAndWrite didn't finish after context cancellation")
		}
	})

	t.Run("read error handling", func(t *testing.T) {
		clientRead, clientWrite := net.Pipe()
		backendRead, backendWrite := net.Pipe()

		defer backendRead.Close()
		defer backendWrite.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start readAndWrite goroutine
		wg.Add(1)
		go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

		// Close the read connection to trigger an error
		clientRead.Close()
		clientWrite.Close()

		// Wait for goroutine to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed - goroutine handled error and finished
		case <-time.After(2 * time.Second):
			t.Fatal("readAndWrite didn't finish after read error")
		}
	})

	t.Run("write error handling", func(t *testing.T) {
		clientRead, clientWrite := net.Pipe()
		backendRead, backendWrite := net.Pipe()

		defer clientRead.Close()
		defer clientWrite.Close()
		defer backendRead.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start readAndWrite goroutine
		wg.Add(1)
		go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

		// Close the write connection to trigger an error
		backendWrite.Close()

		// Write some data to trigger the write error
		go func() {
			clientWrite.Write([]byte("test data"))
			clientWrite.Close()
		}()

		// Wait for goroutine to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed - goroutine handled error and finished
		case <-time.After(2 * time.Second):
			t.Fatal("readAndWrite didn't finish after write error")
		}
	})

	t.Run("large data transfer", func(t *testing.T) {
		clientRead, clientWrite := net.Pipe()
		backendRead, backendWrite := net.Pipe()

		defer clientRead.Close()
		defer clientWrite.Close()
		defer backendRead.Close()
		defer backendWrite.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 1024) // Smaller buffer to test multiple writes
			},
		}

		// Create large test data (larger than buffer)
		testData := bytes.Repeat([]byte("A"), 5000)

		// Start readAndWrite goroutine
		wg.Add(1)
		go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

		// Write test data to client
		go func() {
			defer clientWrite.Close()
			clientWrite.Write(testData)
		}()

		// Read all data from backend
		var result bytes.Buffer
		buffer := make([]byte, 1024)
		for {
			n, err := backendRead.Read(buffer)
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("Failed to read from backend: %v", err)
			}
			result.Write(buffer[:n])
		}

		if !bytes.Equal(result.Bytes(), testData) {
			t.Fatalf("Data mismatch. Expected %d bytes, got %d bytes", len(testData), result.Len())
		}

		// Wait for goroutine to finish
		wg.Wait()
	})
}

// TestHandle tests the handle function
//
//nolint:gocyclo
func TestHandle(t *testing.T) {
	t.Run("successful proxy connection", func(t *testing.T) {
		// Create a mock backend server
		backendListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to create backend listener: %v", err)
		}
		defer backendListener.Close()

		backendAddr := backendListener.Addr().String()

		// Create ready channel to coordinate with backend
		backendReady := make(chan struct{})

		// Start mock backend server
		go func() {
			conn, err := backendListener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()

			// Signal that backend is ready
			close(backendReady)

			// Echo server - read and write back
			buffer := make([]byte, 1024)
			n, err := conn.Read(buffer)
			if err != nil {
				return
			}
			conn.Write(buffer[:n])
		}()

		// Create client connection using pipes
		clientConn, proxyConn := net.Pipe()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start handle function
		wg.Add(1)
		go handle(ctx, proxyConn, backendAddr, &wg, bufPool)

		// Wait for backend to be ready before proceeding
		select {
		case <-backendReady:
			// Backend is ready
		case <-time.After(1 * time.Second):
			t.Fatal("Timeout waiting for backend to be ready")
		}

		// Give the handle function time to establish connections
		time.Sleep(100 * time.Millisecond)

		// Test data exchange
		testData := []byte("Hello Backend!")

		// Write to client in a separate goroutine but don't close it immediately
		writeDone := make(chan struct{})
		go func() {
			defer close(writeDone)
			_, err := clientConn.Write(testData)
			if err != nil {
				t.Errorf("Failed to write to client: %v", err)
				return
			}
		}()

		// Wait for write to complete
		<-writeDone

		// Read response from client
		response := make([]byte, len(testData))

		// Set a read deadline to avoid blocking forever
		clientConn.SetDeadline(time.Now().Add(2 * time.Second))

		n, err := io.ReadFull(clientConn, response)
		if err != nil {
			t.Fatalf("Failed to read response: %v", err)
		}

		if n != len(testData) {
			t.Fatalf("Expected %d bytes, got %d", len(testData), n)
		}

		if !bytes.Equal(response, testData) {
			t.Fatalf("Expected %s, got %s", testData, response)
		}

		// Close connections before cancelling context
		clientConn.Close()
		proxyConn.Close()

		// Cancel context and wait for cleanup
		cancel()

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed
		case <-time.After(3 * time.Second):
			t.Fatal("handle function didn't finish after context cancellation")
		}
	})

	t.Run("backend connection failure", func(t *testing.T) {
		// Use a backend address that will definitely refuse connection
		// Find an unused port
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to find unused port: %v", err)
		}
		unusedPort := l.Addr().(*net.TCPAddr).Port
		l.Close() // Close the listener to free the port

		backendAddr := fmt.Sprintf("127.0.0.1:%d", unusedPort)

		clientConn, proxyConn := net.Pipe()
		defer clientConn.Close()
		defer proxyConn.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start handle function
		wg.Add(1)
		go handle(ctx, proxyConn, backendAddr, &wg, bufPool)

		// Wait for handle to finish (should finish quickly due to connection error)
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed - handle finished due to connection error
		case <-time.After(10 * time.Second):
			t.Fatal("handle function didn't finish after backend connection failure")
		}
	})

	t.Run("context cancellation during handle", func(t *testing.T) {
		// Create a backend that accepts but doesn't respond
		backendListener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("Failed to create backend listener: %v", err)
		}
		defer backendListener.Close()

		backendAddr := backendListener.Addr().String()

		// Start backend that accepts but blocks
		go func() {
			conn, err := backendListener.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			// Just block without doing anything
			select {}
		}()

		clientConn, proxyConn := net.Pipe()
		defer clientConn.Close()
		defer proxyConn.Close()

		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup
		bufPool := &sync.Pool{
			New: func() any {
				return make([]byte, 4096)
			},
		}

		// Start handle function
		wg.Add(1)
		go handle(ctx, proxyConn, backendAddr, &wg, bufPool)

		// Wait a bit for connections to establish
		time.Sleep(100 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for handle to finish
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Test passed - handle finished due to context cancellation
		case <-time.After(3 * time.Second):
			t.Fatal("handle function didn't finish after context cancellation")
		}
	})
}

// Benchmark for readAndWrite function
func BenchmarkReadAndWrite(b *testing.B) {
	clientRead, clientWrite := net.Pipe()
	backendRead, backendWrite := net.Pipe()

	defer clientRead.Close()
	defer clientWrite.Close()
	defer backendRead.Close()
	defer backendWrite.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	bufPool := &sync.Pool{
		New: func() any {
			return make([]byte, 4096)
		},
	}

	// Start readAndWrite goroutine
	wg.Add(1)
	go readAndWrite(ctx, clientRead, backendWrite, cancel, &wg, bufPool)

	testData := bytes.Repeat([]byte("benchmark test data"), 100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Use a WaitGroup to ensure write completes
		var writeWg sync.WaitGroup
		writeWg.Add(1)

		// Write data with guaranteed completion notification
		go func() {
			defer writeWg.Done()
			_, err := clientWrite.Write(testData)
			if err != nil {
				b.Errorf("Write error: %v", err)
			}
		}()

		// Read data
		buffer := make([]byte, len(testData))
		_, err := io.ReadFull(backendRead, buffer)
		if err != nil {
			b.Fatal(err)
		}

		// Ensure write has completed before next iteration
		writeWg.Wait()
	}

	// Only cancel context and wait for cleanup after all iterations are done
	cancel()
	wg.Wait()
}
