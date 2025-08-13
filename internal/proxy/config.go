package proxy

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
)

const (
	listenAddrDefault  = "127.0.0.1:8080"
	backendAddrDefault = "127.0.0.1:9000"
	bufferSizeDefault  = 32
	tlsEnabledDefault  = false
)

type Option func(*config) error

type config struct {
	listenAddr   string
	backendAddr  string
	bufferSize   int
	tlsEnabled   bool
	certFilePath string
	keyFilePath  string
}

// ---- Option functions ----

func WithListenAddr(addr string) Option {
	return func(cfg *config) error {
		host, port, err := parseAddress(addr)
		if err != nil {
			return fmt.Errorf("parse address: %w", err)
		}
		cfg.listenAddr = net.JoinHostPort(host, port)
		return nil
	}
}

func WithBackendAddr(addr string) Option {
	return func(cfg *config) error {
		host, port, err := parseAddress(addr)
		if err != nil {
			return fmt.Errorf("parse address: %w", err)
		}
		cfg.backendAddr = net.JoinHostPort(host, port)
		return nil
	}
}

func WithBufferSize(size int) Option {
	return func(cfg *config) error {
		if size <= 0 {
			return errors.New("buffer size must be positive")
		}
		cfg.bufferSize = size
		return nil
	}
}

func WithTlSEnabled(enabled bool) Option {
	return func(cfg *config) error {
		cfg.tlsEnabled = enabled
		return nil
	}
}

func WithCertFilePath(path string) Option {
	return func(cfg *config) error {
		_, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("cert file path: %w", err)
		}
		cfg.certFilePath = path
		return nil
	}
}

func WithKeyFilePath(path string) Option {
	return func(cfg *config) error {
		_, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("key file path: %w", err)
		}
		cfg.keyFilePath = path
		return nil
	}
}

// ---- Config loaders ----

func FromEnv(prefix string) Option {
	return func(c *config) error {
		if v, ok := os.LookupEnv(prefix + "_LISTEN_ADDR"); ok {
			if err := WithListenAddr(v)(c); err != nil {
				return fmt.Errorf("apply option: %w", err)
			}
		}
		if v, ok := os.LookupEnv(prefix + "_BACKEND_ADDR"); ok {
			if err := WithBackendAddr(v)(c); err != nil {
				return fmt.Errorf("apply option: %w", err)
			}
		}
		if v, ok := os.LookupEnv(prefix + "_BUFFER_SIZE"); ok {
			if n, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("buffer size: %w", err)
			} else if n <= 0 {
				return errors.New("buffer size must be positive")
			} else {
				c.bufferSize = n
			}
		}
		if v, ok := os.LookupEnv(prefix + "_TLS_ENABLED"); ok {
			//nolint:errcheck
			WithTlSEnabled(v == "true")(c)
		}
		if v, ok := os.LookupEnv(prefix + "_CERT_FILE_PATH"); ok {
			if err := WithCertFilePath(v)(c); err != nil {
				return fmt.Errorf("apply option: %w", err)
			}
		}
		if v, ok := os.LookupEnv(prefix + "_KEY_FILE_PATH"); ok {
			if err := WithKeyFilePath(v)(c); err != nil {
				return fmt.Errorf("apply option: %w", err)
			}
		}
		return nil
	}
}

func WithConfigJSON(b []byte) Option {
	if len(b) == 0 {
		return func(cfg *config) error { return nil }
	}
	return func(cfg *config) error {
		var raw struct {
			ListenAddr   string `json:"listen_addr"`
			BackendAddr  string `json:"backend_addr"`
			BufferSize   int    `json:"buffer_size"`
			TlSEnabled   bool   `json:"tls_enabled"`
			CertFilePath string `json:"cert_file_path"`
			KeyFilePath  string `json:"key_file_path"`
		}
		if err := json.Unmarshal(b, &raw); err != nil {
			return fmt.Errorf("parse json config: %w", err)
		}
		if raw.ListenAddr != "" {
			if err := WithListenAddr(raw.ListenAddr)(cfg); err != nil {
				return err
			}
		}
		if raw.BackendAddr != "" {
			if err := WithBackendAddr(raw.BackendAddr)(cfg); err != nil {
				return err
			}
		}
		if raw.BufferSize != 0 {
			if err := WithBufferSize(raw.BufferSize)(cfg); err != nil {
				return err
			}
		}
		if raw.TlSEnabled {
			//nolint:errcheck
			WithTlSEnabled(raw.TlSEnabled)(cfg)
		}
		if raw.CertFilePath != "" {
			if err := WithCertFilePath(raw.CertFilePath)(cfg); err != nil {
				return err
			}
		}
		if raw.KeyFilePath != "" {
			if err := WithKeyFilePath(raw.KeyFilePath)(cfg); err != nil {
				return err
			}
		}
		return nil
	}
}

func WithConfigFile(path string) Option {
	return func(c *config) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read config file: %w", err)
		}
		return WithConfigJSON(b)(c)
	}
}

func WithFlags() Option {
	return func(c *config) error {
		listenAddr := flag.String("listen", listenAddrDefault, "Proxy listen address")
		backendAddr := flag.String("backend", backendAddrDefault, "Backend server address")
		bufferSize := flag.Int("buffer-size", bufferSizeDefault, "Buffer size for data transfer")
		tlsEnabled := flag.Bool("tls-enabled", tlsEnabledDefault, "Enable TLS")
		certFilePath := flag.String("cert-file-path", "", "Path to TLS certificate file")
		keyFilePath := flag.String("key-file-path", "", "Path to TLS key file")
		flag.Parse()

		if *listenAddr != "" {
			if err := WithListenAddr(*listenAddr)(c); err != nil {
				return err
			}
		}
		if *backendAddr != "" {
			if err := WithBackendAddr(*backendAddr)(c); err != nil {
				return err
			}
		}
		if *bufferSize > 0 {
			//nolint:errcheck
			WithBufferSize(*bufferSize)(c)
		}
		if *tlsEnabled {
			//nolint:errcheck
			WithTlSEnabled(*tlsEnabled)(c)
		}
		if *certFilePath != "" {
			if err := WithCertFilePath(*certFilePath)(c); err != nil {
				return err
			}
		}
		if *keyFilePath != "" {
			if err := WithKeyFilePath(*keyFilePath)(c); err != nil {
				return err
			}
		}
		return nil
	}
}

// ---- Helpers ----

func parseAddress(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("split host port: %w", err)
	}
	return host, port, nil
}
