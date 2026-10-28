package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportGroupsStatementCoverage(t *testing.T) {
	directory := t.TempDir()
	profile := filepath.Join(directory, "coverage.out")
	output := filepath.Join(directory, "coverage.md")
	data := "mode: set\n" +
		"github.com/glycerine/apalache-go/frontend.go:1.1,2.1 3 1\n" +
		"github.com/glycerine/apalache-go/frontend.go:3.1,4.1 1 0\n" +
		"github.com/glycerine/apalache-go/shai/protocol.pb.go:1.1,2.1 100 0\n" +
		"github.com/glycerine/apalache-go/shai/service.go:1.1,2.1 4 1\n" +
		"github.com/glycerine/apalache-go/smt_g3.go:1.1,2.1 2 1\n"
	if err := os.WriteFile(profile, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := report(profile, output); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, "| Frontend and annotations | 3 | 4 | 75.0% |") ||
		!strings.Contains(text, "| Shai gRPC services | 4 | 4 | 100.0% |") ||
		!strings.Contains(text, "| Symbolic checker | 2 | 2 | 100.0% |") {
		t.Fatalf("coverage report =\n%s", text)
	}
}
