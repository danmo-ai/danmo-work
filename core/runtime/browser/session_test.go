package browser

import (
	"context"
	"testing"
	"time"

	"danmo-work/core/domain"
)

func TestAcquirePage_Disabled(t *testing.T) {
	m := New(domain.ConfigBrowserSection{Enabled: false})
	defer m.Close(context.Background())
	_, err := m.AcquirePage(context.Background(), "s1")
	if err == nil {
		t.Fatal("expected error when disabled")
	}
}

func TestClosePage_Idempotent(t *testing.T) {
	m := New(domain.ConfigBrowserSection{Enabled: false})
	defer m.Close(context.Background())
	if err := m.ClosePage(context.Background(), "missing"); err != nil {
		t.Fatal(err)
	}
	if err := m.CloseAll(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAcquirePage_RequiresSessionID(t *testing.T) {
	m := New(domain.ConfigBrowserSection{
		Enabled: true,
		CDPURL:  "http://127.0.0.1:9222",
	})
	defer m.Close(context.Background())
	_, err := m.AcquirePage(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected session id required")
	}
}

func TestSessionIdleTTLConstant(t *testing.T) {
	if sessionIdleTTL != 15*time.Minute {
		t.Fatalf("idle ttl=%v", sessionIdleTTL)
	}
}
