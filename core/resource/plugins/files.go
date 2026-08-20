package plugins

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sort"
	"strings"
)

// BuiltinPluginNames lists embedded first-party plugin directory names.
func BuiltinPluginNames() []string {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// BuiltinFile is one embedded plugin file (path relative to FS root).
type BuiltinFile struct {
	Path    string
	Content []byte
}

// LoadBuiltinFiles returns every file under the embedded builtin plugins FS.
func LoadBuiltinFiles() ([]BuiltinFile, error) {
	var files []BuiltinFile
	if err := fs.WalkDir(FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(FS, path)
		if err != nil {
			return nil
		}
		files = append(files, BuiltinFile{Path: path, Content: data})
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

// BuiltinContentHash returns a stable SHA256 of every embedded builtin plugin file.
func BuiltinContentHash() (string, error) {
	files, err := LoadBuiltinFiles()
	if err != nil {
		return "", err
	}
	return hashBuiltinFiles(files), nil
}

func hashBuiltinFiles(files []BuiltinFile) string {
	sorted := append([]BuiltinFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	h := sha256.New()
	for _, f := range sorted {
		_, _ = h.Write([]byte(f.Path))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(f.Content)
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// FilesForPlugin returns embedded files whose path is under pluginName/.
func FilesForPlugin(pluginName string) ([]BuiltinFile, error) {
	all, err := LoadBuiltinFiles()
	if err != nil {
		return nil, err
	}
	prefix := pluginName + "/"
	var out []BuiltinFile
	for _, f := range all {
		if f.Path == pluginName || strings.HasPrefix(f.Path, prefix) {
			out = append(out, f)
		}
	}
	return out, nil
}
