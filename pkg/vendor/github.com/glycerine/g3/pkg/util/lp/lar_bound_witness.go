package lp

// Source: src/util/lp/ul_pair.h, retained LAR bound evidence indices.

import "io"

const InvalidLARIndex uint32 = ^uint32(0)

// LARBoundWitnesses is the source ul_pair value: constraint indices for the
// lower/upper bound and the associated row. Native value copy and == preserve
// the source copy/equality operations; fields replace reference accessors.
type LARBoundWitnesses struct {
	LowBoundWitness   uint32
	UpperBoundWitness uint32
	Row               uint32
}

// Pass InvalidLARIndex for the source default constructor's unassigned row.
func NewLARBoundWitnesses(row uint32) LARBoundWitnesses {
	return LARBoundWitnesses{InvalidLARIndex, InvalidLARIndex, row}
}

// WriteLConstraintKind implements ul_pair.h's stream overload. Its unknown
// value fallback is "??", distinct from lar_constraints.h's assertion/empty
// string formatter. The writer owns the actually accepted output prefix.
func WriteLConstraintKind(w io.Writer, kind LConstraintKind) error {
	text := "??"
	switch kind {
	case LConstraintLE:
		text = "<="
	case LConstraintLT:
		text = "<"
	case LConstraintGE:
		text = ">="
	case LConstraintGT:
		text = ">"
	case LConstraintEQ:
		text = "="
	}
	return writeConstraintText(w, text)
}
