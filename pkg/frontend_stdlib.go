package apalache

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed stdlib/*.tla
var bundledTLAModules embed.FS

func mountBundledTLAModules(libraryPaths []string) ([]string, func(), error) {
	directory, err := os.MkdirTemp("", "apalache-go-stdlib-")
	if err != nil {
		return nil, nil, fmt.Errorf("create bundled TLA+ library directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(directory) }
	entries, err := bundledTLAModules.ReadDir("stdlib")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("read bundled TLA+ modules: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		contents, readErr := bundledTLAModules.ReadFile("stdlib/" + entry.Name())
		if readErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("read bundled TLA+ module %s: %w", entry.Name(), readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(directory, entry.Name()), contents, 0o600); writeErr != nil {
			cleanup()
			return nil, nil, fmt.Errorf("mount bundled TLA+ module %s: %w", entry.Name(), writeErr)
		}
	}
	paths := append([]string(nil), libraryPaths...)
	paths = append(paths, directory)
	return paths, cleanup, nil
}
