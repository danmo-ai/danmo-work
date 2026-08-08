package connector

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"danmo-work/core/paths"
)

// Identity is the durable PC device credential for Hub registration.
type Identity struct {
	DeviceID     string `json:"device_id"`
	DeviceSecret string `json:"device_secret"`
}

type identityStore struct {
	mu   sync.Mutex
	path string
}

func defaultIdentityPath() string {
	return filepath.Join(paths.Home(), "remote.json")
}

// LoadOrCreateIdentity reads ~/.danmo-work/remote.json or creates a new identity.
func LoadOrCreateIdentity(path string) (*Identity, error) {
	if path == "" {
		path = defaultIdentityPath()
	}
	s := &identityStore{path: path}
	return s.loadOrCreate()
}

func (s *identityStore) loadOrCreate() (*Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err == nil {
		var id Identity
		if err := json.Unmarshal(data, &id); err != nil {
			return nil, fmt.Errorf("remote identity: %w", err)
		}
		if id.DeviceID != "" && id.DeviceSecret != "" {
			return &id, nil
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	id, err := newIdentity()
	if err != nil {
		return nil, err
	}
	if err := s.write(id); err != nil {
		return nil, err
	}
	return id, nil
}

func (s *identityStore) write(id *Identity) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newIdentity() (*Identity, error) {
	idBytes := make([]byte, 16)
	secBytes := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, err
	}
	if _, err := rand.Read(secBytes); err != nil {
		return nil, err
	}
	return &Identity{
		DeviceID:     hex.EncodeToString(idBytes),
		DeviceSecret: hex.EncodeToString(secBytes),
	}, nil
}
