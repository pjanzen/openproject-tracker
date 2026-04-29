package auth

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	"github.com/pjanzen/openproject-tracker/internal/config"
)

// headerTransport wraps an http.RoundTripper and injects extra headers.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for k, v := range t.headers {
		clone.Header.Set(k, v)
	}
	return t.base.RoundTrip(clone)
}

// BuildTransport creates an http.RoundTripper from config settings.
func BuildTransport(cfg *config.Config) http.RoundTripper {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.SkipTLSVerify, //nolint:gosec
	}
	if cfg.CAPath != "" {
		pem, err := os.ReadFile(cfg.CAPath)
		if err == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(pem)
			tlsCfg.RootCAs = pool
		}
	}
	base := &http.Transport{TLSClientConfig: tlsCfg}
	if len(cfg.ExtraHeaders) == 0 {
		return base
	}
	return &headerTransport{base: base, headers: cfg.ExtraHeaders}
}
