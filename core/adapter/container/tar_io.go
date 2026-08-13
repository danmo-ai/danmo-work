package container

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"
)

// prepLoadTar returns a path suitable for engine load. Plain docker-archive
// tars pass through; gzipped release assets (.tar.gz) are decompressed to a
// temp file first so all engines (docker/podman/apple-container) load the
// same plain format. Callers should remove the returned temp path after load.
func prepLoadTar(tarPath string) (loadPath, tempPath string, err error) {
	if !strings.HasSuffix(strings.ToLower(tarPath), ".gz") {
		return tarPath, "", nil
	}
	src, err := os.Open(tarPath)
	if err != nil {
		return "", "", fmt.Errorf("container: open env tar: %w", err)
	}
	defer src.Close()
	zr, err := gzip.NewReader(src)
	if err != nil {
		return "", "", fmt.Errorf("container: gunzip env tar: %w", err)
	}
	defer zr.Close()
	dst, err := os.CreateTemp("", "danmo-work-env-*.tar")
	if err != nil {
		return "", "", err
	}
	name := dst.Name()
	if _, err := io.Copy(dst, zr); err != nil {
		dst.Close()
		os.Remove(name)
		return "", "", fmt.Errorf("container: gunzip env tar: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(name)
		return "", "", err
	}
	return name, name, nil
}
