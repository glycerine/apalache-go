//go:build corpus

package apalache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAnnotatedTopLevelScalaFixtureCorpus(t *testing.T) {
	expectedFailures := map[string]string{
		"Bug3400.tla":             "undefined identifier IndInv1",
		"Bug832.tla":              "Parser error in type annotation of Example: '->' expected but ) found",
		"Bug860.tla":              "Parser error in type annotation of Op: '=>' expected but -> found",
		"Bug914.tla":              "Arguments to = should have the same type",
		"Bug925.tla":              "Error when computing the type of Optional",
		"FormulaRefs.tla":         "subformula references are not supported: A!1",
		"ModelValFail.tla":        "types UT, Str",
		"TestAliasNewMissing.tla": "Missing @typeAlias",
		"UntypedConst.tla":        "type annotation for CONSTANT",
		"UntypedVar.tla":          "type annotation for VARIABLE",
	}
	paths, err := filepath.Glob(filepath.Join("..", "test", "tla", "*.tla"))
	if err != nil {
		t.Fatal(err)
	}
	seenFailures := make(map[string]struct{}, len(expectedFailures))
	checked := 0
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(data), "@type") {
			continue
		}
		name := filepath.Base(path)
		if name == "PolyExp.tla" {
			// This fixture deliberately makes let-polymorphism inference grow
			// exponentially and belongs in a separately budgeted stress gate.
			continue
		}
		checked++
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result := ParseAndTypeCheckFile(ctx, path, FrontendOptions{}, TypeCheckOptions{})
			wantDiagnostic, shouldFail := expectedFailures[name]
			if !shouldFail {
				if !result.OK() {
					t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
				}
				return
			}
			seenFailures[name] = struct{}{}
			if result.OK() {
				t.Fatal("negative Scala fixture unexpectedly typechecked")
			}
			text := make([]string, len(result.Diagnostics))
			for index, diagnostic := range result.Diagnostics {
				text[index] = diagnostic.Error()
			}
			if !strings.Contains(strings.Join(text, "\n"), wantDiagnostic) {
				t.Fatalf("diagnostics %v do not contain %q", result.Diagnostics, wantDiagnostic)
			}
		})
	}
	if checked < 220 {
		t.Fatalf("annotated corpus unexpectedly shrank to %d fixtures", checked)
	}
	for name := range expectedFailures {
		if _, seen := seenFailures[name]; !seen {
			t.Errorf("expected negative fixture %s was not present", name)
		}
	}
}

func TestTopLevelScalaSyntaxCorpus(t *testing.T) {
	expectedFailures := map[string]string{
		"Bug3400.tla":        "undefined identifier IndInv1",
		"FoldDefined.tla":    "expected EXCEPT component",
		"FormulaRefs.tla":    "subformula references are not supported: A!1",
		"NonFoldDefined.tla": "undefined operator +",
		"typing1.tla":        "Cannot find source file for module Typing",
	}
	paths, err := filepath.Glob(filepath.Join("..", "test", "tla", "*.tla"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 260 {
		t.Fatalf("syntax corpus unexpectedly shrank to %d fixtures", len(paths))
	}
	seenFailures := make(map[string]struct{}, len(expectedFailures))
	for _, path := range paths {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			result := ParseFile(ctx, path, FrontendOptions{})
			name := filepath.Base(path)
			wantDiagnostic, shouldFail := expectedFailures[name]
			if !shouldFail {
				if !result.OK() {
					t.Fatalf("unexpected diagnostics: %v", result.Diagnostics)
				}
				return
			}
			seenFailures[name] = struct{}{}
			if result.OK() {
				t.Fatal("negative Scala fixture unexpectedly parsed")
			}
			text := make([]string, len(result.Diagnostics))
			for index, diagnostic := range result.Diagnostics {
				text[index] = diagnostic.Error()
			}
			if !strings.Contains(strings.Join(text, "\n"), wantDiagnostic) {
				t.Fatalf("diagnostics %v do not contain %q", result.Diagnostics, wantDiagnostic)
			}
		})
	}
	for name := range expectedFailures {
		if _, seen := seenFailures[name]; !seen {
			t.Errorf("expected negative fixture %s was not present", name)
		}
	}
}
