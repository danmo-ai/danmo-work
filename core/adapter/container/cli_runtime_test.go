package container

import (
	"testing"

	"danmo-work/core/domain"
)

func TestResourceFlagsDockerOmitsEmpty(t *testing.T) {
	if got := resourceFlagsDocker(domain.EnvironmentResources{}); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
	got := resourceFlagsDocker(domain.EnvironmentResources{CPUs: "2", Memory: "1g", Pids: 256})
	want := []string{"--cpus", "2", "--memory", "1g", "--pids-limit", "256"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestResourceFlagsAppleNoPids(t *testing.T) {
	got := resourceFlagsApple(domain.EnvironmentResources{CPUs: "2", Memory: "512m", Pids: 100})
	if len(got) != 4 || got[0] != "--cpus" || got[2] != "--memory" {
		t.Fatalf("got %v", got)
	}
}
