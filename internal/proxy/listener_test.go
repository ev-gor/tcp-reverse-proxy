package proxy

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// a helper to generate a temp self-signed certificate
func generateTempCert(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to open cert file: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to open key file: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	keyOut.Close()

	return certPath, keyPath
}

func TestTCPListenerFactory(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cfg := config{listenAddr: "127.0.0.1:0"}
		ln, err := tcpListenerFactory(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer ln.Close()
	})

	t.Run("invalid address", func(t *testing.T) {
		cfg := config{listenAddr: "invalid:address"}
		ln, err := tcpListenerFactory(cfg)
		if err == nil {
			ln.Close()
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestTLSListenerFactory(t *testing.T) {
	t.Run("empty cert or key path", func(t *testing.T) {
		cfg := config{listenAddr: "127.0.0.1:0"}
		ln, err := tlsListenerFactory(cfg)
		if err == nil {
			ln.Close()
			t.Fatalf("expected error for empty cert/key path")
		}
	})

	t.Run("invalid cert path", func(t *testing.T) {
		cfg := config{
			listenAddr:   "127.0.0.1:0",
			certFilePath: "nonexistent-cert.pem",
			keyFilePath:  "nonexistent-key.pem",
		}
		ln, err := tlsListenerFactory(cfg)
		if err == nil {
			ln.Close()
			t.Fatalf("expected error for invalid cert path")
		}
	})

	t.Run("success with temp cert", func(t *testing.T) {
		tmpDir := t.TempDir()
		certPath, keyPath := generateTempCert(t, tmpDir)

		cfg := config{
			listenAddr:   "127.0.0.1:0",
			certFilePath: certPath,
			keyFilePath:  keyPath,
		}
		ln, err := tlsListenerFactory(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer ln.Close()

		// Серверная сторона — принимает соединение и отправляет данные
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			conn.Write([]byte("hello"))
		}()

		// Клиентская сторона
		clientConfig := &tls.Config{
			InsecureSkipVerify: true, // отключаем проверку CA для теста
		}

		conn, err := tls.Dial("tcp", ln.Addr().String(), clientConfig)
		if err != nil {
			t.Fatalf("failed to dial TLS listener: %v", err)
		}
		defer conn.Close()

		buf := make([]byte, 5)
		_, err = conn.Read(buf)
		if err != nil {
			t.Fatalf("failed to read from TLS connection: %v", err)
		}
		if string(buf) != "hello" {
			t.Errorf("expected 'hello', got %q", string(buf))
		}
	})

	t.Run("check that traffic is really encrypted", func(t *testing.T) {
		tmpDir := t.TempDir()
		certPath, keyPath := generateTempCert(t, tmpDir)

		rawBuffer := &bytes.Buffer{}

		// create TCP listener, but wrap the connection to log the encrypted message
		tcpLn, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("failed to create tcp listener: %v", err)
		}

		// mock Accept() for logging
		spyLn := &spyListener{Listener: tcpLn, buf: rawBuffer}

		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			t.Fatalf("failed to load cert: %v", err)
		}
		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}

		// create TLS listener based on spyListener
		ln := tls.NewListener(spyLn, tlsConfig)
		defer ln.Close()

		// сервер
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			conn.Write([]byte("hello"))
		}()

		// клиент
		clientConfig := &tls.Config{InsecureSkipVerify: true}
		conn, err := tls.Dial("tcp", ln.Addr().String(), clientConfig)
		if err != nil {
			t.Fatalf("failed to dial TLS listener: %v", err)
		}

		buf := make([]byte, 5)
		_, err = io.ReadFull(conn, buf)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		conn.Close()

		if string(buf) != "hello" {
			t.Errorf("expected 'hello', got %q", string(buf))
		}

		// check that the message is encrypted
		if strings.Contains(rawBuffer.String(), "hello") {
			t.Errorf("found plaintext 'hello' in raw TCP stream: %q", rawBuffer.String())
		}
	})
}

type spyListener struct {
	net.Listener
	buf *bytes.Buffer
}

func (sl *spyListener) Accept() (net.Conn, error) {
	c, err := sl.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &connSpy{Conn: c, spyBuffer: sl.buf}, nil
}

type connSpy struct {
	net.Conn
	spyBuffer *bytes.Buffer
}

func (c *connSpy) Write(p []byte) (n int, err error) {
	c.spyBuffer.Write(p)
	return c.Conn.Write(p)
}

func (c *connSpy) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 {
		c.spyBuffer.Write(p[:n])
	}
	return n, err
}
