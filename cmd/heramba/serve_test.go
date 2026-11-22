package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Armur-Ai/heramba/internal/config"
)

func TestDrainFlipsReadyAndWaits(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	var slept time.Duration
	drain(&ready, 3*time.Second, func(d time.Duration) { slept = d })

	if ready.Load() {
		t.Error("drain must flip ready to false")
	}
	if slept != 3*time.Second {
		t.Errorf("drain should wait the window, slept %s want 3s", slept)
	}
}

func TestDrainZeroWindowDoesNotSleep(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	called := false
	drain(&ready, 0, func(time.Duration) { called = true })

	if ready.Load() {
		t.Error("drain must flip ready to false even with no window")
	}
	if called {
		t.Error("drain with zero window must not sleep")
	}
}

func TestDrainNilReadyIsSafe(t *testing.T) {
	called := false
	drain(nil, 1*time.Second, func(time.Duration) { called = true })
	if !called {
		t.Error("drain with a window should still sleep even if ready is nil")
	}
}

func TestTLSEnabledAndValidate(t *testing.T) {
	if (config.Config{}).TLSEnabled() {
		t.Error("empty config should not enable TLS")
	}
	both := config.Config{TLS: config.TLSConfig{CertFile: "c.pem", KeyFile: "k.pem"}}
	if !both.TLSEnabled() {
		t.Error("both files set should enable TLS")
	}
	// Half-configured TLS must fail validation.
	half := config.Default()
	half.TLS.CertFile = "c.pem"
	if err := half.Validate(); err == nil {
		t.Error("cert without key must be a validation error")
	}
	half2 := config.Default()
	half2.TLS.KeyFile = "k.pem"
	if err := half2.Validate(); err == nil {
		t.Error("key without cert must be a validation error")
	}
	full := config.Default()
	full.TLS = config.TLSConfig{CertFile: "c.pem", KeyFile: "k.pem"}
	if err := full.Validate(); err != nil {
		t.Errorf("both files set should validate: %v", err)
	}
}

// genSelfSigned writes a self-signed cert/key for 127.0.0.1 to dir and returns
// the file paths and a cert pool trusting it.
func genSelfSigned(t *testing.T, dir string) (certPath, keyPath string, pool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "heramba-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	pool = x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return certPath, keyPath, pool
}

func TestServeEdgeServesHTTPS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // free the port for the server to rebind

	dir := t.TempDir()
	certPath, keyPath, pool := genSelfSigned(t, dir)
	cfg := config.Default()
	cfg.TLS = config.TLSConfig{CertFile: certPath, KeyFile: keyPath}

	srv := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("secure-ok"))
		}),
	}
	go func() { _ = serveEdge(srv, cfg) }()
	defer srv.Close()

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}}
	var body string
	for i := 0; i < 50; i++ {
		resp, err := client.Get("https://" + addr + "/")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if body != "secure-ok" {
		t.Fatalf("expected HTTPS response secure-ok, got %q", body)
	}
}
