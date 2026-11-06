package duality

// Source: duality.h:442-451 (Transformer::SubsetEq).
import (
	"errors"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SubsetEq aligns the other's parameters, then asks the borrowed auxiliary
// solver whether this formula and the negated other formula are inconsistent.
// Its temporary scope is removed on every Go error path as well as success.
func (t *Transformer) SubsetEq(other *Transformer, aux solver.Solver) (_ bool, result error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if aux == nil {
		return false, ErrWrapperOwner
	}
	m, err := aux.Manager()
	if err != nil {
		return false, err
	}
	if m != t.context.manager {
		return false, ErrContextMismatch
	}
	replacement, err := t.substituteParams(other)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&replacement)) }()
	right, err := m.RefGet(replacement)
	if err != nil {
		return false, err
	}
	notRight, err := m.MkNot(right)
	if err != nil {
		return false, err
	}
	negative, err := m.NewRef(notRight)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&negative)) }()
	left, err := m.RefGet(t.formula)
	if err != nil {
		return false, err
	}
	formula, err := m.MkAnd(left, notRight)
	if err != nil {
		return false, err
	}
	owned, err := m.NewRef(formula)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&owned)) }()
	if err = aux.Push(); err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, aux.Pop(1)) }()
	if err = aux.Assert(formula); err != nil {
		return false, err
	}
	status, err := aux.CheckSat(nil)
	return status == lbool.False, err
}
