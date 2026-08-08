package connector

import (
	"os"
	"strings"
)

// Config controls the PC → Hub connector.
type Config struct {
	Enabled     bool
	HubURL      string // wss://host/v1/connector or https://host (path filled)
	LocalBase   string // http://127.0.0.1:7801
	TLSInsecure bool   // dev only
	AppVersion  string
	IdentityPath string
}

// FromEnv overlays WORK_HUB_URL / WORK_REMOTE_ENABLED on cfg.
func (c Config) WithEnv() Config {
	if v := strings.TrimSpace(os.Getenv("WORK_HUB_URL")); v != "" {
		c.HubURL = v
		c.Enabled = true
	}
	if v := strings.TrimSpace(os.Getenv("WORK_REMOTE_ENABLED")); v != "" {
		c.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := strings.TrimSpace(os.Getenv("WORK_REMOTE_TLS_INSECURE")); v == "1" || strings.EqualFold(v, "true") {
		c.TLSInsecure = true
	}
	if c.LocalBase == "" {
		c.LocalBase = "http://127.0.0.1:7801"
	}
	if c.AppVersion == "" {
		c.AppVersion = "dev"
	}
	return c
}

// NormalizeConnectorURL ensures a wss/ws URL ending with /v1/connector.
func NormalizeConnectorURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(u, "https://"):
		u = "wss://" + strings.TrimPrefix(u, "https://")
	case strings.HasPrefix(u, "http://"):
		u = "ws://" + strings.TrimPrefix(u, "http://")
	}
	u = strings.TrimRight(u, "/")
	if !strings.HasSuffix(u, "/v1/connector") {
		u += "/v1/connector"
	}
	return u
}

// HubHTTPSBase derives https://host from a connector URL for pairing HTTP calls.
func HubHTTPSBase(connectorURL string) string {
	u := NormalizeConnectorURL(connectorURL)
	u = strings.TrimSuffix(u, "/v1/connector")
	switch {
	case strings.HasPrefix(u, "wss://"):
		return "https://" + strings.TrimPrefix(u, "wss://")
	case strings.HasPrefix(u, "ws://"):
		return "http://" + strings.TrimPrefix(u, "ws://")
	default:
		return u
	}
}
