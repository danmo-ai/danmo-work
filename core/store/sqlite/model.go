package sqlite

import (
	"encoding/json"

	"danmo-work/core/domain"

	"gorm.io/gorm"
)

type llmConfigModel struct {
	ID             string `gorm:"primaryKey"`
	Provider       string
	Name           string
	APIKey         string
	BaseURL        string
	// Column kept as default_dialect (protocol dialect default for the provider).
	DefaultProtocolDialect string `gorm:"column:default_dialect"`
	ModelsJSON             string `gorm:"column:models"`
	CreatedAt              string
	UpdatedAt              string
}

func (llmConfigModel) TableName() string { return "llm_configs" }

func (m *llmConfigModel) BeforeSave(_ *gorm.DB) error {
	refs := make([]domain.LLMModelRef, 0)
	for _, ref := range m.toModelRefs() {
		refs = append(refs, ref)
	}
	b, err := json.Marshal(refs)
	if err != nil {
		return err
	}
	m.ModelsJSON = string(b)
	return nil
}

func (m *llmConfigModel) AfterFind(_ *gorm.DB) error {
	// no-op, models are deserialized in toDomain
	return nil
}

func (m *llmConfigModel) toModelRefs() []domain.LLMModelRef {
	if m.ModelsJSON == "" {
		return nil
	}
	var raw []struct {
		Name               string `json:"name"`
		Enabled            bool   `json:"enabled"`
		ProtocolDialect    string `json:"protocolDialect"`
		ReasoningDialect   string `json:"reasoningDialect"` // legacy alias
	}
	if err := json.Unmarshal([]byte(m.ModelsJSON), &raw); err != nil {
		var refs []domain.LLMModelRef
		_ = json.Unmarshal([]byte(m.ModelsJSON), &refs)
		return refs
	}
	out := make([]domain.LLMModelRef, 0, len(raw))
	for _, r := range raw {
		d := r.ProtocolDialect
		if d == "" {
			d = r.ReasoningDialect
		}
		out = append(out, domain.LLMModelRef{
			Name:            r.Name,
			Enabled:         r.Enabled,
			ProtocolDialect: d,
		})
	}
	return out
}

func (m llmConfigModel) toDomain() domain.LLMProviderConfig {
	return domain.LLMProviderConfig{
		ID:             m.ID,
		Provider:       domain.LLMProviderType(m.Provider),
		Name:           m.Name,
		APIKey:         m.APIKey,
		BaseURL:        m.BaseURL,
		DefaultProtocolDialect: m.DefaultProtocolDialect,
		Models:         m.toModelRefs(),
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func llmConfigModelFromDomain(cfg domain.LLMProviderConfig) llmConfigModel {
	b, _ := json.Marshal(cfg.Models)
	return llmConfigModel{
		ID:             cfg.ID,
		Provider:       string(cfg.Provider),
		Name:           cfg.Name,
		APIKey:         cfg.APIKey,
		BaseURL:        cfg.BaseURL,
		DefaultProtocolDialect: cfg.DefaultProtocolDialect,
		ModelsJSON:     string(b),
		CreatedAt:      cfg.CreatedAt,
		UpdatedAt:      cfg.UpdatedAt,
	}
}
