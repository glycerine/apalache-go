package apalache

import "fmt"

func isSequenceCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	_, ok = basic.Type.(SeqType)
	return ok
}

func makeSequence(arena *PureArena, sequenceType SeqType, elements []ArenaCell, integers *IntValueCache) (*PureArena, ArenaCell, error) {
	current, length, err := integers.GetOrCreate(arena, Int64Value(int64(len(elements))).BigInt())
	if err != nil {
		return nil, ArenaCell{}, err
	}
	current, sequence := makeSequenceWithLength(current, sequenceType, elements, length)
	return current, sequence, nil
}

func makeSequenceWithLength(arena *PureArena, sequenceType SeqType, elements []ArenaCell, length ArenaCell) (*PureArena, ArenaCell) {
	current, proto := arena.AppendCellType(UnknownCellType{}, true)
	pointers := make([]ElementPointer, len(elements))
	for index, element := range elements {
		pointers[index] = FixedElementPointer{Cell: element}
	}
	current = current.AppendHas(proto, pointers...)
	current, sequence := current.AppendCellType(BasicCellType{Type: sequenceType})
	current = current.AppendHas(sequence,
		FixedElementPointer{Cell: length}, FixedElementPointer{Cell: proto})
	return current, sequence
}

func unpackSequence(arena *PureArena, sequence ArenaCell) (length, proto ArenaCell, elements []ElementPointer, err error) {
	if arena == nil || !isSequenceCellType(sequence.Type) {
		return ArenaCell{}, ArenaCell{}, nil, fmt.Errorf("cell %s is not a sequence", sequence)
	}
	edges := arena.GetHas(sequence)
	if len(edges) != 2 {
		return ArenaCell{}, ArenaCell{}, nil, fmt.Errorf("sequence %s has %d structural edges, expected 2", sequence, len(edges))
	}
	length, proto = edges[0].Element(), edges[1].Element()
	if !EqualTypes(length.Type.TLAType(), IntT1) {
		return ArenaCell{}, ArenaCell{}, nil, fmt.Errorf("sequence %s has non-integer length %s", sequence, length.Type)
	}
	return length, proto, arena.GetHas(proto), nil
}
