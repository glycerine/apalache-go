package apalache

import (
	"fmt"
	"sort"
)

const (
	ArenaFalseName  = ArenaCellNamePrefix + "0"
	ArenaTrueName   = ArenaCellNamePrefix + "1"
	ArenaBooleanSet = ArenaCellNamePrefix + "2"
	ArenaNatSet     = ArenaCellNamePrefix + "3"
	ArenaIntSet     = ArenaCellNamePrefix + "4"
)

// PureArena is an immutable, solver-free symbolic memory graph.
type PureArena struct {
	cells     map[string]ArenaCell
	has       map[int][]ElementPointer
	domains   map[int]ArenaCell
	codomains map[int]ArenaCell
	top       ArenaCell
	count     int
}

func EmptyPureArena() *PureArena {
	return &PureArena{
		cells: make(map[string]ArenaCell), has: make(map[int][]ElementPointer),
		domains: make(map[int]ArenaCell), codomains: make(map[int]ArenaCell),
		top: NewArenaCell(-1, UnknownCellType{}),
	}
}

func InitialPureArena() *PureArena {
	arena := EmptyPureArena()
	types := []CellType{
		BasicCellType{Type: BoolT1}, BasicCellType{Type: BoolT1}, BasicCellType{Type: mustSet(BoolT1)},
		InfiniteSetCellType{Element: BasicCellType{Type: IntT1}},
		InfiniteSetCellType{Element: BasicCellType{Type: IntT1}},
	}
	arena, cells := arena.AppendCellTypes(types...)
	return arena.AppendHas(cells[2], FixedElementPointer{Cell: cells[0]}, FixedElementPointer{Cell: cells[1]})
}

func (arena *PureArena) CellCount() int     { return arena.count }
func (arena *PureArena) TopCell() ArenaCell { return arena.top }

func (arena *PureArena) CellMap() map[string]ArenaCell {
	return cloneMap(arena.cells)
}

func (arena *PureArena) FindCellByName(name string) (ArenaCell, error) {
	cell, found := arena.cells[name]
	if !found {
		return ArenaCell{}, fmt.Errorf("arena cell %s was not found", name)
	}
	return cell, nil
}

func (arena *PureArena) FindCellByNameExpr(expression Expr) (ArenaCell, error) {
	name, ok := expression.(*NameExpr)
	if !ok || !IsArenaCellName(name.Name()) {
		return ArenaCell{}, fmt.Errorf("expected a name expression with a cell name")
	}
	return arena.FindCellByName(name.Name())
}

func (arena *PureArena) NextCell(cellType CellType, unconstrained ...bool) ArenaCell {
	return NewArenaCell(arena.count, cellType, unconstrained...)
}

func (arena *PureArena) AppendCell(cell ArenaCell) (*PureArena, error) {
	if _, found := arena.cells[cell.String()]; found {
		return nil, fmt.Errorf("cell %s is already a member of the arena", cell)
	}
	result := arena.clone()
	result.cells[cell.String()] = cell
	result.top = cell
	result.count++
	return result, nil
}

func (arena *PureArena) MustAppendCell(cell ArenaCell) *PureArena {
	result, err := arena.AppendCell(cell)
	if err != nil {
		panic(err)
	}
	return result
}

func (arena *PureArena) AppendCellType(cellType CellType, unconstrained ...bool) (*PureArena, ArenaCell) {
	cell := arena.NextCell(cellType, unconstrained...)
	return arena.MustAppendCell(cell), cell
}

func (arena *PureArena) AppendCells(cells ...ArenaCell) (*PureArena, error) {
	result := arena
	for _, cell := range cells {
		var err error
		result, err = result.AppendCell(cell)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (arena *PureArena) AppendCellTypes(types ...CellType) (*PureArena, []ArenaCell) {
	result := arena
	cells := make([]ArenaCell, len(types))
	for index, cellType := range types {
		result, cells[index] = result.AppendCellType(cellType)
	}
	return result, cells
}

func (arena *PureArena) AppendHas(parent ArenaCell, children ...ElementPointer) *PureArena {
	result := arena.clone()
	result.has[parent.ID] = append(append([]ElementPointer(nil), result.has[parent.ID]...), children...)
	return result
}

func (arena *PureArena) GetHas(parent ArenaCell) []ElementPointer {
	return append([]ElementPointer(nil), arena.has[parent.ID]...)
}

func (arena *PureArena) IsLinkedViaHas(source, destination ArenaCell) bool {
	for _, pointer := range arena.has[source.ID] {
		if pointer.Element().Equal(destination) {
			return true
		}
	}
	return false
}

func (arena *PureArena) FindCellsByType(cellType CellType) []ArenaCell {
	result := make([]ArenaCell, 0)
	for _, cell := range arena.cells {
		if EqualCellTypes(cell.Type, cellType) {
			result = append(result, cell)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result
}

func (arena *PureArena) HasDomain(cell ArenaCell) bool {
	_, found := arena.domains[cell.ID]
	return found
}

func (arena *PureArena) SetDomain(function, domain ArenaCell) *PureArena {
	result := arena.clone()
	result.domains[function.ID] = domain
	return result
}

func (arena *PureArena) Domain(function ArenaCell) (ArenaCell, error) {
	domain, found := arena.domains[function.ID]
	if !found {
		return ArenaCell{}, fmt.Errorf("cell %s has no domain edge", function)
	}
	return domain, nil
}

func (arena *PureArena) HasCodomain(cell ArenaCell) bool {
	_, found := arena.codomains[cell.ID]
	return found
}

func (arena *PureArena) SetCodomain(function, codomain ArenaCell) *PureArena {
	result := arena.clone()
	result.codomains[function.ID] = codomain
	return result
}

func (arena *PureArena) Codomain(function ArenaCell) (ArenaCell, error) {
	codomain, found := arena.codomains[function.ID]
	if !found {
		return ArenaCell{}, fmt.Errorf("cell %s has no codomain edge", function)
	}
	return codomain, nil
}

func (arena *PureArena) FalseCell() ArenaCell      { return arena.mustFind(ArenaFalseName) }
func (arena *PureArena) TrueCell() ArenaCell       { return arena.mustFind(ArenaTrueName) }
func (arena *PureArena) BooleanSetCell() ArenaCell { return arena.mustFind(ArenaBooleanSet) }
func (arena *PureArena) NatSetCell() ArenaCell     { return arena.mustFind(ArenaNatSet) }
func (arena *PureArena) IntSetCell() ArenaCell     { return arena.mustFind(ArenaIntSet) }

func (arena *PureArena) mustFind(name string) ArenaCell {
	cell, err := arena.FindCellByName(name)
	if err != nil {
		panic(err)
	}
	return cell
}

func (arena *PureArena) clone() *PureArena {
	result := &PureArena{
		cells: cloneMap(arena.cells), has: make(map[int][]ElementPointer, len(arena.has)),
		domains: cloneMap(arena.domains), codomains: cloneMap(arena.codomains), top: arena.top, count: arena.count,
	}
	for id, pointers := range arena.has {
		result.has[id] = append([]ElementPointer(nil), pointers...)
	}
	return result
}
