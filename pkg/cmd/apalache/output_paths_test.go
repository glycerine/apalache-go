package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
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
	intermediates, err := paths.intermediateFiles("00_OutSanyParser.tla")
	if err != nil {
		t.Fatal(err)
	}
	if len(intermediates) != 2 || intermediates[0] != filepath.Join(paths.runDir, "intermediate", "00_OutSanyParser.tla") ||
		intermediates[1] != filepath.Join(absoluteCustom, "intermediate", "00_OutSanyParser.tla") {
		t.Fatalf("intermediate output files = %v", intermediates)
	}
	for _, path := range intermediates {
		if info, err := os.Stat(filepath.Dir(path)); err != nil || !info.IsDir() {
			t.Fatalf("intermediate directory for %s = %v, %v", path, info, err)
		}
	}
	if err := paths.writeRunFile("check", []string{"--length=3", "Model.tla"}); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{paths.runDir, absoluteCustom} {
		data, err := os.ReadFile(filepath.Join(directory, "run.txt"))
		if err != nil || string(data) != "check --length=3 Model.tla\n" {
			t.Fatalf("run metadata in %s = %q, %v", directory, data, err)
		}
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
	if err := config.validateForFrontend("transpile"); err != nil {
		t.Fatalf("transpile JSON output directories = %v", err)
	}
	if err := config.validateForTracee(); err != nil {
		t.Fatalf("tracee JSON output directories = %v", err)
	}
	if err := config.validateForFrontend("parse"); err != nil {
		t.Fatalf("parse JSON output directories = %v", err)
	}
	if err := config.validateForFrontend("typecheck"); err != nil {
		t.Fatalf("typecheck JSON output directories = %v", err)
	}
	if err := config.validateForModelCommand("test", false); err != nil {
		t.Fatalf("test JSON output directories = %v", err)
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

func TestCommonOutputOptionsPrecedenceAndEnvironmentValidation(t *testing.T) {
	jsonDebug, jsonIntermediate := true, false
	config := jsonCommandConfig{Debug: &jsonDebug, WriteIntermediate: &jsonIntermediate}
	flags := flag.NewFlagSet("common-output", flag.ContinueOnError)
	common := addCommonOutputFlags(flags)
	if err := flags.Parse([]string{"--debug=false", "--write-intermediate=yes"}); err != nil {
		t.Fatal(err)
	}
	visited := make(map[string]bool)
	flags.Visit(func(flag *flag.Flag) { visited[flag.Name] = true })
	resolved, err := common.resolve(config, visited)
	if err != nil || resolved.debug || !resolved.writeIntermediate {
		t.Fatalf("CLI common output options = %#v, %v", resolved, err)
	}

	t.Setenv("WRITE_INTERMEDIATE", "no")
	resolved, err = (&commonOutputFlags{}).resolve(jsonCommandConfig{}, map[string]bool{})
	if err != nil || resolved.writeIntermediate {
		t.Fatalf("environment common output options = %#v, %v", resolved, err)
	}
	t.Setenv("WRITE_INTERMEDIATE", "sometimes")
	if _, err = (&commonOutputFlags{}).resolve(jsonCommandConfig{}, map[string]bool{}); err == nil ||
		!strings.Contains(err.Error(), "invalid WRITE_INTERMEDIATE") {
		t.Fatalf("invalid environment common output option = %v", err)
	}
}

func TestTypecheckWritesDebugConfigurationAndFrontendIntermediates(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	source := filepath.Join(directory, "CommonOutput.tla")
	text := "---- MODULE CommonOutput ----\nEXTENDS Integers\n\\* @type: Int;\nVARIABLE x\nInit == x = 0\n====\n"
	if err := os.WriteFile(source, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(directory, "run")
	var stdout, stderr strings.Builder
	code := run(t.Context(), []string{
		"typecheck", "--run-dir=" + runDir, "--debug", "--write-intermediate=1", source,
	}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("typecheck = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(filepath.Join(runDir, "application-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err = json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["command"] != "typecheck" || snapshot["debug"] != true || snapshot["write-intermediate"] != true ||
		snapshot["run-dir"] != runDir {
		t.Fatalf("application configuration = %#v", snapshot)
	}
	if snapshot["output"] != "summary" || len(snapshot["features"].([]any)) != 0 ||
		snapshot["typechecker"].(map[string]any)["infer-poly"] != true {
		t.Fatalf("effective typecheck configuration = %#v", snapshot)
	}
	for _, name := range []string{"00_OutFrontend.tla", "00_OutFrontend.json"} {
		if info, statErr := os.Stat(filepath.Join(runDir, "intermediate", name)); statErr != nil || info.Size() == 0 {
			t.Fatalf("frontend intermediate %s = %v, %v", name, info, statErr)
		}
	}
}
