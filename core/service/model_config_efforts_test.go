package service

import (
	"testing"

	"danmo-work/core/domain"
)

func TestDefaultEffortsForProvider(t *testing.T) {
	oa := DefaultEffortsForProvider(domain.LLMProviderOpenAI)
	if len(oa) < 2 || oa[0] != "off" {
		t.Fatalf("openai efforts=%v", oa)
	}
	resp := DefaultEffortsForProvider(domain.LLMProviderOpenAIResponses)
	if len(resp) < 2 || resp[0] != "off" {
		t.Fatalf("openai_responses efforts=%v", resp)
	}
	local := DefaultEffortsForProvider(domain.LLMProviderLocal)
	if len(local) < 2 || local[0] != "off" {
		t.Fatalf("local (alias) efforts=%v", local)
	}
	an := DefaultEffortsForProvider(domain.LLMProviderAnthropic)
	if len(an) < 2 || an[0] != "off" {
		t.Fatalf("anthropic efforts=%v", an)
	}
	if DefaultEffortsForProvider(domain.LLMProviderMock) != nil {
		t.Fatal("mock should have no default efforts")
	}
}
