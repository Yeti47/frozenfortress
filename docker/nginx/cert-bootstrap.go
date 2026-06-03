package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	certPath := envOrDefault("FF_TLS_CERT_PATH", "/data/certs/frozenfortress.crt")
	keyPath := envOrDefault("FF_TLS_KEY_PATH", "/data/certs/frozenfortress.key")

	certExists := fileExists(certPath)
	keyExists := fileExists(keyPath)
	if certExists && keyExists {
		return
	}
	if certExists != keyExists {
		fatalf("partial TLS certificate pair found; both %s and %s must exist, or neither", certPath, keyPath)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0700); err != nil {
		fatalf("failed to create certificate directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		fatalf("failed to create key directory: %v", err)
	}

	commonName := envOrDefault("FF_TLS_COMMON_NAME", "frozenfortress.local")
	hosts := parseHosts(envOrDefault("FF_TLS_HOSTS", "localhost,frozenfortress.local"))

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		fatalf("failed to generate TLS key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		fatalf("failed to generate TLS serial: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(825 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, host)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		fatalf("failed to create TLS certificate: %v", err)
	}

	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		fatalf("failed to open certificate file: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		fatalf("failed to write certificate: %v", err)
	}
	if err := certFile.Close(); err != nil {
		fatalf("failed to close certificate file: %v", err)
	}

	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fatalf("failed to open key file: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		fatalf("failed to write key: %v", err)
	}
	if err := keyFile.Close(); err != nil {
		fatalf("failed to close key file: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseHosts(value string) []string {
	parts := strings.Split(value, ",")
	hosts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			hosts = append(hosts, part)
		}
	}
	if len(hosts) == 0 {
		return []string{"localhost", "frozenfortress.local"}
	}
	return hosts
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
