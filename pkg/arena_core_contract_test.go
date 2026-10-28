package apalache

import "testing"

func TestArenaCellScalaValueAndBuilderContracts(t *testing.T) {
	cell := NewArenaCell(12, BasicCellType{Type: IntT1}, true)
	if cell.String() != "$C$12" || !cell.Unconstrained || cell.Compare(NewArenaCell(13, UnknownCellType{})) >= 0 {
		t.Fatalf("cell value contract = %#v", cell)
	}
	if !cell.Equal(NewArenaCell(12, BasicCellType{Type: BoolT1})) || cell.Equal(NewArenaCell(11, BasicCellType{Type: IntT1})) {
		t.Fatal("ArenaCell equality must depend only on the identifier")
	}
	built, err := cell.ToBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	name, ok := built.(*NameExpr)
	if !ok || name.Name() != cell.String() || !name.Tag().Equal(Typed(IntT1)) {
		t.Fatalf("ToBuilder() = %#v", built)
	}
	if direct := cell.NameExpr(); direct.Name() != name.Name() || !direct.Tag().Equal(name.Tag()) {
		t.Fatalf("NameExpr() = %#v, builder = %#v", direct, name)
	}

	for _, valid := range []string{"$C$", "$C$0", "$C$-1", "$C$12junk"} {
		if !IsArenaCellName(valid) {
			t.Errorf("IsArenaCellName(%q) = false", valid)
		}
	}
	for _, invalid := range []string{"", "$C", "C$1", "x$C$1"} {
		if IsArenaCellName(invalid) {
			t.Errorf("IsArenaCellName(%q) = true", invalid)
		}
	}
	for _, test := range []struct {
		name string
		id   int
	}{{"$C$0", 0}, {"$C$-1", -1}, {"$C$214", 214}} {
		if id, err := ArenaCellIDFromName(test.name); err != nil || id != test.id {
			t.Errorf("ArenaCellIDFromName(%q) = %d, %v", test.name, id, err)
		}
	}
	for _, invalid := range []string{"x", "$C$", "$C$1x"} {
		if _, err := ArenaCellIDFromName(invalid); err == nil {
			t.Errorf("ArenaCellIDFromName(%q) succeeded", invalid)
		}
	}
}

func TestElementPointerScalaContracts(t *testing.T) {
	cell := NewArenaCell(1, BasicCellType{Type: IntT1})
	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	fixed := FixedElementPointer{Cell: cell}
	if fixed.Element() != cell || !bool(fixed.ToSMT().(*ValueExpr).Value().(BoolValue)) {
		t.Fatalf("fixed pointer = %#v", fixed)
	}
	general := fixed.Generalize()
	if general.Cell != cell || !bool(general.Expression.(*ValueExpr).Value().(BoolValue)) {
		t.Fatalf("generalized pointer = %#v", general)
	}
	if restricted := fixed.Restrict(p); restricted.Cell != cell || restricted.Expression != p {
		t.Fatalf("restricted fixed pointer = %#v", restricted)
	}

	guarded := SMTElementPointer{Cell: cell, Expression: p}
	if guarded.Generalize() != guarded || guarded.ToSMT() != p {
		t.Fatalf("guarded pointer identity = %#v", guarded)
	}
	restricted := guarded.Restrict(q)
	and, ok := restricted.Expression.(*OperExpr)
	if !ok || and.Operator().Name() != "AND" || and.Arguments()[0] != p || and.Arguments()[1] != q {
		t.Fatalf("restricted guarded pointer = %#v", restricted)
	}

	assertPanics(t, func() { guarded.Restrict(NewValueExpr(Int64Value(1), Typed(IntT1))) })
	assertPanics(t, func() {
		SMTElementPointer{Cell: cell, Expression: NewValueExpr(Int64Value(1), Typed(IntT1))}.Restrict(q)
	})
}

func TestPureArenaScalaPersistenceAndOwnershipContracts(t *testing.T) {
	empty := EmptyPureArena()
	if empty.CellCount() != 0 || empty.TopCell().ID != -1 {
		t.Fatalf("empty arena = count %d, top %v", empty.CellCount(), empty.TopCell())
	}
	if _, ok := empty.TopCell().Type.(UnknownCellType); !ok {
		t.Fatalf("invalid cell type = %T", empty.TopCell().Type)
	}

	intType := BasicCellType{Type: IntT1}
	setType := BasicCellType{Type: mustSetType(t, IntT1)}
	first := empty.NextCell(intType, true)
	if first.ID != 0 || !first.Unconstrained {
		t.Fatalf("NextCell() = %#v", first)
	}
	one, err := empty.AppendCell(first)
	if err != nil || one.CellCount() != 1 || one.TopCell() != first || empty.CellCount() != 0 {
		t.Fatalf("AppendCell() = %#v, %v", one, err)
	}
	if duplicate, err := one.AppendCell(NewArenaCell(first.ID, setType)); err == nil || duplicate != nil {
		t.Fatalf("duplicate append = %#v, %v", duplicate, err)
	}

	three, cells := one.AppendCellTypes(intType, setType)
	if len(cells) != 2 || cells[0].ID != 1 || cells[1].ID != 2 || three.TopCell() != cells[1] || three.CellCount() != 3 {
		t.Fatalf("AppendCellTypes() cells = %#v, arena count/top = %d/%v", cells, three.CellCount(), three.TopCell())
	}
	cellMap := three.CellMap()
	delete(cellMap, first.String())
	cellMap["fake"] = NewArenaCell(99, intType)
	if _, err := three.FindCellByName(first.String()); err != nil || len(three.CellMap()) != 3 {
		t.Fatal("CellMap exposed arena ownership")
	}
	if found, err := three.FindCellByNameExpr(first.NameExpr()); err != nil || found != first {
		t.Fatalf("FindCellByNameExpr() = %v, %v", found, err)
	}
	if _, err := three.FindCellByNameExpr(NewValueExpr(Int64Value(1), Typed(IntT1))); err == nil {
		t.Fatal("non-name cell lookup succeeded")
	}
	if _, err := three.FindCellByNameExpr(NewNameExpr("ordinary", Typed(IntT1))); err == nil {
		t.Fatal("ordinary name cell lookup succeeded")
	}

	withEdges := three.AppendHas(cells[1], FixedElementPointer{Cell: first})
	withMoreEdges := withEdges.AppendHas(cells[1], SMTElementPointer{Cell: cells[0], Expression: NewNameExpr("member", Typed(BoolT1))})
	if len(three.GetHas(cells[1])) != 0 || len(withEdges.GetHas(cells[1])) != 1 || len(withMoreEdges.GetHas(cells[1])) != 2 {
		t.Fatal("AppendHas mutated an earlier arena or lost ordering")
	}
	exposed := withMoreEdges.GetHas(cells[1])
	exposed[0] = SMTElementPointer{Cell: cells[0], Expression: NewNameExpr("changed", Typed(BoolT1))}
	if _, fixed := withMoreEdges.GetHas(cells[1])[0].(FixedElementPointer); !fixed {
		t.Fatal("GetHas exposed arena ownership")
	}
	if !withMoreEdges.IsLinkedViaHas(cells[1], first) || withMoreEdges.IsLinkedViaHas(cells[1], NewArenaCell(42, intType)) {
		t.Fatal("IsLinkedViaHas returned an incorrect result")
	}

	matching := withMoreEdges.FindCellsByType(intType)
	if len(matching) != 2 || matching[0].ID != 0 || matching[1].ID != 1 || len(withMoreEdges.FindCellsByType(BasicCellType{Type: BoolT1})) != 0 {
		t.Fatalf("FindCellsByType() = %#v", matching)
	}
}

func TestPureArenaScalaDomainAndInitialContracts(t *testing.T) {
	intSet := BasicCellType{Type: mustSetType(t, IntT1)}
	functionType := BasicCellType{Type: mustFunType(t, IntT1, IntT1)}
	arena, cells := EmptyPureArena().AppendCellTypes(functionType, intSet, intSet)
	function, firstSet, secondSet := cells[0], cells[1], cells[2]
	if arena.HasDomain(function) || arena.HasCodomain(function) {
		t.Fatal("new function unexpectedly has domain edges")
	}
	if _, err := arena.Domain(function); err == nil {
		t.Fatal("missing domain lookup succeeded")
	}
	if _, err := arena.Codomain(function); err == nil {
		t.Fatal("missing codomain lookup succeeded")
	}
	linked := arena.SetDomain(function, firstSet).SetCodomain(function, secondSet)
	if arena.HasDomain(function) || !linked.HasDomain(function) || !linked.HasCodomain(function) {
		t.Fatal("domain updates were not persistent")
	}
	if domain, _ := linked.Domain(function); domain != firstSet {
		t.Fatalf("domain = %v", domain)
	}
	if codomain, _ := linked.Codomain(function); codomain != secondSet {
		t.Fatalf("codomain = %v", codomain)
	}
	overridden := linked.SetDomain(function, secondSet)
	if domain, _ := overridden.Domain(function); domain != secondSet {
		t.Fatalf("overridden domain = %v", domain)
	}
	if domain, _ := linked.Domain(function); domain != firstSet {
		t.Fatal("domain override mutated the prior arena")
	}

	initial := InitialPureArena()
	if initial.CellCount() != 5 || initial.FalseCell().ID != 0 || initial.TrueCell().ID != 1 || initial.BooleanSetCell().ID != 2 || initial.NatSetCell().ID != 3 || initial.IntSetCell().ID != 4 {
		t.Fatalf("initial arena convention = %#v", initial.CellMap())
	}
	booleanMembers := initial.GetHas(initial.BooleanSetCell())
	if len(booleanMembers) != 2 || booleanMembers[0].Element() != initial.FalseCell() || booleanMembers[1].Element() != initial.TrueCell() {
		t.Fatalf("BOOLEAN membership = %#v", booleanMembers)
	}
}
