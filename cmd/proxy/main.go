package main

import (
	// Standard library imports
	"context"   // For context management and cancellation
	"log"       // For logging messages
	"os"        // For OS functionality like signals
	"os/signal" // For signal handling
	"sync"      // For synchronization primitives
	"syscall"   // For system call constants

	// Project imports
	"github.com/ev-gor/tcp-reverse-proxy/internal/proxy" // Proxy implementation
)

func main() {
	// Create wait group to track all goroutines
	var wg sync.WaitGroup

	// Setup context that will be cancelled on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop() // Ensure context cancellation function is called
	// Initialize the proxy server with configured addresses
	proxyServer, proxyError := proxy.CreateProxy()
	if proxyError != nil {
		//nolint:gocritic
		log.Fatalf("Failed to create proxy server: %v", proxyError)
	}
	// Add to wait group before starting the goroutine
	wg.Add(1)

	// Start the proxy server in a separate goroutine
	go func() {
		// Run the proxy until context is cancelled or error occurs
		if err := proxyServer.Run(ctx, &wg); err != nil {
			log.Printf("Proxy server error: %v", err)
			stop() // Cancel context on error
		}
	}()
	// Block until context is cancelled (by signal or error)
	<-ctx.Done()

	// Wait for all goroutines to complete before exiting
	wg.Wait()
}
