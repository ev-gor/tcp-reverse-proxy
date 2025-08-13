package proxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
)

type ListenerFactory func(config config) (net.Listener, error)

var tcpListenerFactory ListenerFactory = func(config config) (net.Listener, error) {
	l, err := net.Listen("tcp", config.listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen error: %w", err)
	}
	return l, nil
}

var tlsListenerFactory ListenerFactory = func(config config) (net.Listener, error) {
	if config.certFilePath == "" || config.keyFilePath == "" {
		return nil, errors.New("cert file path or key file path is empty")
	}
	cert, err := tls.LoadX509KeyPair(config.certFilePath, config.keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("load x509 key pair: %w", err)
	}
	//nolint:gosec
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	l, err := tls.Listen("tcp", config.listenAddr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("listen error: %w", err)
	}
	return l, nil
}
