package apalache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportedLETAnnotationsKeepDeclarationIdentity(t *testing.T) {
	dir := t.TempDir()
	helper := `---- MODULE Helper ----
Foo == LET (* @type: Int; *) Local == 1 IN Local
Bar == LET (* @type: Str; *) Local == "s" IN Local
====`
	if err := os.WriteFile(filepath.Join(dir, "Helper.tla"), []byte(helper), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, imports, expr string }{
		{"extends", "EXTENDS Helper", `Foo=1 /\ Bar="s"`},
		{"instance", "H == INSTANCE Helper", `H!Foo=1 /\ H!Bar="s"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE Imported ----\n" + tc.imports + "\nInv == " + tc.expr + "\n===="
			result := ParseAndTypeCheckSource(context.Background(), filepath.Join(dir, "Imported.tla"), source, FrontendOptions{}, TypeCheckOptions{UseRows: true})
			if !result.OK() {
				t.Fatal(result.Diagnostics)
			}
		})
	}
	// Do not obtain a green test merely by dropping the imported annotations.
	if err := os.WriteFile(filepath.Join(dir, "Helper.tla"), []byte(strings.Replace(helper, "@type: Int;", "@type: Bool;", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	result := ParseAndTypeCheckSource(context.Background(), filepath.Join(dir, "Imported.tla"), "---- MODULE Imported ----\nEXTENDS Helper\nInv == Foo=1\n====", FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if result.OK() {
		t.Fatal("inconsistent imported annotation was ignored")
	}

}
