package qe

// Source: src/qe/qe_lite.cpp:227-237,1734,1755, arith_util::is_numeral.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// The destination belongs to the enclosing DER or FM call's numbers. This
// reader keeps the native copy invocation alive until its destination and
// detached allocations can be cleaned up together after an interrupted copy.
type liteNumeralRead struct {
	manager    *ast.ASTManager
	receiver   *numeral.OwnedRational
	invocation numeral.RationalServiceInvocation
	bytes      uint64
}

func newLiteNumeralRead(m *ast.ASTManager) (*liteNumeralRead, error) {
	bytes := uint64(reflect.TypeOf(liteNumeralRead{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	return &liteNumeralRead{manager: m, bytes: bytes}, nil
}

func (r *liteNumeralRead) retire() error {
	if err := r.invocation.SourceReady(); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.invocation = numeral.RationalServiceInvocation{}
	r.receiver = nil
	return nil
}

func (r *liteNumeralRead) close() error {
	if err := r.invocation.Cleanup(false); err != nil {
		return err
	}
	if r.receiver != nil && *r.receiver != (numeral.OwnedRational{}) {
		if err := r.manager.NumeralService().CleanupValue(r.receiver); err != nil {
			return err
		}
	}
	if err := r.invocation.Cleanup(true); err != nil {
		return err
	}
	if err := r.invocation.EndContext(); err != nil {
		return err
	}
	r.invocation = numeral.RationalServiceInvocation{}
	r.receiver = nil
	r.manager.Budget().Release(r.bytes)
	r.bytes = 0
	return nil
}

func (r *liteNumeralRead) read(family ast.FamilyID, h ast.ASTHandle, destination *numeral.OwnedRational) (bool, error) {
	r.receiver = destination
	var integer bool
	matched, err := r.manager.ArithmeticNumeralValueWithInvocation(family, h, destination, &integer, &r.invocation)
	if err == nil {
		err = r.retire()
	}
	return matched, err
}

func (c *equalityDERCall) readNumeral(h ast.ASTHandle, coefficient *numeral.OwnedRational) (bool, error) {
	if c.numeralRead == nil {
		var err error
		c.numeralRead, err = newLiteNumeralRead(c.owner.manager)
		if err != nil {
			return false, err
		}
	}
	return c.numeralRead.read(c.owner.arithmetic, h, coefficient)
}
