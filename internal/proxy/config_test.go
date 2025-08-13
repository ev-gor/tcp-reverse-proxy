package proxy

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	p, err := CreateProxy()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.config.listenAddr != listenAddrDefault {
		t.Errorf("expected default listen addr %q, got %q", listenAddrDefault, p.config.listenAddr)
	}
	if p.config.backendAddr != backendAddrDefault {
		t.Errorf("expected default backend addr %q, got %q", backendAddrDefault, p.config.backendAddr)
	}
	if p.config.bufferSize != bufferSizeDefault {
		t.Errorf("expected default buffer size %d, got %d", bufferSizeDefault, p.config.bufferSize)
	}
	if p.config.tlsEnabled != tlsEnabledDefault {
		t.Errorf("expected default TLS %v, got %v", tlsEnabledDefault, p.config.tlsEnabled)
	}
}

// -------------------- Positive tests --------------------
func TestWithCertAndKeyFilePath(t *testing.T) {
	certFile, keyFile, err := createTempCertAndKey(t)
	if err != nil {
		t.Fatalf("create temp cert and key: %v", err)
	}
	p, err := CreateProxy(WithCertFilePath(certFile), WithKeyFilePath(keyFile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.config.certFilePath != certFile {
		t.Errorf("expected cert file path %q, got %q", certFile, p.config.certFilePath)
	}
	if p.config.keyFilePath != keyFile {
		t.Errorf("expected key file path %q, got %q", keyFile, p.config.keyFilePath)
	}
}

func TestFromEnv(t *testing.T) {
	certFile, keyFile, err := createTempCertAndKey(t)
	if err != nil {
		t.Fatalf("create temp cert and key: %v", err)
	}
	t.Setenv("TEST_LISTEN_ADDR", "127.0.0.1:9999")
	t.Setenv("TEST_BACKEND_ADDR", "127.0.0.1:8888")
	t.Setenv("TEST_BUFFER_SIZE", "64")
	t.Setenv("TEST_TLS_ENABLED", "true")
	t.Setenv("TEST_CERT_FILE_PATH", certFile)
	t.Setenv("TEST_KEY_FILE_PATH", keyFile)
	defer os.Clearenv()

	p, err := CreateProxy(FromEnv("TEST"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.config.listenAddr != "127.0.0.1:9999" {
		t.Errorf("expected listen addr %q, got %q", "127.0.0.1:9999", p.config.listenAddr)
	}
	if p.config.backendAddr != "127.0.0.1:8888" {
		t.Errorf("expected backend addr %q, got %q", "127.0.0.1:8888", p.config.backendAddr)
	}
	if p.config.bufferSize != 64 {
		t.Errorf("expected buffer size 64, got %d", p.config.bufferSize)
	}
	if !p.config.tlsEnabled {
		t.Errorf("expected TLS enabled")
	}
	if p.config.certFilePath != certFile {
		t.Errorf("expected cert file path %q, got %q", certFile, p.config.certFilePath)
	}
	if p.config.keyFilePath != keyFile {
		t.Errorf("expected key file path %q, got %q", keyFile, p.config.keyFilePath)
	}
}

func TestWithFlags(t *testing.T) {
	certFile, keyFile, err := createTempCertAndKey(t)
	if err != nil {
		t.Fatalf("create temp cert and key: %v", err)
	}
	os.Args = []string{
		"cmd",
		"-listen", "127.0.0.1:7000",
		"-backend", "127.0.0.1:7001",
		"-buffer-size", "256",
		"-tls-enabled",
		"-cert-file-path", certFile,
		"-key-file-path", keyFile,
	}

	p, err := CreateProxy(WithFlags())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.config.listenAddr != "127.0.0.1:7000" {
		t.Errorf("got listen addr %q", p.config.listenAddr)
	}
	if p.config.backendAddr != "127.0.0.1:7001" {
		t.Errorf("got backend addr %q", p.config.backendAddr)
	}
	if p.config.bufferSize != 256 {
		t.Errorf("got buffer size %d", p.config.bufferSize)
	}
	if !p.config.tlsEnabled {
		t.Errorf("expected TLS enabled")
	}
	if p.config.certFilePath != certFile {
		t.Errorf("got cert file path %q", p.config.certFilePath)
	}
	if p.config.keyFilePath != keyFile {
		t.Errorf("got key file path %q", p.config.keyFilePath)
	}
}

func TestWithConfigJSON(t *testing.T) {
	jsonConfig := `{
		"listen_addr": "0.0.0.0:1111",
		"backend_addr": "0.0.0.0:2222",
		"buffer_size": 128,
		"tls_enabled": true
	}`

	p, err := CreateProxy(WithConfigJSON([]byte(jsonConfig)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.config.listenAddr != "0.0.0.0:1111" {
		t.Errorf("got listen addr %q", p.config.listenAddr)
	}
	if p.config.backendAddr != "0.0.0.0:2222" {
		t.Errorf("got backend addr %q", p.config.backendAddr)
	}
	if p.config.bufferSize != 128 {
		t.Errorf("got buffer size %d", p.config.bufferSize)
	}
	if !p.config.tlsEnabled {
		t.Errorf("expected TLS enabled")
	}
}

func TestWithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.json")
	content := `{"listen_addr": "1.2.3.4:5555"}`
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	p, err := CreateProxy(WithConfigFile(tmpFile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.config.listenAddr != "1.2.3.4:5555" {
		t.Errorf("expected listen addr 1.2.3.4:5555, got %q", p.config.listenAddr)
	}
}

// -------------------- Negative tests --------------------

func TestInvalidAddress(t *testing.T) {
	_, err := CreateProxy(WithListenAddr("invalid"))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected address parsing error, got %v", err)
	}
}

func TestWithBackendAddrInvalid(t *testing.T) {
	_, err := CreateProxy(WithBackendAddr("invalid"))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected backend address parsing error, got %v", err)
	}
}

func TestInvalidBufferSize(t *testing.T) {
	_, err := CreateProxy(WithBufferSize(0))
	if err == nil || !strings.Contains(err.Error(), "buffer size must be positive") {
		t.Errorf("expected buffer size error, got %v", err)
	}
}

func TestMissingCertFile(t *testing.T) {
	_, err := CreateProxy(WithCertFilePath("/nonexistent/cert.pem"))
	if err == nil || !strings.Contains(err.Error(), "cert file path") {
		t.Errorf("expected cert file error, got %v", err)
	}
}

func TestMissingKeyFile(t *testing.T) {
	_, err := CreateProxy(WithKeyFilePath("/nonexistent/key.pem"))
	if err == nil || !strings.Contains(err.Error(), "key file path") {
		t.Errorf("expected key file error, got %v", err)
	}
}

func TestFromEnvInvalidValues(t *testing.T) {
	// Invalid listen address
	t.Setenv("BAD_LISTEN_ADDR_LISTEN_ADDR", "invalid")
	_, err := CreateProxy(FromEnv("BAD_LISTEN_ADDR"))
	if err == nil || !strings.Contains(err.Error(), "parse address") {
		t.Errorf("expected parse address error, got %v", err)
	}
	os.Clearenv()

	// Invalid backend address
	t.Setenv("BAD_BACKEND_ADDR_BACKEND_ADDR", "invalid")
	_, err = CreateProxy(FromEnv("BAD_BACKEND_ADDR"))
	if err == nil || !strings.Contains(err.Error(), "parse address") {
		t.Errorf("expected parse address error, got %v", err)
	}
	os.Clearenv()

	// Invalid buffer size (non-integer)
	t.Setenv("BAD_BUFFER_SIZE_BUFFER_SIZE", "abc")
	_, err = CreateProxy(FromEnv("BAD_BUFFER_SIZE"))
	if err == nil || !strings.Contains(err.Error(), "buffer size") {
		t.Errorf("expected buffer size parse error, got %v", err)
	}
	os.Clearenv()

	// Invalid buffer size (<=0)
	t.Setenv("BAD_BUFFER_SIZE2_BUFFER_SIZE", "-1")
	_, err = CreateProxy(FromEnv("BAD_BUFFER_SIZE2"))
	if err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Errorf("expected buffer size positive error, got %v", err)
	}
	os.Clearenv()

	// Missing cert file
	t.Setenv("BAD_CERT_CERT_FILE_PATH", "/nonexistent/cert.pem")
	_, err = CreateProxy(FromEnv("BAD_CERT"))
	if err == nil || !strings.Contains(err.Error(), "cert file path") {
		t.Errorf("expected cert file error, got %v", err)
	}
	os.Clearenv()

	// Missing key file
	t.Setenv("BAD_KEY_KEY_FILE_PATH", "/nonexistent/key.pem")
	_, err = CreateProxy(FromEnv("BAD_KEY"))
	if err == nil || !strings.Contains(err.Error(), "key file path") {
		t.Errorf("expected key file error, got %v", err)
	}
	os.Clearenv()
}

func TestWithFlagsInvalidValues(t *testing.T) {
	resetFlags()
	os.Args = []string{
		"cmd",
		"-listen", "invalid",
	}
	_, err := CreateProxy(WithFlags())
	if err == nil || !strings.Contains(err.Error(), "parse address") {
		t.Errorf("expected parse address error, got %v", err)
	}

	resetFlags()
	os.Args = []string{
		"cmd",
		"-backend", "invalid",
	}
	_, err = CreateProxy(WithFlags())
	if err == nil || !strings.Contains(err.Error(), "parse address") {
		t.Errorf("expected parse address error, got %v", err)
	}

	resetFlags()
	os.Args = []string{
		"cmd",
		"-cert-file-path", "/nonexistent/cert.pem",
	}
	_, err = CreateProxy(WithFlags())
	if err == nil || !strings.Contains(err.Error(), "cert file path") {
		t.Errorf("expected cert file error, got %v", err)
	}

	resetFlags()
	os.Args = []string{
		"cmd",
		"-key-file-path", "/nonexistent/key.pem",
	}
	_, err = CreateProxy(WithFlags())
	if err == nil || !strings.Contains(err.Error(), "key file path") {
		t.Errorf("expected key file error, got %v", err)
	}

	resetFlags()
}

func TestWithConfigJSONEmpty(t *testing.T) {
	opt := WithConfigJSON([]byte{})
	cfg := config{}
	err := opt(&cfg)
	if err != nil {
		t.Errorf("expected nil error for empty JSON, got %v", err)
	}
}

func TestWithConfigJSONInvalidJSON(t *testing.T) {
	_, err := CreateProxy(WithConfigJSON([]byte("{invalid json}")))
	if err == nil || !strings.Contains(err.Error(), "parse json config") {
		t.Errorf("expected JSON parse error, got %v", err)
	}
}

func TestWithConfigJSONInvalidFields(t *testing.T) {
	// Invalid listen address
	b := []byte(`{"listen_addr":"invalid"}`)
	_, err := CreateProxy(WithConfigJSON(b))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected parse address error, got %v", err)
	}

	// Invalid backend address
	b = []byte(`{"backend_addr":"invalid"}`)
	_, err = CreateProxy(WithConfigJSON(b))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected parse address error, got %v", err)
	}

	// Invalid buffer size
	b = []byte(`{"buffer_size": -1}`)
	_, err = CreateProxy(WithConfigJSON(b))
	if err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Errorf("expected buffer size error, got %v", err)
	}

	// Missing cert file
	b = []byte(`{"cert_file_path":"/nonexistent/cert.pem"}`)
	_, err = CreateProxy(WithConfigJSON(b))
	if err == nil || !strings.Contains(err.Error(), "cert file path") {
		t.Errorf("expected cert file error, got %v", err)
	}

	// Missing key file
	b = []byte(`{"key_file_path":"/nonexistent/key.pem"}`)
	_, err = CreateProxy(WithConfigJSON(b))
	if err == nil || !strings.Contains(err.Error(), "key file path") {
		t.Errorf("expected key file error, got %v", err)
	}
}

func TestWithConfigFileInvalidPath(t *testing.T) {
	_, err := CreateProxy(WithConfigFile("/nonexistent/config.json"))
	if err == nil || !strings.Contains(err.Error(), "read config file") {
		t.Errorf("expected config file read error, got %v", err)
	}
}

func TestWithConfigFileInvalidFields(t *testing.T) {
	tmpDir := t.TempDir()

	// Invalid listen address
	tmp := filepath.Join(tmpDir, "cfg1.json")
	os.WriteFile(tmp, []byte(`{"listen_addr":"invalid"}`), 0o644)
	_, err := CreateProxy(WithConfigFile(tmp))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected parse address error, got %v", err)
	}

	// Invalid backend address
	tmp = filepath.Join(tmpDir, "cfg2.json")
	os.WriteFile(tmp, []byte(`{"backend_addr":"invalid"}`), 0o644)
	_, err = CreateProxy(WithConfigFile(tmp))
	if err == nil || !strings.Contains(err.Error(), "split host port") {
		t.Errorf("expected parse address error, got %v", err)
	}

	// Invalid buffer size
	tmp = filepath.Join(tmpDir, "cfg3.json")
	os.WriteFile(tmp, []byte(`{"buffer_size": -1}`), 0o644)
	_, err = CreateProxy(WithConfigFile(tmp))
	if err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Errorf("expected buffer size error, got %v", err)
	}

	// Missing cert file
	tmp = filepath.Join(tmpDir, "cfg4.json")
	os.WriteFile(tmp, []byte(`{"cert_file_path":"/nonexistent/cert.pem"}`), 0o644)
	_, err = CreateProxy(WithConfigFile(tmp))
	if err == nil || !strings.Contains(err.Error(), "cert file path") {
		t.Errorf("expected cert file error, got %v", err)
	}

	// Missing key file
	tmp = filepath.Join(tmpDir, "cfg5.json")
	os.WriteFile(tmp, []byte(`{"key_file_path":"/nonexistent/key.pem"}`), 0o644)
	_, err = CreateProxy(WithConfigFile(tmp))
	if err == nil || !strings.Contains(err.Error(), "key file path") {
		t.Errorf("expected key file error, got %v", err)
	}
}

// ----------helpers----------------
func createTempCertAndKey(t *testing.T) (string, string, error) {
	t.Helper()
	tmpDir := t.TempDir()
	certFile := filepath.Join(tmpDir, "cert.pem")
	keyFile := filepath.Join(tmpDir, "key.pem")
	if err := os.WriteFile(certFile, []byte("cert"), 0o644); err != nil {
		return "", "", fmt.Errorf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyFile, []byte("key"), 0o644); err != nil {
		return "", "", fmt.Errorf("write key file: %v", err)
	}

	return certFile, keyFile, nil
}

func resetFlags() {
	flag.CommandLine = flag.NewFlagSet("cmd", flag.ExitOnError)
}
