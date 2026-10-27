package apalache

import "fmt"

func isFunctionCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	_, ok = basic.Type.(FunType)
	return ok
}

func functionRelationType(functionType FunType) SetType {
	pair, _ := NewTupleType(functionType.Argument(), functionType.Result())
	relation, _ := NewSetType(pair)
	return relation
}

func unpackFunctionPair(arena *PureArena, pair ArenaCell) (ArenaCell, ArenaCell, error) {
	edges := arena.GetHas(pair)
	if len(edges) != 2 {
		return ArenaCell{}, ArenaCell{}, fmt.Errorf("function relation pair %s has %d fields", pair, len(edges))
	}
	return edges[0].Element(), edges[1].Element(), nil
}
