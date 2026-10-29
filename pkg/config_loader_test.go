package apalache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestApalacheConfigLoaderSelectionAndPrecedence(t *testing.T) {
	work, home := configLoaderDirectories(t)
	writeConfigTestFile(t, filepath.Join(work, LocalConfigFilename), "not json")
	writeGlobalConfigTestFile(t, home, "not json")
	explicit := filepath.Join(work, "selected.json")
	writeConfigTestFile(t, explicit, `{"out-dir":"selected","checker":{"length":2,"tuning":{"selected":"1","shared":"selected"}}}`)
	primary := ParseApalacheConfigJSON(`{"checker":{"length":3,"tuning":{"primary":"1","shared":"primary"}}}`).RequireValue()
	primary = primary.WithCommand(ConfigCheck)
	primary.Context.ConfigFile = &explicit

	result := NewApalacheConfigLoader(work, home).Load(primary)
	if !result.IsSuccess() {
		t.Fatal(result.Errors())
	}
	merged := result.RequireValue()
	wantTuning := map[string]string{"selected": "1", "primary": "1", "shared": "primary"}
	if merged.Common.OutDir == nil || *merged.Common.OutDir != "selected" || merged.Checker.Length == nil ||
		*merged.Checker.Length != 3 || merged.Checker.Tuning == nil || !reflect.DeepEqual(*merged.Checker.Tuning, wantTuning) {
		t.Fatalf("merged = %#v", merged)
	}
}

func TestApalacheConfigLoaderDiscoveryDoesNotTraverseOrFallBack(t *testing.T) {
	work, home := configLoaderDirectories(t)
	child := filepath.Join(work, "child")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfigTestFile(t, filepath.Join(work, LocalConfigFilename), `{"run-dir":"parent-run"}`)
	writeGlobalConfigTestFile(t, home, `{"run-dir":"global-run"}`)
	result := NewApalacheConfigLoader(child, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
	if !result.IsSuccess() || result.RequireValue().Common.RunDir == nil || *result.RequireValue().Common.RunDir != "global-run" {
		value, _ := result.Value()
		t.Fatalf("global discovery = %#v, %#v", result.Errors(), value)
	}

	writeConfigTestFile(t, filepath.Join(child, LocalConfigFilename), `{"features":["invalid-feature"]}`)
	result = NewApalacheConfigLoader(child, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
	want := []string{LocalConfigFilename + ": $.features: Unexpected feature: invalid-feature"}
	if result.IsSuccess() || !reflect.DeepEqual(result.Errors(), want) {
		t.Fatalf("local failure = %#v", result.Errors())
	}
}

func TestApalacheConfigLoaderDiscoveryCandidateIsolation(t *testing.T) {
	t.Run("local shields invalid global", func(t *testing.T) {
		work, home := configLoaderDirectories(t)
		writeConfigTestFile(t, filepath.Join(work, LocalConfigFilename), `{"run-dir":"local-run"}`)
		writeGlobalConfigTestFile(t, home, "not json")
		result := NewApalacheConfigLoader(work, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
		if !result.IsSuccess() || result.RequireValue().Common.RunDir == nil ||
			*result.RequireValue().Common.RunDir != "local-run" {
			t.Fatalf("local selection = %#v", result.Errors())
		}
	})

	t.Run("invalid selected global is reported", func(t *testing.T) {
		work, home := configLoaderDirectories(t)
		writeGlobalConfigTestFile(t, home, "not json")
		result := NewApalacheConfigLoader(work, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
		if result.IsSuccess() || len(result.Errors()) == 0 ||
			!strings.Contains(result.Errors()[0], TLAPlusConfigDirectory) {
			t.Fatalf("invalid global = %#v", result.Errors())
		}
	})

	t.Run("legacy global filename is ignored", func(t *testing.T) {
		work, home := configLoaderDirectories(t)
		globalDirectory := filepath.Join(home, TLAPlusConfigDirectory)
		if err := os.MkdirAll(globalDirectory, 0o755); err != nil {
			t.Fatal(err)
		}
		writeConfigTestFile(t, filepath.Join(globalDirectory, "apalache.cfg"), "not json")
		writeGlobalConfigTestFile(t, home, `{"run-dir":"global-run"}`)
		result := NewApalacheConfigLoader(work, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
		if !result.IsSuccess() || result.RequireValue().Common.RunDir == nil ||
			*result.RequireValue().Common.RunDir != "global-run" {
			t.Fatalf("global JSON selection = %#v", result.Errors())
		}
	})
}

func TestApalacheConfigLoaderNoSelectionAndMissingExplicit(t *testing.T) {
	work, home := configLoaderDirectories(t)
	primary := EmptyApalacheConfig().WithCommand(ConfigCheck)
	result := NewApalacheConfigLoader(work, home).Load(primary)
	if !result.IsSuccess() || !reflect.DeepEqual(result.RequireValue(), primary) {
		value, _ := result.Value()
		t.Fatalf("no selection = %#v, %#v", result.Errors(), value)
	}

	missing := filepath.Join(work, "missing.json")
	primary.Context.ConfigFile = &missing
	result = NewApalacheConfigLoader(work, home).Load(primary)
	if result.IsSuccess() || len(result.Errors()) != 1 || !strings.Contains(result.Errors()[0], "Configuration file not found") ||
		!strings.Contains(result.Errors()[0], "missing.json") {
		t.Fatalf("missing explicit = %#v", result.Errors())
	}
}

func TestApalacheConfigLoaderLegacyExplicitAndDiscovery(t *testing.T) {
	work, home := configLoaderDirectories(t)
	writeConfigTestFile(t, filepath.Join(work, ".apalache.cfg"), "not json")
	writeConfigTestFile(t, filepath.Join(work, LocalConfigFilename), `{"run-dir":"local-run"}`)
	result := NewApalacheConfigLoader(work, home).Load(EmptyApalacheConfig())
	if !result.IsSuccess() || *result.RequireValue().Common.RunDir != "local-run" {
		t.Fatalf("legacy discovery = %#v", result.Errors())
	}

	legacy := filepath.Join(work, "selected.cfg")
	writeConfigTestFile(t, legacy, `{}`)
	primary := ApalacheConfig{Context: RunContextPatch{ConfigFile: &legacy}}
	result = NewApalacheConfigLoader(work, home).Load(primary)
	if result.IsSuccess() || len(result.Errors()) != 1 || !strings.Contains(result.Errors()[0], "selected.cfg") ||
		!strings.Contains(result.Errors()[0], "selected.json") || !strings.Contains(result.Errors()[0], "Rename it") {
		t.Fatalf("legacy explicit = %#v", result.Errors())
	}
	replacement := filepath.Join(work, "selected.json")
	writeConfigTestFile(t, replacement, `{}`)
	result = NewApalacheConfigLoader(work, home).Load(primary)
	if result.IsSuccess() || !strings.Contains(result.Errors()[0], "Remove it; use") {
		t.Fatalf("legacy replacement = %#v", result.Errors())
	}
}

func TestApalacheConfigLoaderDoesNotRecursivelyLoadConfigFile(t *testing.T) {
	work, home := configLoaderDirectories(t)
	writeConfigTestFile(t, filepath.Join(work, LocalConfigFilename), `{"config-file":"missing.json","run-dir":"local-run"}`)
	result := NewApalacheConfigLoader(work, home).Load(EmptyApalacheConfig().WithCommand(ConfigCheck))
	if !result.IsSuccess() || result.RequireValue().Common.RunDir == nil || *result.RequireValue().Common.RunDir != "local-run" {
		t.Fatalf("nonrecursive load = %#v", result.Errors())
	}
}

func TestApalacheConfigLoaderReadFailure(t *testing.T) {
	work, home := configLoaderDirectories(t)
	primary := EmptyApalacheConfig()
	primary.Context.ConfigFile = &work
	result := NewApalacheConfigLoader(work, home).Load(primary)
	if result.IsSuccess() || len(result.Errors()) != 1 || !strings.Contains(result.Errors()[0], "Could not read configuration") {
		t.Fatalf("read failure = %#v", result.Errors())
	}
}

func configLoaderDirectories(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	work, home := filepath.Join(root, "work"), filepath.Join(root, "home")
	if err := os.Mkdir(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return work, home
}

func writeGlobalConfigTestFile(t *testing.T, home, contents string) {
	t.Helper()
	directory := filepath.Join(home, TLAPlusConfigDirectory)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfigTestFile(t, filepath.Join(directory, GlobalConfigFilename), contents)
}

func writeConfigTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
