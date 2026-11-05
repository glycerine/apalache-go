package apalache

import "testing"

// TestRouteG3TextOptionsKeepsZeroAndOneAsIntegers pins the Scala UnsignedInt
// behavior: z3.pp.max_depth=0 is an integer, not a Boolean.
func TestRouteG3TextOptionsKeepsZeroAndOneAsIntegers(t *testing.T) {
	options := routeG3TextOptions(nil, map[string]string{
		"z3.pp.max_depth": "0",
		"z3.one":          "1",
		"z3.five":         "5",
		"z3.yes":          "true",
		"z3.no":           "false",
		"z3.ratio":        "0.5",
		"z3.name":         "auto",
	})
	if got := options.uints["z3.pp.max_depth"]; got != 0 {
		t.Fatalf("max_depth=0 routed to uints = %d, want 0", got)
	}
	if got := options.uints["z3.one"]; got != 1 {
		t.Fatalf("one routed to uints = %d, want 1", got)
	}
	if got := options.uints["z3.five"]; got != 5 {
		t.Fatalf("five routed to uints = %d, want 5", got)
	}
	if _, ok := options.bools["z3.max_depth"]; ok {
		t.Fatal("numeric value was routed to bools")
	}
	if !options.bools["z3.yes"] || options.bools["z3.no"] {
		t.Fatalf("true/false routed wrong: %#v", options.bools)
	}
	if _, ok := options.bools["z3.one"]; ok {
		t.Fatal("\"1\" was routed to bools, not uints")
	}
	if options.doubles["z3.ratio"] != 0.5 {
		t.Fatalf("ratio routed wrong: %#v", options.doubles)
	}
	if options.symbols["z3.name"] != "auto" {
		t.Fatalf("symbol routed wrong: %#v", options.symbols)
	}
}
