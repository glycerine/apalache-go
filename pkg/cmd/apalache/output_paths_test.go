package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareCommandOutputPathsMirrorsScalaLayout(t *testing.T) {
	root := t.TempDir()
	outDir := filepath.Join(root, "out")
	customRunDir := filepath.Join(root, "custom")
	paths, err := prepareCommandOutputPaths(outDir, customRunDir, filepath.Join(root, "Model.tla"), "check")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(paths.runDir) != filepath.Join(outDir, "Model.tla") {
		t.Fatalf("generated run directory = %q", paths.runDir)
	}
	if info, statErr := os.Stat(paths.runDir); statErr != nil || !info.IsDir() {
		t.Fatalf("generated run directory: %v", statErr)
	}
	absoluteCustom, err := filepath.Abs(customRunDir)
	if err != nil {
		t.Fatal(err)
	}
	if paths.customRunDir != absoluteCustom {
		t.Fatalf("custom run directory = %q, want %q", paths.customRunDir, absoluteCustom)
	}
	files := paths.implicitFiles("example.itf.json")
	if len(files) != 2 || files[0] != filepath.Join(paths.runDir, "example.itf.json") ||
		files[1] != filepath.Join(absoluteCustom, "example.itf.json") {
		t.Fatalf("implicit output files = %v", files)
	}
}

func TestResolveOutputDirectoryOptionsPrecedence(t *testing.T) {
	t.Setenv("OUT_DIR", "environment-out")
	t.Setenv("RUN_DIR", "environment-run")
	jsonOut, jsonRun := "json-out", "json-run"
	config := jsonCommandConfig{OutDir: &jsonOut, RunDir: &jsonRun}
	if err := config.validateForCheck(); err != nil {
		t.Fatalf("check JSON output directories = %v", err)
	}
	if err := config.validateForSimulate(); err != nil {
		t.Fatalf("simulate JSON output directories = %v", err)
	}
	if err := config.validateForModelCommand("test", false); err == nil {
		t.Fatal("test JSON output directories should remain unsupported until the command writes run output")
	}
	outDir, runDir := "cli-out", "cli-run"
	resolveOutputDirectoryOptions(config, map[string]bool{"out-dir": true, "run-dir": true}, &outDir, &runDir)
	if outDir != "cli-out" || runDir != "cli-run" {
		t.Fatalf("CLI precedence = (%q, %q)", outDir, runDir)
	}
	outDir, runDir = "", ""
	resolveOutputDirectoryOptions(config, map[string]bool{}, &outDir, &runDir)
	if outDir != jsonOut || runDir != jsonRun {
		t.Fatalf("JSON precedence = (%q, %q)", outDir, runDir)
	}
	config = jsonCommandConfig{}
	outDir, runDir = "", ""
	resolveOutputDirectoryOptions(config, map[string]bool{}, &outDir, &runDir)
	if outDir != "environment-out" || runDir != "environment-run" {
		t.Fatalf("environment fallback = (%q, %q)", outDir, runDir)
	}
}
