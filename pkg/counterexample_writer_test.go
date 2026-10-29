package apalache

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func counterexampleWriterTrace() Trace[Expr] {
	module := NewModule("Root", []Decl{
		NewConstDecl("N", Typed(IntT1)),
		NewVarDecl("x", Typed(IntT1)),
	})
	return NewTrace(module, []TraceState{
		{"N": intLiteral(3)},
		{"x": intLiteral(1)},
		{"x": intLiteral(2)},
	}, [][]string{nil, {"Init"}, {"Next"}}, boolExpr(true))
}

func TestWriteCounterexampleEveryFormat(t *testing.T) {
	trace := counterexampleWriterTrace()
	for _, format := range []string{"tla", "tlc", "json", "itf.json"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			if err := WriteCounterexample(&output, format, trace); err != nil {
				t.Fatal(err)
			}
			switch format {
			case "tla":
				for _, fragment := range []string{
					"EXTENDS Root", "Constant initialization state", "Initial state [Init]", "State1 [Next]",
					"ConstInit == N = 3", "State0 == x = 1", "State1 == x = 2", "InvariantViolation == TRUE",
				} {
					if !strings.Contains(output.String(), fragment) {
						t.Fatalf("TLA output lacks %q:\n%s", fragment, output.String())
					}
				}
			case "tlc":
				if !strings.Contains(output.String(), "1: <Initial predicate>") || !strings.Contains(output.String(), "2: <Next>") {
					t.Fatalf("TLC output = %s", output.String())
				}
			default:
				if !json.Valid(output.Bytes()) {
					t.Fatalf("invalid %s JSON: %s", format, output.String())
				}
				if !strings.Contains(output.String(), "\n  \"") {
					t.Fatalf("%s counterexample is not indented: %s", format, output.String())
				}
			}
		})
	}
	if err := WriteCounterexample(&bytes.Buffer{}, "yaml", trace); err == nil {
		t.Fatal("accepted unknown counterexample format")
	}
	if err := WriteCounterexample(nil, "tla", trace); err == nil {
		t.Fatal("accepted missing output")
	}
}

func TestWriteCounterexampleMatchesScalaTimestampAndProvenance(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("CST", -6*60*60))
	trace := counterexampleWriterTrace()
	var tla bytes.Buffer
	if err := writeCounterexampleAt(&tla, "tla", trace, now); err != nil {
		t.Fatal(err)
	}
	wantSuffix := strings.Repeat("=", 80) + "\n" +
		"(* Created by Apalache on Fri Jan 02 03:04:05 CST 2026 *)\n" +
		"(* https://github.com/apalache-mc/apalache *)\n"
	if !strings.HasSuffix(tla.String(), wantSuffix) {
		t.Fatalf("TLA footer = %q", tla.String())
	}
	var tlc bytes.Buffer
	if err := writeCounterexampleAt(&tlc, "tlc", trace, now); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tlc.String(), "@!@!@STARTMSG 2262:0 @!@!@\n"+
		"Created by Apalache on Fri Jan 02 03:04:05 CST 2026\n") {
		t.Fatalf("TLC timestamp header = %q", tlc.String())
	}
}

func TestWriteCounterexampleAllFormatsMirrorsRunDirectory(t *testing.T) {
	base := t.TempDir()
	mirror := filepath.Join(base, "mirror")
	manager := NewOutputManager()
	if err := manager.Configure(OutputManagerOptions{
		OutDir: base, CustomRunDir: mirror, Command: "check",
		Now: func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
	}); err != nil {
		t.Fatal(err)
	}
	paths, err := WriteCounterexampleAllFormats(manager, "violation", "7", counterexampleWriterTrace())
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 {
		t.Fatalf("written paths = %#v", paths)
	}
	runDir, _ := manager.RunDirPath()
	for _, name := range []string{"violation7.tla", "MCviolation7.out", "violation7.json", "violation7.itf.json"} {
		for _, directory := range []string{runDir, mirror} {
			if _, err = os.Stat(filepath.Join(directory, name)); err != nil {
				t.Fatalf("missing %s mirror in %s: %v", name, directory, err)
			}
		}
	}
}
