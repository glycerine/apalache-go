package ast

// Source: src/ast/ast.cpp:1650-1653; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The caller holds the manager lock while scanning the canonical table.
func (m *ASTManager) traceNondeterminismCollision(existing, candidate astNodeRef) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("nondet_bug") {
		return nil
	}
	firstHash, err := m.nodeHash(existing)
	if err != nil {
		return err
	}
	secondHash, err := m.nodeHash(candidate)
	if err != nil {
		return err
	}
	firstID := m.nodeAt(existing.slot).id
	secondID := m.nodeAt(candidate.slot).id
	return tags.EmitConditional("nondet_bug", "slow_not_contains", "src/ast/ast.cpp", 1650,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, "id1:   "+strconv.FormatUint(uint64(firstID), 10)+
				", id2: "+strconv.FormatUint(uint64(secondID), 10)+"\n"+
				"hash1: "+strconv.FormatUint(uint64(firstHash), 10)+
				", hash2: "+strconv.FormatUint(uint64(secondHash), 10)+"\n")
			return err
		})
}
