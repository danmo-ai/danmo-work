package container

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestPrepLoadTarPlainPassthrough(t *testing.T) {
	p, tmp, err := prepLoadTar("/tmp/danmo-work-env-linux-amd64.tar")
	if err != nil || p != "/tmp/danmo-work-env-linux-amd64.tar" || tmp != "" {
		t.Fatalf("p=%q tmp=%q err=%v", p, tmp, err)
	}
}

func TestPrepLoadTarGunzip(t *testing.T) {
	payload := []byte("docker-archive-bytes")
	src := filepath.Join(t.TempDir(), "env.tar.gz")
	f, err := os.Create(src)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	if _, err := zw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	p, tmp, err := prepLoadTar(src)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp)
	if p != tmp {
		t.Fatalf("load path %q should be temp path %q", p, tmp)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q", got)
	}
}

func TestPrepLoadTarBadGzip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "env.tar.gz")
	if err := os.WriteFile(src, []byte("not gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepLoadTar(src); err == nil {
		t.Fatal("expected error for non-gzip input")
	}
}

func TestResolveTarPathGzipAsset(t *testing.T) {
	t.Setenv("WORK_ENV_TAR", "")
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	envDir := filepath.Join(root, "out", "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(envDir, "danmo-work-env-linux-"+LinuxArch()+".tar.gz")
	if err := os.WriteFile(p, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DQ_ROOT", root)
	if got := ResolveTarPath(""); got != p {
		t.Fatalf("got %q want %q", got, p)
	}
}
