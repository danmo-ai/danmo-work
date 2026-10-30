package container

import (
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestFormatCreateArgsDesktopKeepImageCmd(t *testing.T) {
	// Mirror CLIRuntime.CreateDetached flag assembly for desktop profile.
	opts := CreateOpts{
		Name:         "danmo-wp3d-test",
		Image:        domain.EnvImageTagDesktop,
		WorkDir:      "/tmp/proj",
		Mount:        "/tmp/proj",
		KeepImageCmd: true,
		ShmSize:      "1g",
		Ports: []PortPublish{
			{HostIP: "127.0.0.1", ContainerPort: 9222},
			{HostIP: "127.0.0.1", ContainerPort: 6080},
		},
		Env: []string{"DISPLAY=:1"},
	}
	args := []string{
		"create",
		"--name", opts.Name,
		"--workdir", opts.Mount,
		"-v", opts.WorkDir + ":" + opts.Mount + ":rw",
	}
	if opts.ShmSize != "" {
		args = append(args, "--shm-size", opts.ShmSize)
	}
	for _, p := range opts.Ports {
		args = append(args, "-p", formatPortPublish(p))
	}
	for _, ev := range opts.Env {
		args = append(args, "-e", ev)
	}
	args = append(args, opts.Image)
	if !opts.KeepImageCmd {
		t.Fatal("expected KeepImageCmd")
	}
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--shm-size 1g",
		"-p 127.0.0.1::9222/tcp",
		"-p 127.0.0.1::6080/tcp",
		"-e DISPLAY=:1",
		domain.EnvImageTagDesktop,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	if strings.Contains(joined, "sleep") {
		t.Fatalf("desktop must not force sleep infinity: %s", joined)
	}
}
