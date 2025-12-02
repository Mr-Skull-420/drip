package tls

import (
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/crypto/acme/autocert"
)

// AutoCertManager manages automatic certificate provisioning with Let's Encrypt
type AutoCertManager struct {
	manager *autocert.Manager
	logger  *zap.Logger
}

// NewAutoCertManager creates a new AutoCert manager
func NewAutoCertManager(domain, cacheDir string, logger *zap.Logger) *AutoCertManager {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain, "*."+domain),
		Cache:      autocert.DirCache(cacheDir),
	}

	logger.Info("AutoTLS enabled",
		zap.String("domain", domain),
		zap.String("cache_dir", cacheDir),
	)

	return &AutoCertManager{
		manager: m,
		logger:  logger,
	}
}

// GetTLSConfig returns the TLS configuration
func (a *AutoCertManager) GetTLSConfig() *tls.Config {
	return a.manager.TLSConfig()
}

// HTTPHandler returns the HTTP handler for ACME challenges
func (a *AutoCertManager) HTTPHandler() http.Handler {
	return a.manager.HTTPHandler(nil)
}

// GetCertificate gets a certificate for the given ClientHelloInfo
func (a *AutoCertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := a.manager.GetCertificate(hello)
	if err != nil {
		a.logger.Error("Failed to get certificate",
			zap.String("server_name", hello.ServerName),
			zap.Error(err),
		)
		return nil, err
	}

	a.logger.Debug("Certificate obtained",
		zap.String("server_name", hello.ServerName),
	)

	return cert, nil
}

// DefaultCacheDir returns the default cache directory for certificates
func DefaultCacheDir() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".drip", "certs")
}
