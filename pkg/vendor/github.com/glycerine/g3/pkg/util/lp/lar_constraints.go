package lp

// Source: src/util/lp/lar_constraints.h. Retained variable/term bounds and
// the owning coefficient constraint consumed by quick_xplain.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARBaseConstraint owns its RHS in a borrowed numeral manager. Derived
// constraints embed this owner; do not copy it or its owning rational.
type LARBaseConstraint struct {
	Kind      LConstraintKind
	RightSide numeral.Rational
	manager   *numeral.RationalManager
	calls     exactEtaCalls
	stopped   bool
}

func newLARBaseConstraint(m *numeral.RationalManager, kind LConstraintKind) LARBaseConstraint {
	return LARBaseConstraint{Kind: kind, RightSide: numeral.NewRational(), manager: m}
}

func (c *LARBaseConstraint) ready() error {
	if c.stopped || c.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}

func (c *LARBaseConstraint) copyValue(out, value *numeral.Rational) error {
	if err := c.ready(); err != nil {
		return err
	}
	before, err := c.calls.begin(c.manager)
	if err != nil {
		return err
	}
	err = c.calls.after(c.manager, before, c.manager.Copy(out, value))
	if err != nil {
		c.stopped = true
	}
	return err
}

// FreeCoefficient copies the source zero return into initialized caller-owned
// out. The caller owns out even if the copy stops; Close owns call receipts.
func (c *LARBaseConstraint) FreeCoefficient(out *numeral.Rational) error {
	zero := numeral.NewRational()
	return c.copyValue(out, &zero)
}

func (c *LARBaseConstraint) Close() error {
	if err := c.calls.cleanupRaw(c.manager); err != nil {
		return err
	}
	if err := c.manager.Del(&c.RightSide); err != nil {
		return err
	}
	c.RightSide = numeral.NewRational()
	if err := c.calls.cleanupRoots(c.manager); err != nil {
		return err
	}
	c.stopped = false
	return nil
}

type LARVarConstraint struct {
	LARBaseConstraint
	Column uint32
}

// All constraint constructors return an owner on error for explicit cleanup.
// Kind is initialized before RHS; the derived fields publish after its copy.
func NewLARVarConstraint(m *numeral.RationalManager, column uint32, kind LConstraintKind, rhs *numeral.Rational) (*LARVarConstraint, error) {
	c := &LARVarConstraint{LARBaseConstraint: newLARBaseConstraint(m, kind)}
	if err := c.copyValue(&c.RightSide, rhs); err != nil {
		return c, err
	}
	c.Column = column
	return c, nil
}

func (c *LARVarConstraint) Size() uint32 { return 1 }

func (c *LARVarConstraint) CoefficientsAsVector() (*SparseRationalVector, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	v := NewSparseRationalVector(c.manager)
	one := numeral.NewRational()
	one.Num = numeral.NewInt(1)
	if err := v.appendTermCoefficient(c.Column, &one); err != nil {
		return v, err
	}
	return v, nil
}

// LARTermConstraint borrows Term, including its current coefficients and free
// coefficient. Close destroys only the independently owned RHS and call roots.
type LARTermConstraint struct {
	LARBaseConstraint
	Term *LARTerm
}

func NewLARTermConstraint(m *numeral.RationalManager, term *LARTerm, kind LConstraintKind, rhs *numeral.Rational) (*LARTermConstraint, error) {
	c := &LARTermConstraint{LARBaseConstraint: newLARBaseConstraint(m, kind)}
	if term.manager != m {
		return c, ErrExactVectorOwner
	}
	if err := c.copyValue(&c.RightSide, rhs); err != nil {
		return c, err
	}
	c.Term = term
	return c, nil
}

func (c *LARTermConstraint) Size() uint32 { return c.Term.Size() }

func (c *LARTermConstraint) CoefficientsAsVector() (*SparseRationalVector, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	return c.Term.CoefficientsAsVector()
}

func (c *LARTermConstraint) FreeCoefficient(out *numeral.Rational) error {
	if err := c.ready(); err != nil {
		return err
	}
	if err := c.Term.ready(); err != nil {
		return err
	}
	return c.copyValue(out, &c.Term.Value)
}

// LARConstraint owns an ordered coefficient vector: duplicates and explicit
// zero coefficients are retained, as in the source vector copy constructor.
type LARConstraint struct {
	LARBaseConstraint
	Coefficients *SparseRationalVector
}

func NewLARConstraint(m *numeral.RationalManager, coefficients []RationalSparseEntry, kind LConstraintKind, rhs *numeral.Rational) (*LARConstraint, error) {
	c := &LARConstraint{LARBaseConstraint: newLARBaseConstraint(m, kind)}
	if err := c.copyValue(&c.RightSide, rhs); err != nil {
		return c, err
	}
	v, err := copyLARCoefficients(m, coefficients)
	c.Coefficients = v
	if err != nil {
		c.stopped = true
	}
	return c, err
}

func (c *LARConstraint) Size() uint32 { return c.Coefficients.Size() }

func (c *LARConstraint) CoefficientsAsVector() (*SparseRationalVector, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	if err := c.Coefficients.ready(); err != nil {
		return nil, err
	}
	return copyLARCoefficients(c.manager, c.Coefficients.Data)
}

func (c *LARConstraint) Close() error {
	// Derived member destruction precedes the base's owning RHS destruction.
	if c.Coefficients != nil {
		if err := c.Coefficients.Close(); err != nil {
			return err
		}
		c.Coefficients = nil
	}
	return c.LARBaseConstraint.Close()
}

// Unlike make_pair/push_back, a vector copy constructs each coefficient once.
// The unpublished partial element stays in appendValues until explicit Close.
func copyLARCoefficients(m *numeral.RationalManager, entries []RationalSparseEntry) (*SparseRationalVector, error) {
	v := NewSparseRationalVector(m)
	for i := range entries {
		v.appendValues = NewIndexedRationalVector(m, 1)
		w := v.appendValues
		if err := w.copySlot(&w.Data[0], &entries[i].Value); err != nil {
			return v, err
		}
		v.Data = append(v.Data, RationalSparseEntry{Index: entries[i].Index, Value: w.Data[0]})
		w.Data[0] = numeral.NewRational()
		v.appendValues = nil
	}
	return v, nil
}

// LARStoredConstraint is the actual polymorphic lar_base_constraint surface.
// Base borrows its shared source fields; the returned owning vectors still
// require Close. Concrete constructors and destructors retain their lifetimes.
type LARStoredConstraint interface {
	Base() *LARBaseConstraint
	Size() uint32
	CoefficientsAsVector() (*SparseRationalVector, error)
	FreeCoefficient(*numeral.Rational) error
	Close() error
}

func (c *LARBaseConstraint) Base() *LARBaseConstraint { return c }
