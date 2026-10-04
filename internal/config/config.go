package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Config holds settings read from the environment (and an optional .env file).
type Config struct {
	// Port is the TCP port the HTTP server listens on (PORT, default 8080).
	Port string
	// Env is "development" or "production" (APP_ENV, default production).
	Env string
	// TrustedProxies lists reverse-proxy networks whose X-Forwarded-For header
	// is trusted to identify the client (TRUSTED_PROXIES, comma-separated
	// CIDRs or IPs). Empty means the TCP peer address is always used.
	TrustedProxies []netip.Prefix
}

// IsDev reports whether the app runs in development mode.
func (c *Config) IsDev() bool {
	return c.Env == EnvDevelopment
}

// LoadConfig reads configuration. Real environment variables take precedence
// over values in .env.
func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("loading .env: %w", err)
	}

	cfg := &Config{
		Port: getenv("PORT", "8080"),
		Env:  strings.ToLower(getenv("APP_ENV", EnvProduction)),
	}

	if cfg.Env != EnvDevelopment && cfg.Env != EnvProduction {
		return nil, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.Env)
	}

	proxies, err := parsePrefixes(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return nil, fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}
	cfg.TrustedProxies = proxies

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// parsePrefixes parses a comma-separated list of CIDRs or bare IP addresses.
func parsePrefixes(value string) ([]netip.Prefix, error) {
	var prefixes []netip.Prefix
	for _, field := range strings.Split(value, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		if strings.Contains(field, "/") {
			prefix, err := netip.ParsePrefix(field)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(field)
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
	}
	return prefixes, nil
}
