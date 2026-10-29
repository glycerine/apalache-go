package apalache

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVerificationInventoryNamesExistingTests(t *testing.T) {
	functions := make(map[string]string)
	tagged := make(map[string]struct{})
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		text := string(data)
		for _, match := range regexp.MustCompile(`(?m)^func ((?:Test|Fuzz)[A-Za-z0-9_]+)\(`).FindAllStringSubmatch(text, -1) {
			if previous, duplicate := functions[match[1]]; duplicate {
				t.Errorf("test function %s is duplicated in %s and %s", match[1], previous, path)
			}
			functions[match[1]] = path
		}
		if match := regexp.MustCompile(`(?m)^//go:build (oracle|solver|corpus|release|deterministic|resource)$`).FindStringSubmatch(text); match != nil {
			for _, function := range regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`).FindAllStringSubmatch(text, -1) {
				tagged[function[1]] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	file, err := os.Open("TEST_INVENTORY.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	inventoried := make(map[string]int)
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			if scanner.Text() != "class\tgate\tevidence\tguarantee\tknown_limit" {
				t.Fatalf("inventory header = %q", scanner.Text())
			}
			continue
		}
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 5 || fields[0] == "" || fields[1] == "" || fields[2] == "" || fields[3] == "" || fields[4] == "" {
			t.Errorf("inventory line %d must contain five nonempty fields", line)
			continue
		}
		for _, name := range strings.Split(fields[2], ";") {
			if _, found := functions[name]; !found {
				t.Errorf("inventory line %d names missing test %s", line, name)
			}
			inventoried[name]++
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for name, count := range inventoried {
		if count != 1 {
			t.Errorf("inventory names %s %d times", name, count)
		}
	}
	for name := range functions {
		if strings.HasPrefix(name, "Fuzz") && inventoried[name] != 1 {
			t.Errorf("fuzz function %s is not inventoried", name)
		}
	}
	for name := range tagged {
		if inventoried[name] != 1 {
			t.Errorf("tagged test %s is not inventoried", name)
		}
	}
}
