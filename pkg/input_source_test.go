package apalache

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInputFormatAndFileInferenceMatchScala(t *testing.T) {
	for _, test := range []struct {
		name string
		want InputFormat
	}{
		{"Spec.TLA", InputFormatTLA},
		{"trace.ITF.JSON", InputFormatITF},
		{"module.qnt.json", InputFormatQuint},
		{"module.json", InputFormatJSON},
	} {
		source, err := NewFileInputSource(test.name)
		if err != nil || source.Format() != test.want {
			t.Fatalf("NewFileInputSource(%q) = %#v, %v", test.name, source, err)
		}
	}
	if _, err := NewFileInputSource("no-extension"); err == nil || !strings.Contains(err.Error(), "unsupported file format") {
		t.Fatalf("extensionless source error = %v", err)
	}
	for _, value := range []string{"TLA", "json", "ItF", "QNT"} {
		if _, err := ParseInputFormat(value); err != nil {
			t.Fatalf("ParseInputFormat(%q): %v", value, err)
		}
	}
	if _, err := ParseInputFormat("yaml"); err == nil {
		t.Fatal("unsupported input format was accepted")
	}
}

func TestFileAndStringInputSourcesReadAndReportAvailability(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "Spec.tla")
	if err := os.WriteFile(path, []byte("utf-8: λ"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, _ := NewFileInputSource(path)
	if !file.Exists() || file.String() != path {
		t.Fatalf("file source = %#v", file)
	}
	if content, err := file.ReadUTF8(); err != nil || content != "utf-8: λ" {
		t.Fatalf("file content = %q, %v", content, err)
	}
	missing, _ := NewFileInputSource(filepath.Join(directory, "Missing.tla"))
	if missing.Exists() {
		t.Fatal("missing source exists")
	}
	if _, err := missing.ReadUTF8(); err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("missing source error = %v", err)
	}

	auxiliary := []string{"---- MODULE Aux ----"}
	memory := NewStringInputSource("---- MODULE M ----", auxiliary, InputFormatTLA)
	auxiliary[0] = "mutated"
	if !memory.Exists() || memory.String() != "StringSource(tla)" ||
		!reflect.DeepEqual(memory.Auxiliary, []string{"---- MODULE Aux ----"}) {
		t.Fatalf("string source = %#v", memory)
	}
	if content, err := memory.ReadUTF8(); err != nil || content != "---- MODULE M ----" {
		t.Fatalf("string content = %q, %v", content, err)
	}
}
