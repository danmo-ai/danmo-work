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
	an := DefaultEffortsForProvider(domain.LLMProviderAnthropic)
	if len(an) < 2 || an[0] != "off" {
		t.Fatalf("anthropic efforts=%v", an)
	}
	if DefaultEffortsForProvider(domain.LLMProviderMock) != nil {
		t.Fatal("mock should have no default efforts")
	}
}
