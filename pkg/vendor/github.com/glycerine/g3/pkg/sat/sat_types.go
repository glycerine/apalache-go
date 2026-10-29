package sat

// Translation of literal/model parts of src/sat/sat_types.h, Leonardo de Moura.

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"strconv"
	"strings"
)

type BoolVar uint32

const NullBoolVar BoolVar = ^BoolVar(0) >> 1

// A literal is fixed uint32 in both build modes, distinct from arena handles.
// Its Go zero value denotes positive variable zero. Source default construction
// must call DefaultLiteral explicitly to produce the null sentinel.
type Literal uint32

func DefaultLiteral() Literal { return Literal(NullBoolVar << 1) }
func MakeLiteral(v BoolVar, sign bool) Literal {
	if v > NullBoolVar {
		panic("sat: Boolean variable outside literal encoding")
	}
	bits := uint32(v) << 1
	if sign {
		bits++
	}
	return Literal(bits)
}
func LiteralFromUint(v uint32) Literal { return Literal(v) }
func (l Literal) variable() BoolVar    { return BoolVar(l >> 1) }
func (l Literal) sign() bool           { return l&1 != 0 }
func (l Literal) unsign() Literal      { return l &^ 1 }
func (l Literal) index() uint32        { return uint32(l) }
func (l Literal) hash() uint32         { return uint32(l) }
func (l *Literal) neg()                { *l ^= 1 }
func (l Literal) Negated() Literal     { return l ^ 1 }
func (l Literal) String() string {
	prefix := ""
	if l.sign() {
		prefix = "-"
	}
	return prefix + strconv.FormatUint(uint64(l.variable()), 10)
}
func (l Literal) dimacsString() string {
	prefix := ""
	if l.sign() {
		prefix = "-"
	}
	return prefix + strconv.FormatUint(uint64(l.variable())+1, 10)
}

// Model owns a pointer-free slice; consumers retain their own result storage.
// This is the source partial model representation, not a model producer.
type Model []lbool.LBool

func (m Model) valueAt(l Literal) lbool.LBool {
	r := m[l.variable()]
	if l.sign() {
		return r.Negated()
	}
	return r
}
func (m Model) String() string {
	var out strings.Builder
	first := true
	for v, value := range m {
		if value == lbool.Undef {
			continue
		}
		if !first {
			out.WriteByte(' ')
		}
		first = false
		if value != lbool.True {
			out.WriteByte('-')
		}
		out.WriteString(strconv.FormatUint(uint64(v), 10))
	}
	return out.String()
}
func satLiteralsString(ls []Literal) string {
	var out strings.Builder
	for i, l := range ls {
		if i > 0 {
			out.WriteByte(' ')
		}
		out.WriteString(l.String())
	}
	return out.String()
}
