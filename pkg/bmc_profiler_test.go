package apalache

import (
	"bytes"
	"strings"
	"testing"
)

func TestIdleSMTListenerImplementsAllCallbacks(t *testing.T) {
	var listener SMTListener = IdleSMTListener{}
	listener.OnIntroduceSMTConstant("x")
	listener.OnIntroduceCell(42)
	listener.OnSMTAssert(NewNameExpr("p", Untyped()), 100)
}

func TestRuleStatAverageUsesScalaIntegerDivision(t *testing.T) {
	stat := NewRuleStat("R")
	if stat.RuleName != "R" || stat.SMTAssertionSizeAverage() != 0 {
		t.Fatalf("new stat = %+v", stat)
	}
	stat.SMTAssertionsSelf = 3
	stat.SMTAssertionSizeSum = 11
	if stat.SMTAssertionSizeAverage() != 3 {
		t.Fatalf("average = %d", stat.SMTAssertionSizeAverage())
	}
}

func TestRuleStatListenerNestedSelfAttributionAndExpressionSize(t *testing.T) {
	listener := NewRuleStatListener()
	leaf := NewNameExpr("p", Untyped())
	nested := MustOperExpr(Op("AND"), Untyped(), leaf,
		MustOperExpr(Op("NOT"), Untyped(), NewValueExpr(BoolValue(false), Untyped())))
	// Scala counts only OperEx recursively. LET-IN is an atomic size-one node.
	let := NewLetInExpr(nested, Untyped(), NewOperDecl("Local", nil, nested, Untyped()))

	listener.OnIntroduceCell(0) // ignored outside a rule
	listener.EnterRule("outer")
	listener.OnIntroduceSMTConstant("a")
	listener.OnSMTAssert(nested, 999) // size 4; nSmtNodes is deliberately ignored
	listener.EnterRule("inner")
	listener.OnIntroduceCell(7)
	listener.OnSMTAssert(let, 999) // size 1
	listener.ExitRule()
	listener.OnIntroduceCell(8)
	listener.ExitRule()

	outer := listener.Locator().RuleStat("outer")
	inner := listener.Locator().RuleStat("inner")
	if outer.Calls != 1 || outer.SMTConstantsSelf != 1 || outer.CellsSelf != 1 ||
		outer.SMTAssertionsSelf != 1 || outer.SMTAssertionSizeSum != 4 {
		t.Fatalf("outer = %+v", outer)
	}
	if inner.Calls != 1 || inner.SMTConstantsSelf != 0 || inner.CellsSelf != 1 ||
		inner.SMTAssertionsSelf != 1 || inner.SMTAssertionSizeSum != 1 {
		t.Fatalf("inner = %+v", inner)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("empty ExitRule did not assert")
		}
	}()
	listener.ExitRule()
}

func TestRuleStatLocatorIdentityOrderingAndReport(t *testing.T) {
	locator := NewRuleStatLocator()
	zulu := locator.RuleStat("Zulu")
	alpha := locator.RuleStat("Alpha")
	if locator.RuleStat("Zulu") != zulu {
		t.Fatal("locator allocated a second record for one name")
	}
	zulu.Calls, zulu.CellsSelf = 2, 3
	alpha.Calls, alpha.SMTAssertionsSelf, alpha.SMTAssertionSizeSum = 5, 2, 9
	stats := locator.Stats()
	if len(stats) != 2 || stats[0] != alpha || stats[1] != zulu {
		t.Fatalf("sorted stats = %+v", stats)
	}

	var output bytes.Buffer
	if err := locator.WriteStats(&output); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 6 || lines[0] != "Rule profiling statistics" || lines[1] != strings.Repeat("-", 80) ||
		lines[3] != strings.Repeat("-", 80) {
		t.Fatalf("report framing:\n%s", output.String())
	}
	if !strings.HasPrefix(lines[4], "Alpha") || !strings.HasPrefix(lines[5], "Zulu") ||
		!strings.Contains(lines[4], "        4") {
		t.Fatalf("report order/average:\n%s", output.String())
	}
	if err := locator.WriteStats(nil); err == nil {
		t.Fatal("accepted nil profiling writer")
	}
}
