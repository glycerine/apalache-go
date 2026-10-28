package apalache

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScalaPortManifestCoversEveryProductionSource(t *testing.T) {
	repository := ".."
	if _, err := os.Stat(filepath.Join(repository, "build.sbt")); os.IsNotExist(err) {
		t.Skip("Scala source checkout is not available")
	} else if err != nil {
		t.Fatal(err)
	}
	expected := make(map[string]struct{})
	err := filepath.WalkDir(repository, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".scala") || !strings.Contains(filepath.ToSlash(path), "/src/main/scala/") {
			return nil
		}
		relative, err := filepath.Rel(repository, path)
		if err != nil {
			return err
		}
		expected[filepath.ToSlash(relative)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := os.Open("SCALA_PORT_MANIFEST.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer manifest.Close()
	seen := make(map[string]struct{})
	validStatus := map[string]struct{}{"partial": {}, "dependency": {}, "open": {}, "ported": {}, "not-applicable": {}}
	scanner := bufio.NewScanner(manifest)
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			if scanner.Text() != "scala_path\tstatus\tgo_evidence\tremaining" {
				t.Fatalf("manifest header = %q", scanner.Text())
			}
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 4 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[3] == "" {
			t.Fatalf("manifest line %d is incomplete: %q", line, scanner.Text())
		}
		if _, valid := validStatus[fields[1]]; !valid {
			t.Fatalf("manifest line %d has invalid status %q", line, fields[1])
		}
		if _, duplicate := seen[fields[0]]; duplicate {
			t.Fatalf("manifest line %d duplicates %s", line, fields[0])
		}
		seen[fields[0]] = struct{}{}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for path := range expected {
		if _, found := seen[path]; !found {
			t.Errorf("production Scala source is absent from manifest: %s", path)
		}
	}
	for path := range seen {
		if _, found := expected[path]; !found {
			t.Errorf("manifest contains stale production Scala source: %s", path)
		}
	}
	if len(seen) != len(expected) {
		t.Fatalf("manifest entries = %d, production Scala files = %d", len(seen), len(expected))
	}
}
