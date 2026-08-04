package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"danmo-work/core/domain"
)

func TestResolveConnectorDepsScriptConvention(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix convention")
	}
	pkg := t.TempDir()
	deps := filepath.Join(pkg, "deps")
	if err := os.MkdirAll(deps, 0o755); err != nil {
		t.Fatal(err)
	}
	rel := "deps/" + runtime.GOOS + ".sh"
	absWant := filepath.Join(pkg, filepath.FromSlash(rel))
	if err := os.WriteFile(absWant, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	gotRel, gotAbs, ok, err := ResolveConnectorDepsScript(pkg, nil)
	if err != nil || !ok {
		t.Fatalf("resolve: ok=%v err=%v", ok, err)
	}
	if gotRel != rel {
		t.Fatalf("rel=%q want %q", gotRel, rel)
	}
	if gotAbs != absWant {
		t.Fatalf("abs=%q want %q", gotAbs, absWant)
	}
}

func TestResolveConnectorDepsScriptMissingPlatformFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix convention")
	}
	pkg := t.TempDir()
	if err := os.MkdirAll(filepath.Join(pkg, "deps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Only write the "other" platform script.
	other := "linux"
	if runtime.GOOS == "linux" {
		other = "darwin"
	}
	if err := os.WriteFile(filepath.Join(pkg, "deps", other+".sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, ok, err := ResolveConnectorDepsScript(pkg, nil)
	if ok || err == nil {
		t.Fatal("expected missing platform error")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveConnectorDepsScriptNoDepsSkips(t *testing.T) {
	pkg := t.TempDir()
	_, _, ok, err := ResolveConnectorDepsScript(pkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected skip")
	}
}

func TestRunConnectorDepsScriptWritesHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash fixture")
	}
	home := t.TempDir()
	prev := marketDepsHome
	marketDepsHome = func() string { return home }
	t.Cleanup(func() { marketDepsHome = prev })

	pkg := t.TempDir()
	script := filepath.Join(pkg, "deps", "run.sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nmkdir -p \"$DANMO_HOME/bin\"\necho hi > \"$DANMO_HOME/bin/marker\"\necho ran-$CONNECTOR_ID\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	logOut, err := RunConnectorDepsScript(context.Background(), pkg, script, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logOut, "ran-demo") {
		t.Fatalf("log=%q", logOut)
	}
	b, err := os.ReadFile(filepath.Join(home, "bin", "marker"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "hi" {
		t.Fatalf("marker=%q", b)
	}
}

func TestResolveConnectorUninstallScriptOptional(t *testing.T) {
	pkg := t.TempDir()
	_, _, ok, err := ResolveConnectorUninstallScript(pkg, nil)
	if err != nil || ok {
		t.Fatalf("expected skip, ok=%v err=%v", ok, err)
	}
}

func TestRunConnectorUninstallScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash fixture")
	}
	home := t.TempDir()
	prev := marketDepsHome
	marketDepsHome = func() string { return home }
	t.Cleanup(func() { marketDepsHome = prev })

	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(bin, "codegraph")
	if err := os.WriteFile(marker, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	pkg := t.TempDir()
	script := filepath.Join(pkg, "deps", "uninstall-"+runtime.GOOS+".sh")
	if err := os.MkdirAll(filepath.Dir(script), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\nrm -f \"$DANMO_HOME/bin/codegraph\"\necho cleaned\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	item := domain.MarketItem{ID: "cg", UninstallDeps: map[string]string{runtime.GOOS: "deps/uninstall-" + runtime.GOOS + ".sh"}}
	rel, logOut, err := RunConnectorUninstallForPackage(context.Background(), pkg, item)
	if err != nil {
		t.Fatal(err)
	}
	if rel == "" || !strings.Contains(logOut, "cleaned") {
		t.Fatalf("rel=%q log=%q", rel, logOut)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("marker should be removed")
	}
}
