package apalache

import (
	"reflect"
	"testing"
)

func TestCellTypesSignaturesConversionsAndValidation(t *testing.T) {
	tuple := mustTuple(IntT1, BoolT1)
	function := mustFun(IntT1, BoolT1)
	tests := []struct {
		typ       Type
		signature string
	}{
		{BoolT1, "b"}, {IntT1, "i"}, {StrT1, "UT_Str"},
		{mustSet(IntT1), "Si"}, {mustSeq(BoolT1), "Q_b"},
		{function, "fi_b"}, {tuple, "T_i_b"},
	}
	for _, test := range tests {
		cellType, err := CellTypeFromType(test.typ)
		if err != nil || cellType.Signature() != test.signature || !EqualTypes(cellType.TLAType(), test.typ) {
			t.Errorf("CellTypeFromType(%s) = %v, %v", test.typ, cellType, err)
		}
	}
	variable, _ := NewVarType(1)
	for _, unsupported := range []Type{RealT1, variable, mustOperator(nil, IntT1)} {
		if _, err := CellTypeFromType(unsupported); err == nil {
			t.Errorf("unsupported arena type %s was accepted", unsupported)
		}
	}
	openVar, _ := NewVarType(2)
	openRow, _ := NewRowType(map[string]Type{"x": IntT1}, &openVar)
	if _, err := CellTypeFromType(NewRecordRowType(openRow)); err == nil {
		t.Fatal("open record row was accepted")
	}

	domain := BasicCellType{Type: mustSet(IntT1)}
	codomain, _ := NewPowerSetCellType(mustSet(BoolT1))
	functionSet, err := NewFiniteFunctionSetCellType(domain, codomain)
	if err != nil || functionSet.Signature() != "F%Si_%PSb" {
		t.Fatalf("function-set cell type = %v, %v", functionSet, err)
	}
	want := mustSet(mustFun(IntT1, mustSet(BoolT1)))
	if !EqualTypes(functionSet.TLAType(), want) {
		t.Fatalf("function-set TLA type = %s, want %s", functionSet.TLAType(), want)
	}
}

func TestArenaCellNamesAndElementPointers(t *testing.T) {
	cell := NewArenaCell(12, BasicCellType{Type: IntT1}, true)
	if cell.String() != "$C$12" || !cell.Unconstrained || !cell.NameExpr().Tag().Equal(Typed(IntT1)) {
		t.Fatalf("arena cell = %#v", cell)
	}
	if id, err := ArenaCellIDFromName(cell.String()); err != nil || id != 12 || !IsArenaCellName(cell.String()) {
		t.Fatalf("cell name parse = %d, %v", id, err)
	}
	if _, err := ArenaCellIDFromName("x"); err == nil {
		t.Fatal("non-cell name was accepted")
	}
	if _, err := ArenaCellIDFromName("$C$12junk"); err == nil {
		t.Fatal("cell name with a non-numeric suffix was accepted")
	}

	condition := NewNameExpr("p", Typed(BoolT1))
	fixed := FixedElementPointer{Cell: cell}
	if value := fixed.ToSMT().(*ValueExpr).Value().(BoolValue); !bool(value) {
		t.Fatal("fixed pointer is not always true")
	}
	restricted := fixed.Restrict(condition)
	if restricted.Element() != cell || restricted.ToSMT() != condition {
		t.Fatalf("restricted fixed pointer = %#v", restricted)
	}
	again := restricted.Restrict(NewNameExpr("q", Typed(BoolT1)))
	if again.ToSMT().(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("restricted SMT pointer = %#v", again)
	}
}

func TestPureArenaCellsEdgesAndPersistence(t *testing.T) {
	empty := EmptyPureArena()
	intType := BasicCellType{Type: IntT1}
	setType := BasicCellType{Type: mustSet(IntT1)}
	withSet, setCell := empty.AppendCellType(setType)
	withCells, cells := withSet.AppendCellTypes(intType, intType)
	if empty.CellCount() != 0 || withSet.CellCount() != 1 || withCells.CellCount() != 3 || withCells.TopCell().ID != 2 {
		t.Fatalf("arena counts = %d, %d, %d", empty.CellCount(), withSet.CellCount(), withCells.CellCount())
	}
	linked := withCells.AppendHas(setCell, FixedElementPointer{Cell: cells[0]}, SMTElementPointer{Cell: cells[1], Expression: NewNameExpr("in", Typed(BoolT1))})
	if len(withCells.GetHas(setCell)) != 0 || len(linked.GetHas(setCell)) != 2 || !linked.IsLinkedViaHas(setCell, cells[1]) {
		t.Fatal("has edges are incorrect or mutated an earlier arena")
	}
	found, err := linked.FindCellByNameExpr(cells[0].NameExpr())
	if err != nil || !found.Equal(cells[0]) {
		t.Fatalf("cell lookup = %v, %v", found, err)
	}
	if got := linked.FindCellsByType(intType); !reflect.DeepEqual(got, cells) {
		t.Fatalf("cells by type = %v, want %v", got, cells)
	}
	functionArena, function := linked.AppendCellType(BasicCellType{Type: mustFun(IntT1, IntT1)})
	domainArena := functionArena.SetDomain(function, setCell).SetCodomain(function, setCell)
	if functionArena.HasDomain(function) || !domainArena.HasDomain(function) || !domainArena.HasCodomain(function) {
		t.Fatal("domain/codomain updates were not persistent")
	}
	if domain, _ := domainArena.Domain(function); !domain.Equal(setCell) {
		t.Fatalf("domain = %v", domain)
	}
	if _, err := domainArena.AppendCell(cells[0]); err == nil {
		t.Fatal("duplicate cell was accepted")
	}
}

func TestInitialPureArenaConventions(t *testing.T) {
	arena := InitialPureArena()
	if arena.CellCount() != 5 || arena.FalseCell().ID != 0 || arena.TrueCell().ID != 1 || arena.BooleanSetCell().ID != 2 || arena.NatSetCell().ID != 3 || arena.IntSetCell().ID != 4 {
		t.Fatalf("initial arena convention is broken: %#v", arena.CellMap())
	}
	has := arena.GetHas(arena.BooleanSetCell())
	if len(has) != 2 || !has[0].Element().Equal(arena.FalseCell()) || !has[1].Element().Equal(arena.TrueCell()) {
		t.Fatalf("BOOLEAN edges = %#v", has)
	}
}
