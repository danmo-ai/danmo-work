package sqlite

import (
	"encoding/json"

	"danmo-work/core/domain"

	"gorm.io/gorm"
)

type llmConfigModel struct {
	ID         string `gorm:"primaryKey"`
	Provider   string
	Name       string
	APIKey     string
	BaseURL    string
	ModelsJSON string `gorm:"column:models"`
	CreatedAt  string
	UpdatedAt  string
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
	var refs []domain.LLMModelRef
	if m.ModelsJSON != "" {
		_ = json.Unmarshal([]byte(m.ModelsJSON), &refs)
	}
	return refs
}

func (m llmConfigModel) toDomain() domain.LLMProviderConfig {
	return domain.LLMProviderConfig{
		ID:        m.ID,
		Provider:  domain.LLMProviderType(m.Provider),
		Name:      m.Name,
		APIKey:    m.APIKey,
		BaseURL:   m.BaseURL,
		Models:    m.toModelRefs(),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

func llmConfigModelFromDomain(cfg domain.LLMProviderConfig) llmConfigModel {
	b, _ := json.Marshal(cfg.Models)
	return llmConfigModel{
		ID:         cfg.ID,
		Provider:   string(cfg.Provider),
		Name:       cfg.Name,
		APIKey:     cfg.APIKey,
		BaseURL:    cfg.BaseURL,
		ModelsJSON: string(b),
		CreatedAt:  cfg.CreatedAt,
		UpdatedAt:  cfg.UpdatedAt,
	}
}
