package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type commandOutputPaths struct {
	runDir       string
	customRunDir string
}

func prepareCommandOutputPaths(outDir, customRunDir, sourcePath, command string) (commandOutputPaths, error) {
	paths := commandOutputPaths{}
	if outDir != "" {
		namespace := command
		if sourcePath != "" {
			namespace = filepath.Base(sourcePath)
		}
		namespaceDir := filepath.Join(outDir, namespace)
		if err := os.MkdirAll(namespaceDir, 0o755); err != nil {
			return commandOutputPaths{}, fmt.Errorf("create output directory %s: %w", namespaceDir, err)
		}
		prefix := time.Now().Format("2006-01-02T15-04-05") + "_"
		runDir, err := os.MkdirTemp(namespaceDir, prefix)
		if err != nil {
			return commandOutputPaths{}, fmt.Errorf("create run directory in %s: %w", namespaceDir, err)
		}
		paths.runDir = runDir
	}
	if customRunDir != "" {
		if err := os.MkdirAll(customRunDir, 0o755); err != nil {
			return commandOutputPaths{}, fmt.Errorf("create custom run directory %s: %w", customRunDir, err)
		}
		absolute, err := filepath.Abs(customRunDir)
		if err != nil {
			return commandOutputPaths{}, fmt.Errorf("resolve custom run directory %s: %w", customRunDir, err)
		}
		paths.customRunDir = absolute
	}
	return paths, nil
}

func (paths commandOutputPaths) implicitFiles(name string) []string {
	files := make([]string, 0, 2)
	if paths.runDir != "" {
		files = append(files, filepath.Join(paths.runDir, name))
	}
	if paths.customRunDir != "" && paths.customRunDir != paths.runDir {
		files = append(files, filepath.Join(paths.customRunDir, name))
	}
	if len(files) == 0 {
		files = append(files, name)
	}
	return files
}

func (paths commandOutputPaths) intermediateFiles(name string) ([]string, error) {
	directories := make([]string, 0, 2)
	if paths.runDir != "" {
		directories = append(directories, filepath.Join(paths.runDir, "intermediate"))
	}
	if paths.customRunDir != "" && paths.customRunDir != paths.runDir {
		directories = append(directories, filepath.Join(paths.customRunDir, "intermediate"))
	}
	if len(directories) == 0 {
		directories = append(directories, "intermediate")
	}
	files := make([]string, len(directories))
	for index, directory := range directories {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, fmt.Errorf("create intermediate output directory %s: %w", directory, err)
		}
		files[index] = filepath.Join(directory, name)
	}
	return files, nil
}

func (paths commandOutputPaths) writeRunFile(command string, args []string) error {
	directories := make([]string, 0, 2)
	if paths.runDir != "" {
		directories = append(directories, paths.runDir)
	}
	if paths.customRunDir != "" && paths.customRunDir != paths.runDir {
		directories = append(directories, paths.customRunDir)
	}
	line := strings.Join(append([]string{command}, args...), " ") + "\n"
	for _, directory := range directories {
		if err := os.WriteFile(filepath.Join(directory, "run.txt"), []byte(line), 0o644); err != nil {
			return fmt.Errorf("write run metadata in %s: %w", directory, err)
		}
	}
	return nil
}

func resolveOutputDirectoryOptions(
	config jsonCommandConfig,
	visited map[string]bool,
	outDir, runDir *string,
) {
	if !visited["out-dir"] {
		if config.OutDir != nil {
			*outDir = *config.OutDir
		} else if value, found := os.LookupEnv("OUT_DIR"); found {
			*outDir = value
		}
	}
	if !visited["run-dir"] {
		if config.RunDir != nil {
			*runDir = *config.RunDir
		} else if value, found := os.LookupEnv("RUN_DIR"); found {
			*runDir = value
		}
	}
}
