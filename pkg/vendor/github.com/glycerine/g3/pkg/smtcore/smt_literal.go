package smtcore

// Source: src/smt/smt_literal.h:35-103; src/smt/smt_literal.cpp:49-59,86-98;
// src/smt/smt_types.h:33-37.

import (
	utext "github.com/glycerine/g3/pkg/util/text"
)

// SMTLiteral preserves the source signed 32-bit index and negative sentinels.
// Source default construction must use DefaultSMTLiteral; Go's zero value is
// the source true literal. These are inline values, with no owning payload.

type SMTLiteral int32

const (
	SMTNullBoolVar  int32      = -1
	SMTTrueBoolVar  int32      = 0
	SMTFirstBoolVar int32      = 1
	SMTNullLiteral  SMTLiteral = -2
	SMTTrueLiteral  SMTLiteral = 0
	SMTFalseLiteral SMTLiteral = 1
)

func DefaultSMTLiteral() SMTLiteral { return SMTNullLiteral }
func NewSMTLiteral(v int32, sign bool) SMTLiteral {
	value := v << 1
	if sign {
		value++
	}
	return SMTLiteral(value)
}
func SMTLiteralFromIndex(value int32) SMTLiteral { return SMTLiteral(value) }
func (l SMTLiteral) Var() int32                  { return int32(l) >> 1 }
func (l SMTLiteral) Sign() bool                  { return int32(l)&1 != 0 }
func (l SMTLiteral) Index() int32                { return int32(l) }
func (l SMTLiteral) Hash() uint32                { return uint32(l) }
func (l SMTLiteral) Less(other SMTLiteral) bool  { return l < other }
func (l SMTLiteral) Negated() SMTLiteral         { return l ^ 1 }
func (l *SMTLiteral) Negate()                    { *l ^= 1 }

// SMTBackwardSubsumption follows the source nested membership scan. Duplicate
// literals need only one matching occurrence. Both spans are borrowed; no
// backing, sort, deduplication or approximate-set filter is introduced here.
func SMTBackwardSubsumption(first, second []SMTLiteral) bool {
	i := 0
	for ; i < len(first); i++ {
		j := 0
		for ; j < len(second); j++ {
			if first[i] == second[j] {
				break
			}
		}
		if j == len(second) {
			break
		}
	}
	return i == len(first)
}

// DisplayNumeric is the literal ostream insertion at smt_literal.cpp:49-59.
// Its null output is p-1; the separate AST-aware display's null special case
// belongs to that source method, not this insertion.
func (l SMTLiteral) DisplayNumeric(out *utext.NumericTextStream) error {
	if l == SMTTrueLiteral {
		return out.AppendCString("true")
	}
	if l == SMTFalseLiteral {
		return out.AppendCString("false")
	}
	if l.Sign() {
		if e := out.AppendCString("(not p"); e != nil {
			return e
		}
		if e := out.AppendInt32(l.Var()); e != nil {
			return e
		}
		return out.AppendCString(")")
	}
	if e := out.AppendCString("p"); e != nil {
		return e
	}
	return out.AppendInt32(l.Var())
}
