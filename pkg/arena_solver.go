package apalache

import (
	"fmt"
	"strings"
)

// ArenaSolverAdapter synchronizes immutable PureArena updates with declarations
// in a mutable g3 context. Copies share the encoder/solver, as Scala's
// PureArenaAdapter copies share their SolverContext.
type ArenaSolverAdapter struct {
	arena   *PureArena
	encoder *G3GroundEncoder
}

func NewArenaSolverAdapter(solver *G3SolverContext) (*ArenaSolverAdapter, error) {
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		return nil, err
	}
	adapter := &ArenaSolverAdapter{arena: EmptyPureArena(), encoder: encoder}
	types := []CellType{
		BasicCellType{Type: BoolT1},
		BasicCellType{Type: BoolT1},
		BasicCellType{Type: mustSet(BoolT1)},
		InfiniteSetCellType{Element: BasicCellType{Type: IntT1}},
		InfiniteSetCellType{Element: BasicCellType{Type: IntT1}},
	}
	cells := make([]ArenaCell, len(types))
	for index, cellType := range types {
		adapter, cells[index], err = adapter.AppendCellType(cellType)
		if err != nil {
			return nil, err
		}
	}
	adapter, err = adapter.AppendHas(cells[2], FixedElementPointer{Cell: cells[0]}, FixedElementPointer{Cell: cells[1]})
	if err != nil {
		return nil, err
	}
	initialConstraints := []Expr{
		MustOperExpr(Op("NOT"), Typed(BoolT1), cells[0].NameExpr()),
		cells[1].NameExpr(),
		MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1), cells[0].NameExpr(), cells[2].NameExpr()),
		MustOperExpr(Op("Apalache!StoreInSet"), Typed(BoolT1), cells[1].NameExpr(), cells[2].NameExpr()),
	}
	if err := encoder.AssertAll(initialConstraints); err != nil {
		return nil, fmt.Errorf("assert initial arena constraints: %w", err)
	}
	return adapter, nil
}

func (adapter *ArenaSolverAdapter) PureArena() *PureArena { return adapter.arena }
func (adapter *ArenaSolverAdapter) Encoder() *G3GroundEncoder {
	return adapter.encoder
}

// SetSolver returns an adapter over the same persistent arena and a fresh
// solver context. Like Scala's setSolver, it does not replay declarations.
func (adapter *ArenaSolverAdapter) SetSolver(solver *G3SolverContext) (*ArenaSolverAdapter, error) {
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		return nil, err
	}
	return &ArenaSolverAdapter{arena: adapter.arena, encoder: encoder}, nil
}

func (adapter *ArenaSolverAdapter) CellCount() int     { return adapter.arena.CellCount() }
func (adapter *ArenaSolverAdapter) TopCell() ArenaCell { return adapter.arena.TopCell() }
func (adapter *ArenaSolverAdapter) FalseCell() ArenaCell {
	return adapter.arena.FalseCell()
}
func (adapter *ArenaSolverAdapter) TrueCell() ArenaCell { return adapter.arena.TrueCell() }
func (adapter *ArenaSolverAdapter) BooleanSetCell() ArenaCell {
	return adapter.arena.BooleanSetCell()
}
func (adapter *ArenaSolverAdapter) NatSetCell() ArenaCell { return adapter.arena.NatSetCell() }
func (adapter *ArenaSolverAdapter) IntSetCell() ArenaCell { return adapter.arena.IntSetCell() }

func (adapter *ArenaSolverAdapter) FindCellByName(name string) (ArenaCell, error) {
	return adapter.arena.FindCellByName(name)
}

func (adapter *ArenaSolverAdapter) FindCellByNameExpr(expression Expr) (ArenaCell, error) {
	return adapter.arena.FindCellByNameExpr(expression)
}

func (adapter *ArenaSolverAdapter) AppendCellType(cellType CellType, unconstrained ...bool) (*ArenaSolverAdapter, ArenaCell, error) {
	newArena, cell := adapter.arena.AppendCellType(cellType, unconstrained...)
	if err := adapter.encoder.DeclareCell(cell); err != nil {
		return nil, ArenaCell{}, err
	}
	return &ArenaSolverAdapter{arena: newArena, encoder: adapter.encoder}, cell, nil
}

func (adapter *ArenaSolverAdapter) AppendTLAType(typ Type, unconstrained ...bool) (*ArenaSolverAdapter, ArenaCell, error) {
	cellType, err := CellTypeFromType(typ)
	if err != nil {
		return nil, ArenaCell{}, err
	}
	return adapter.AppendCellType(cellType, unconstrained...)
}

func (adapter *ArenaSolverAdapter) AppendCellTypeNoSMT(cellType CellType, unconstrained ...bool) (*ArenaSolverAdapter, ArenaCell) {
	newArena, cell := adapter.arena.AppendCellType(cellType, unconstrained...)
	return &ArenaSolverAdapter{arena: newArena, encoder: adapter.encoder}, cell
}

func (adapter *ArenaSolverAdapter) AppendCellTypes(types ...CellType) (*ArenaSolverAdapter, []ArenaCell, error) {
	current := adapter
	cells := make([]ArenaCell, len(types))
	for index, cellType := range types {
		var err error
		current, cells[index], err = current.AppendCellType(cellType)
		if err != nil {
			return nil, nil, err
		}
	}
	return current, cells, nil
}

func (adapter *ArenaSolverAdapter) AppendHas(parent ArenaCell, children ...ElementPointer) (*ArenaSolverAdapter, error) {
	for _, pointer := range children {
		if err := adapter.encoder.DeclareMembership(parent, pointer.Element()); err != nil {
			return nil, err
		}
	}
	return adapter.AppendHasNoSMT(parent, children...), nil
}

func (adapter *ArenaSolverAdapter) AppendHasNoSMT(parent ArenaCell, children ...ElementPointer) *ArenaSolverAdapter {
	return &ArenaSolverAdapter{arena: adapter.arena.AppendHas(parent, children...), encoder: adapter.encoder}
}

func (adapter *ArenaSolverAdapter) GetHas(parent ArenaCell) []ArenaCell {
	pointers := adapter.arena.GetHas(parent)
	result := make([]ArenaCell, len(pointers))
	for index, pointer := range pointers {
		result[index] = pointer.Element()
	}
	return result
}

func (adapter *ArenaSolverAdapter) GetHasPointers(parent ArenaCell) []ElementPointer {
	return adapter.arena.GetHas(parent)
}

func (adapter *ArenaSolverAdapter) SetDomain(function, domain ArenaCell) (*ArenaSolverAdapter, error) {
	if adapter.arena.HasDomain(function) {
		return nil, fmt.Errorf("function cell %s already has a domain", function)
	}
	return &ArenaSolverAdapter{arena: adapter.arena.SetDomain(function, domain), encoder: adapter.encoder}, nil
}

func (adapter *ArenaSolverAdapter) SetCodomain(function, codomain ArenaCell) (*ArenaSolverAdapter, error) {
	if adapter.arena.HasCodomain(function) {
		return nil, fmt.Errorf("function cell %s already has a codomain", function)
	}
	return &ArenaSolverAdapter{arena: adapter.arena.SetCodomain(function, codomain), encoder: adapter.encoder}, nil
}

func (adapter *ArenaSolverAdapter) Domain(function ArenaCell) (ArenaCell, error) {
	return adapter.arena.Domain(function)
}

func (adapter *ArenaSolverAdapter) Codomain(function ArenaCell) (ArenaCell, error) {
	return adapter.arena.Codomain(function)
}

func (adapter *ArenaSolverAdapter) HasDomain(cell ArenaCell) bool {
	return adapter.arena.HasDomain(cell)
}

func (adapter *ArenaSolverAdapter) HasCodomain(cell ArenaCell) bool {
	return adapter.arena.HasCodomain(cell)
}

func (adapter *ArenaSolverAdapter) IsLinkedViaHas(source, destination ArenaCell) bool {
	return adapter.arena.IsLinkedViaHas(source, destination)
}

func (adapter *ArenaSolverAdapter) FindCellsByType(cellType CellType) []ArenaCell {
	return adapter.arena.FindCellsByType(cellType)
}

func (adapter *ArenaSolverAdapter) Assert(expression Expr) error {
	return adapter.encoder.Assert(expression)
}

func (adapter *ArenaSolverAdapter) AssertAll(expressions []Expr) error {
	return adapter.encoder.AssertAll(expressions)
}

func (adapter *ArenaSolverAdapter) SubgraphString(root ArenaCell) string {
	var builder strings.Builder
	var visit func(ArenaCell)
	visit = func(cell ArenaCell) {
		fmt.Fprintf(&builder, "%d ", cell.ID)
		children := adapter.GetHas(cell)
		if len(children) == 0 {
			return
		}
		builder.WriteString("has:(")
		for _, child := range children {
			visit(child)
		}
		builder.WriteByte(')')
	}
	visit(root)
	return builder.String()
}
