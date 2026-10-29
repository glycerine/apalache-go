package ast

// Source: src/ast/array_decl_plugin.cpp:86-113; src/util/rational.h:67,110-181.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"math"
	"reflect"
)

// Concrete source lexical values, held in one real charged cold frame only
// when needed by this finite-cardinality branch. No heap rational wrapper or
// shadow pointer payload is created. Failed source constructors are owned for
// explicit abort, but are not completed values on the source DT schedule.
type astArrayCardinalityFrame struct {
	Next                             *astArrayCardinalityFrame
	Self                             *astArrayCardinalityFrame
	Owner                            *ASTManager
	tokens                           [4]unumeral.OwnedRational // immutable latest actual returned construction/acquisition tuples
	Values                           [4]unumeral.OwnedRational // domain_sz, num_elements, conversion, power result
	Acquired, completed              [4]bool
	Invocation, header               uint64
	sourceDone                       bool
	bodyCause, sourceCause, ownCause error
}

func (m *ASTManager) arrayCardinalityBytes() uint64 {
	var n uint64
	for f := m.ArrayCardinality; f != nil; f = f.Next {
		n += f.header
	}
	return n
}
func (m *ASTManager) newArrayCardinalityFrame() (*astArrayCardinalityFrame, error) {
	if m.arrayCardinalityInvocation == math.MaxUint64 {
		return nil, ErrASTPending
	}
	bytes := uint64(reflect.TypeOf(astArrayCardinalityFrame{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	m.arrayCardinalityInvocation++
	f := &astArrayCardinalityFrame{Owner: m, Invocation: m.arrayCardinalityInvocation, header: bytes, Next: m.ArrayCardinality}
	f.Self = f
	m.ArrayCardinality = f
	return f, nil
}
func (f *astArrayCardinalityFrame) construct(i int, n uint64, set bool) error {
	if f.Acquired[i] {
		return ErrASTPending
	}
	v, e := f.Owner.numerals.NewValue()
	if e != nil {
		return e
	}
	f.Values[i] = v
	f.tokens[i] = v
	f.Acquired[i] = true
	if set {
		if e = f.Owner.numerals.SetUint64(&f.Values[i], n); e != nil {
			return e
		}
	}
	f.completed[i] = true
	f.tokens[i] = f.Values[i]
	return nil
}
func (f *astArrayCardinalityFrame) destroy(i int, own bool) error {
	if !f.Acquired[i] || !own && !f.completed[i] {
		return nil
	}
	e := f.Owner.numerals.Del(&f.Values[i])
	if e != nil {
		return e
	}
	f.Acquired[i] = false
	f.completed[i] = false
	return nil
}
func (f *astArrayCardinalityFrame) sourceEnd() error {
	if f.sourceDone {
		return f.sourceCause
	}
	if f.sourceCause != nil {
		return f.sourceCause
	}
	for i := 3; i >= 0; i-- {
		if e := f.destroy(i, false); e != nil {
			f.sourceCause = e
			return e
		}
	}
	f.sourceDone = true
	return nil
}
func (m *ASTManager) cleanupArrayCardinality() error {
	for f := m.ArrayCardinality; f != nil; f = f.Next {
		if f.Self != f || f.Owner != m || f.Invocation == 0 {
			return ErrASTHandle
		}
		for i := 3; i >= 0; i-- {
			if e := f.destroy(i, true); e != nil {
				if f.ownCause == nil {
					f.ownCause = e
				}
				return e
			}
		}
	}
	return nil
}
func (m *ASTManager) releaseArrayCardinalityHistory() {
	for f := m.ArrayCardinality; f != nil; {
		next := f.Next
		for _, a := range f.Acquired {
			if a {
				panic(ErrASTPending)
			}
		}
		m.BudgetField.Release(f.header)
		f.header = 0
		f = next
	}
	m.ArrayCardinality = nil
}
func (m *ASTManager) finishArrayCardinality(f *astArrayCardinalityFrame, body error) error {
	if f.bodyCause == nil {
		f.bodyCause = body
	}
	source := f.sourceEnd()
	// A clean completed lexical scope has no remaining acquisition or diagnostic
	// history. Failed scopes stay named until actual enclosing owner completion.
	if body == nil && source == nil {
		for _, a := range f.Acquired {
			if a {
				return ErrASTPending
			}
		}
		if m.ArrayCardinality != f {
			return ErrASTPending
		}
		m.ArrayCardinality = f.Next
		m.BudgetField.Release(f.header)
		f.header = 0
	}
	return errors.Join(body, source)
}
func (m *ASTManager) finiteArraySize(ps astArrayParameters) (SortSize, *astArrayCardinalityFrame, error) {
	f, e := m.newArrayCardinalityFrame()
	if e != nil {
		return SortSize{}, f, e
	}
	if e = f.construct(0, 1, true); e != nil {
		return SortSize{}, f, e
	}
	if e = f.construct(1, 0, false); e != nil {
		return SortSize{}, f, e
	}
	for i := 0; i < ps.count()-1; i++ {
		p, e := ps.at(m, i)
		if e != nil {
			return SortSize{}, f, e
		}
		info, e := m.sortInfo(m.sortAt(p.ast.slot).info)
		if e != nil {
			return SortSize{}, f, e
		}
		n, e := info.size.Size()
		if e != nil {
			return SortSize{}, f, e
		}
		if e = f.construct(2, n, true); e != nil {
			return SortSize{}, f, e
		}
		body := m.numerals.MulAssign(&f.Values[0], &f.Values[2])
		dt := f.destroy(2, false)
		if dt != nil && f.sourceCause == nil {
			f.sourceCause = dt
		}
		if e = errors.Join(body, dt); e != nil {
			return SortSize{}, f, e
		}
	}
	large, e := m.numerals.GreaterInt(&f.Values[0], 128)
	if e != nil {
		return SortSize{}, f, e
	}
	if !large {
		p, e := ps.at(m, ps.count()-1)
		if e != nil {
			return SortSize{}, f, e
		}
		info, e := m.sortInfo(m.sortAt(p.ast.slot).info)
		if e != nil {
			return SortSize{}, f, e
		}
		n, e := info.size.Size()
		if e != nil {
			return SortSize{}, f, e
		}
		if e = m.numerals.SetUint64(&f.Values[1], n); e != nil {
			return SortSize{}, f, e
		}
		exponent, e := m.numerals.Int64(&f.Values[0])
		if e != nil {
			return SortSize{}, f, e
		}
		result, e := m.numerals.Power(&f.Values[1], uint32(exponent))
		if e != nil {
			return SortSize{}, f, e
		}
		f.Values[3] = result
		f.tokens[3] = result
		f.Acquired[3] = true
		f.completed[3] = true
		body := m.numerals.Copy(&f.Values[1], &f.Values[3])
		dt := f.destroy(3, false)
		if dt != nil && f.sourceCause == nil {
			f.sourceCause = dt
		}
		if e = errors.Join(body, dt); e != nil {
			return SortSize{}, f, e
		}
	}
	// Source repeats the comparison after the conditional expression, then uses
	// short-circuit || before is_uint64. It does not reuse the earlier predicate.
	large, e = m.numerals.GreaterInt(&f.Values[0], 128)
	if e != nil {
		return SortSize{}, f, e
	}
	if large {
		return VeryBigSortSize(), f, nil
	}
	size, e := SortSizeFromRational(m.numerals, &f.Values[1])
	return size, f, e
}
