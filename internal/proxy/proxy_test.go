package proxy

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// TestProxy_Run tests the basic functionality of the proxy
func TestProxy_Run(t *testing.T) {
	// Create a mock backend server
	backendListener, err := net.Listen("tcp", "127.0.0.1:9000")
	if err != nil {
		t.Fatalf("Failed to create backend listener: %v", err)
	}
	defer backendListener.Close()

	// Create a channel to receive data on backend
	backendChan := make(chan string)
	go func() {
		conn, err := backendListener.Accept()
		if err != nil {
			t.Errorf("Backend accept error: %v", err)
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Errorf("Backend read error: %v", err)
			return
		}
		backendChan <- string(buf[:n])

		// Send response back
		_, err = conn.Write([]byte("response"))
		if err != nil {
			t.Errorf("Backend write error: %v", err)
			return
		}
	}()

	// Create and start proxy
	proxy := &Proxy{
		ListenAddr:  "127.0.0.1:8080",
		BackendAddr: backendListener.Addr().String(),
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	wg.Add(1)
	go func() {
		if err := proxy.Run(ctx, &wg); err != nil {
			t.Errorf("Proxy run error: %v", err)
		}
	}()

	// Wait for proxy to start
	time.Sleep(100 * time.Millisecond)

	// Connect to proxy
	conn, err := net.Dial("tcp", proxy.ListenAddr)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Send test data
	testData := "test message"
	_, err = conn.Write([]byte(testData))
	if err != nil {
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	// Check if backend received the data
	select {
	case received := <-backendChan:
		if received != testData {
			t.Errorf("Backend received wrong data. Got %q, want %q", received, testData)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for backend to receive data")
	}

	// Read response from proxy
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read from proxy: %v", err)
	}

	response := string(buf[:n])
	expectedResponse := "response"
	if response != expectedResponse {
		t.Errorf("Got wrong response from proxy. Got %q, want %q", response, expectedResponse)
	}
}

// TestProxy_ConnectionRefused tests proxy behavior when backend is unavailable
func TestProxy_ConnectionRefused(t *testing.T) {
	proxy := &Proxy{
		ListenAddr:  "127.0.0.1:8080",
		BackendAddr: "127.0.0.1:44444", // Using an unlikely to be used port
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	wg.Add(1)
	go func() {
		if err := proxy.Run(ctx, &wg); err != nil {
			t.Errorf("Proxy run error: %v", err)
		}
	}()

	// Wait for proxy to start
	time.Sleep(100 * time.Millisecond)

	// Try to connect and send data
	conn, err := net.Dial("tcp", proxy.ListenAddr)
	if err != nil {
		t.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer conn.Close()

	// Write should succeed but read should fail as backend is not available
	_, err = conn.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Failed to write to proxy: %v", err)
	}

	// Read should fail or return no data
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		t.Error("Expected read to fail or return no data when backend is unavailable")
	}
}

// TestProxy_Shutdown tests graceful shutdown of the proxy
func TestProxy_Shutdown(t *testing.T) {
	proxy := &Proxy{
		ListenAddr:  "127.0.0.1:8080",
		BackendAddr: "127.0.0.1:44444",
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())

	wg.Add(1)
	go func() {
		if err := proxy.Run(ctx, &wg); err != nil {
			t.Errorf("Proxy run error: %v", err)
		}
	}()

	// Wait for proxy to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context to initiate shutdown
	cancel()

	// Wait for shutdown with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Shutdown completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for proxy shutdown")
	}
}
