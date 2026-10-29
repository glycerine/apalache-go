package numeral

// Source: src/util/mpz.h, mpq.h, mpbq.h, mpbqi.h, mpzzp.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	// ValidOwner verifies the address identity of a borrowed primitive owner.
)

func (m *IntegerManager) ValidOwner() bool        { return m != nil && m.self == m }
func (m *NaturalManager) ValidOwner() bool        { return m != nil && m.self == m }
func (m *IntegerModularManager) ValidOwner() bool { return m != nil && m.self == m }
func (m *RationalManager) ValidOwner() bool       { return m != nil && m.self == m }
func (m *BinaryRationalManager) ValidOwner() bool { return m != nil && m.self == m }
func (m *BinaryIntervalManager) ValidOwner() bool { return m != nil && m.self == m }
func (m *IntegerModularManager) Integers() *IntegerManager {
	if m == nil {
		return nil
	}
	return m.manager
}
func (m *BinaryRationalManager) Integers() *IntegerManager { return m.Z }
func (m *IntegerManager) Naturals() *NaturalManager        { return m.Natural }
func (m *IntegerManager) Budget() *arena.Budget            { return m.BudgetField }
func (m *RationalManager) Budget() *arena.Budget           { return m.BudgetField }
func (m *IntegerManager) Closed() bool                     { return m.closed }
func (m *NaturalManager) Closed() bool                     { return m.ClosedField }
func (m *IntegerModularManager) Closed() bool              { return m.closed }
func (m *RationalManager) Closed() bool                    { return m.closed }
func (m *BinaryRationalManager) Closed() bool              { return m.closed }
func (m *BinaryIntervalManager) Closed() bool              { return m.closed }

// Check validates the same primitive owner and source entry preconditions used
// by its numeric operations. Higher algorithms must validate borrowed values
// without duplicating utility manager internals.
func (m *IntegerManager) Check(a *Int) error                   { return m.check(a) }
func (m *IntegerManager) CheckCleanup(a *Int) error            { return m.checkCleanup(a) }
func (m *NaturalManager) Check() error                         { return m.check() }
func (m *IntegerModularManager) Check() error                  { return m.check() }
func (m *RationalManager) Check(a *Rational) error             { return m.check(a) }
func (m *BinaryRationalManager) Check(a *BinaryRational) error { return m.check(a) }
func (m *BinaryIntervalManager) Check(a *BinaryInterval) error { return m.check(a) }
func (m *RationalManager) CheckOwnerReady() error              { return m.checkOwnerReady() }

// BorrowBounds returns the actual numeric owner; callers validate the interval
// manager's entry preconditions before using its borrowed owner.
func (m *BinaryIntervalManager) BorrowBounds() *BinaryRationalManager {
	if m == nil {
		return nil
	}
	return m.bounds
}
func (m *BinaryRationalManager) Open() error { return m.open() }

// SourceReady validates a borrowed numeric owner before a higher source class
// enters its own algorithms or retains any new numeric fields.
func (m *IntegerManager) SourceReady() error {
	if m == nil || !m.ValidOwner() {
		return umemory.ErrRuntimeOwner
	}
	if m.closed || m.SourceFinalizing || m.constructionFailed {
		return arena.ErrClosed
	}
	return m.Memory.CheckOpen()
}
func (m *IntegerModularManager) BorrowModulus() *Int { return &m.P }
func (m *IntegerModularManager) IntegerMode() bool   { return m.z }
func IsIntegerRational(a *Rational) bool             { return isIntegerRational(a) }
func IsOneInteger(a *Int) bool                       { return isOneInteger(a) }

func (m *BinaryIntervalManager) Open() error { return m.open() }
