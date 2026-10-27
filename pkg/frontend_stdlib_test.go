package apalache

import (
	"context"
	"testing"
)

func TestBundledVariantsAndOptionModulesTypeCheck(t *testing.T) {
	result := ParseAndTypeCheckSource(context.Background(), "BundledModules.tla", `---- MODULE BundledModules ----
EXTENDS Option
\* @type: Some(Int) | None(UNIT);
Value == Some(1)
IsValue == IsSome(Value)
====
`, FrontendOptions{}, TypeCheckOptions{UseRows: true})
	if !result.OK() {
		t.Fatalf("bundled module diagnostics = %v", result.Diagnostics)
	}
}
