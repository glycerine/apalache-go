package apalache

import "testing"

func TestSequenceRepresentationContracts(t *testing.T) {
	sequenceType := mustSeq(IntT1).(SeqType)
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	var element ArenaCell
	var err error
	scope, element, err = rewriter.Rewrite(intLiteral(7), scope)
	if err != nil {
		t.Fatal(err)
	}
	arena, sequence, err := makeSequence(scope.Arena, sequenceType, []ArenaCell{element}, rewriter.IntegerCache())
	if err != nil {
		t.Fatal(err)
	}
	length, proto, elements, err := unpackSequence(arena, sequence)
	if err != nil || len(elements) != 1 || !elements[0].Element().Equal(element) {
		t.Fatalf("unpack sequence = length %v, proto %v, elements %#v, error %v", length, proto, elements, err)
	}
	lengthValue, found := rewriter.IntegerCache().FindValue(length)
	if !found || lengthValue.Int64() != 1 || len(arena.GetHas(proto)) != 1 {
		t.Fatalf("sequence representation = length %v (%v), proto capacity %d", lengthValue, found, len(arena.GetHas(proto)))
	}

	if _, _, _, err = unpackSequence(arena, element); err == nil {
		t.Fatal("non-sequence cell was unpacked")
	}
	brokenArena, missingEdges := arena.AppendCellType(BasicCellType{Type: sequenceType})
	if _, _, _, err = unpackSequence(brokenArena, missingEdges); err == nil {
		t.Fatal("sequence without length/proto edges was unpacked")
	}
	brokenArena, wrongLengthProto := brokenArena.AppendCellType(UnknownCellType{}, true)
	brokenArena, wrongLength := brokenArena.AppendCellType(BasicCellType{Type: sequenceType})
	brokenArena = brokenArena.AppendHas(wrongLength,
		FixedElementPointer{Cell: brokenArena.TrueCell()}, FixedElementPointer{Cell: wrongLengthProto})
	if _, _, _, err = unpackSequence(brokenArena, wrongLength); err == nil {
		t.Fatal("sequence with non-integer length was unpacked")
	}
}
