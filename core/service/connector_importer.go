package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"danmo-work/core/domain"
)

// ConnectorImporter parses a market connector package directory into a catalog entry.
type ConnectorImporter struct{}

func NewConnectorImporter() *ConnectorImporter {
	return &ConnectorImporter{}
}

// Import reads connector.json (preferred) from dirPath.
func (i *ConnectorImporter) Import(dirPath string) (*domain.ConnectorCatalogEntry, error) {
	path := filepath.Join(dirPath, "connector.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read connector.json: %w", err)
	}
	var entry domain.ConnectorCatalogEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("parse connector.json: %w", err)
	}
	if entry.ID == "" {
		entry.ID = filepath.Base(dirPath)
	}
	if entry.Name == "" {
		return nil, fmt.Errorf("connector.json: name is required")
	}
	entry.Transport = strings.TrimSpace(entry.Transport)
	if entry.Transport == "" {
		return nil, fmt.Errorf("connector.json: transport is required")
	}
	if entry.Auth == "" {
		entry.Auth = domain.MCPAuthNone
	}
	return &entry, nil
}
