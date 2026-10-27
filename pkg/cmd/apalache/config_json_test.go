package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCommandConfigIsStrict(t *testing.T) {
	directory := t.TempDir()
	tests := []struct {
		name, document, want string
	}{
		{"unknown key", `{"checker":{"length":1,"mystery":true}}`, "unknown field"},
		{"duplicate key", `{"checker":{"length":1,"length":2}}`, "duplicate key"},
		{"trailing value", `{"checker":{"length":1}} {}`, "trailing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(directory, strings.ReplaceAll(test.name, " ", "-")+".json")
			if err := os.WriteFile(path, []byte(test.document), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := loadCommandConfig(path); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("loadCommandConfig error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunCheckLoadsJSONConfigurationWithCLIPrecedence(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", filepath.Join(directory, "home"))
	t.Chdir(directory)
	if err := os.WriteFile("Counter.tla", []byte(`---- MODULE Counter ----
EXTENDS Integers
\* @type: Int;
VARIABLE x
Init == x = 0
Next == x' = x + 1
Inv == x < 2
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	configuration := `{
  "source":"Counter.tla",
  "checker":{"length":2,"inv":["Inv"],"no-deadlock":true},
  "typechecker":{"infer-poly":true}
}`
	if err := os.WriteFile(".apalache.json", []byte(configuration), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"check"}, &stdout, &stderr); code != exitCounterexample ||
		!strings.Contains(stdout.String(), "Invariant Inv violated at depth 2") {
		t.Fatalf("configured check = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}

	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"check", "--length=1"}, &stdout, &stderr); code != exitOK ||
		!strings.Contains(stdout.String(), "through depth 1") {
		t.Fatalf("CLI override = %d, stdout = %q, stderr = %q", code, &stdout, &stderr)
	}
}

func TestRunCheckRejectsLegacyExplicitConfiguration(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"check", "--config-file=.apalache.cfg", "Spec.tla"}, &stdout, &stderr)
	if code != exitUsage || !strings.Contains(stderr.String(), "legacy Apalache configuration") {
		t.Fatalf("legacy config = %d, stderr = %q", code, &stderr)
	}
}
