package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestCreateProxy(t *testing.T) {
	tests := []struct {
		name    string
		options []Option
		wantErr bool
	}{
		{
			name:    "default configuration",
			options: nil,
			wantErr: false,
		},
		{
			name: "with valid options",
			options: []Option{
				WithListenAddr(":8080"),
				WithBackendAddr("localhost:9090"),
				WithBufferSize(2),
			},
			wantErr: false,
		},
		{
			name: "with invalid option",
			options: []Option{
				func(cfg *config) error {
					return errors.New("invalid option")
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := CreateProxy(tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && proxy == nil {
				t.Error("CreateProxy() returned nil proxy without error")
			}
		})
	}
}

func TestProxy_Run_ListenError_PortInUse(t *testing.T) {
	// First, create a listener to occupy a port
	tempListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create temp listener: %v", err)
	}
	defer tempListener.Close()

	// Get the address that's now in use
	addr := tempListener.Addr().String()

	// Try to create a proxy on the same address
	proxy, err := CreateProxy(WithListenAddr(addr))
	if err != nil {
		t.Fatalf("CreateProxy() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	err = proxy.Run(ctx, &wg)
	if err == nil {
		t.Error("Run() should return listen error for address already in use")
	}

	// Verify the error is a listen error
	if err != nil && !contains(err.Error(), "listen error") {
		t.Errorf("Expected listen error, got: %v", err)
	}

	// Wait for goroutine to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, WaitGroup completed
	case <-time.After(1 * time.Second):
		t.Error("WaitGroup did not complete in time")
	}
}

func TestProxy_Run_GracefulShutdown(t *testing.T) {
	// Create a proxy with a valid address
	proxy, err := CreateProxy(WithListenAddr(":0")) // Use port 0 for auto-assignment
	if err != nil {
		t.Fatalf("CreateProxy() failed: %v", err)
	}

	// Start the proxy in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var runErr error

	wg.Add(1)
	go func() {
		runErr = proxy.Run(ctx, &wg)
	}()

	// Give the proxy time to start listening
	time.Sleep(10 * time.Millisecond)

	// Cancel the context to trigger shutdown
	cancel()

	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Verify that Run() returned without error (graceful shutdown)
		if runErr != nil {
			t.Errorf("Run() should return nil on graceful shutdown, got: %v", runErr)
		}
	case <-time.After(1 * time.Second):
		t.Error("Run() did not complete gracefully within timeout")
	}
}

func TestProxy_Run_BufferPoolInitialization(t *testing.T) {
	bufferSize := 4
	proxy, err := CreateProxy(WithBufferSize(bufferSize))
	if err != nil {
		t.Fatalf("CreateProxy() failed: %v", err)
	}

	// Test that buffer pool is properly initialized
	buf := proxy.bufPool.Get().([]byte)
	expectedSize := 1024 * bufferSize
	if len(buf) != expectedSize {
		t.Errorf("Buffer pool buffer size = %d, expected %d", len(buf), expectedSize)
	}
	proxy.bufPool.Put(&buf)
}

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
	proxy, proxyErr := CreateProxy()
	if proxyErr != nil {
		t.Fatalf("CreateProxy() failed: %v", proxyErr)
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
	conn, err := net.Dial("tcp", proxy.config.listenAddr)
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
	proxy, proxyErr := CreateProxy(WithBackendAddr("127.0.0.1:44444"))
	if proxyErr != nil {
		t.Fatalf("CreateProxy() failed: %v", proxyErr)
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
	conn, err := net.Dial("tcp", proxy.config.listenAddr)
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

func TestProxy_AcceptError(t *testing.T) {
	fmt.Println("TestProxy_AcceptError")
	mockListener := newMockListener(true)
	proxy, err := CreateProxy()
	if err != nil {
		t.Fatalf("CreateProxy() failed: %v", err)
	}
	proxy.listenerFactory = func(config config) (net.Listener, error) {
		return mockListener, nil
	}

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	wg.Add(1)
	go func() {
		if err := proxy.Run(ctx, &wg); err != nil {
			if err.Error() != "mock accept error" {
				t.Errorf("Expected accept error, got: %v", err)
			}
		}
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	wg.Wait()
}

type mockListener struct {
	conns   chan net.Conn
	close   chan struct{}
	isError bool
}

func newMockListener(isError bool) *mockListener {
	return &mockListener{
		conns:   make(chan net.Conn, 1),
		close:   make(chan struct{}),
		isError: isError,
	}
}

func (m *mockListener) Accept() (net.Conn, error) {
	if m.isError {
		m.isError = false
		return nil, errors.New("mock accept error")
	}
	select {
	case c := <-m.conns:
		return c, nil
	case <-m.close:
		return nil, net.ErrClosed
	}
}

func (m *mockListener) Close() error {
	close(m.close)
	return nil
}

func (*mockListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
