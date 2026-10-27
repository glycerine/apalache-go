package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Tiny.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Tiny ----
Value == 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", path}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if got, want := stdout.String(), "parsed module Tiny\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunParseJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Tiny.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Tiny ----\nValue == 1\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", "-output=json", path}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), `"name": "ApalacheIR"`) {
		t.Fatalf("stdout does not contain ApalacheIR: %s", &stdout)
	}
}

func TestRunParseFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Bad.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Bad ----\nValue == missing\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"parse", path}, &stdout, &stderr)
	if code != exitFrontend {
		t.Fatalf("run() = %d, want %d; stderr = %s", code, exitFrontend, &stderr)
	}
	if stderr.Len() == 0 {
		t.Fatal("run() emitted no diagnostic")
	}
}

func TestRunTypeCheckAndTypedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Typed.tla")
	if err := os.WriteFile(path, []byte(`---- MODULE Typed ----
EXTENDS Integers
\* @type: Int;
CONSTANT N
Value == N + 1
====
`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"typecheck", path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("run() = %d, stderr = %s", code, &stderr)
	}
	if got, want := stdout.String(), "type-checked module Typed\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(context.Background(), []string{"typecheck", "-output=json", path}, &stdout, &stderr); code != exitOK {
		t.Fatalf("JSON run() = %d, stderr = %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), `"type": "Int"`) || strings.Contains(stdout.String(), `"type": "Untyped"`) {
		t.Fatalf("stdout is not fully typed IR: %s", &stdout)
	}
}

func TestRunTypeCheckReportsMissingAnnotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Untyped.tla")
	if err := os.WriteFile(path, []byte("---- MODULE Untyped ----\nCONSTANT N\nValue == N\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"typecheck", path}, &stdout, &stderr)
	if code != exitFrontend || !strings.Contains(stderr.String(), "expected a type annotation for CONSTANT N") {
		t.Fatalf("run() = %d, stderr = %q", code, &stderr)
	}
}

func TestRunUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr); code != exitUsage {
		t.Fatalf("run() = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("stderr = %q", &stderr)
	}
}
