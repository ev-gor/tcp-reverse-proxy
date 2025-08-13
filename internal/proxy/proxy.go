package proxy

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
)

type Proxy struct {
	config          config
	bufPool         sync.Pool
	listenerFactory ListenerFactory
}

func CreateProxy(options ...Option) (*Proxy, error) {
	cfg := config{
		listenAddr:  listenAddrDefault,
		backendAddr: backendAddrDefault,
		bufferSize:  bufferSizeDefault,
		tlsEnabled:  tlsEnabledDefault,
	}

	for _, opt := range options {
		if err := opt(&cfg); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	factory := tcpListenerFactory
	if cfg.tlsEnabled {
		factory = tlsListenerFactory
	}

	return &Proxy{
		config:          cfg,
		bufPool:         sync.Pool{New: func() any { return make([]byte, 1024*cfg.bufferSize) }},
		listenerFactory: factory,
	}, nil
}

func (p *Proxy) Run(ctx context.Context, wg *sync.WaitGroup) error {
	defer wg.Done()
	listener, listenerErr := p.listenerFactory(p.config)
	if listenerErr != nil {
		return fmt.Errorf("create listener: %w", listenerErr)
	}
	fmt.Printf("Listening on :%v\n", p.config.listenAddr)

	// Setup goroutine to close listener when context is cancelled
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		//nolint:errcheck
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
		go handle(ctx, conn, p.config.backendAddr, wg, &p.bufPool)
	}
}
