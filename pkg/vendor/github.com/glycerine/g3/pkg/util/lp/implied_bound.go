package lp

// Source: src/util/lp/implied_bound.h; lar_constraints.h:flip_kind and
// lconstraint_kind_string; ul_pair.h:lconstraint_kind.

import "github.com/glycerine/g3/pkg/util/numeral"

// LConstraintKind retains the source encoding: negation reverses a relation,
// and division by two makes an inequality strict.
type LConstraintKind int32

const (
	LConstraintLE LConstraintKind = -2
	LConstraintLT LConstraintKind = -1
	LConstraintEQ LConstraintKind = 0
	LConstraintGT LConstraintKind = 1
	LConstraintGE LConstraintKind = 2
)

func FlipConstraintKind(kind LConstraintKind) LConstraintKind { return -kind }

func LConstraintKindString(kind LConstraintKind) (string, error) {
	switch kind {
	case LConstraintLE:
		return "<=", nil
	case LConstraintLT:
		return "<", nil
	case LConstraintGE:
		return ">=", nil
	case LConstraintGT:
		return ">", nil
	case LConstraintEQ:
		return "=", nil
	}
	return "", sourceAssertionFailure()
}

// ImpliedBound owns Bound in a borrowed numeral manager. Its evidence describes
// the column, direction, coefficient sign, originating row/term and strictness.
// Do not copy this owner. Close also releases a stopped construction call.
type ImpliedBound struct {
	Bound                 numeral.Rational
	Column                uint32
	IsLowBound            bool
	CoefficientIsPositive bool
	RowOrTermIndex        uint32
	Strict                bool
	manager               *numeral.RationalManager
	calls                 exactEtaCalls
	stopped               bool
}

// NewImpliedBound returns an owner even on error; the caller must close it.
// Source member order copies Bound before publishing the evidence fields.
func NewImpliedBound(m *numeral.RationalManager, bound *numeral.Rational, column uint32, low, coefficientPositive bool, rowOrTerm uint32, strict bool) (*ImpliedBound, error) {
	b := &ImpliedBound{Bound: numeral.NewRational(), manager: m}
	before, err := b.calls.begin(m)
	if err != nil {
		return b, err
	}
	err = b.calls.after(m, before, m.Copy(&b.Bound, bound))
	if err != nil {
		b.stopped = true
		return b, err
	}
	b.Column, b.IsLowBound = column, low
	b.CoefficientIsPositive, b.RowOrTermIndex, b.Strict = coefficientPositive, rowOrTerm, strict
	return b, nil
}

func (b *ImpliedBound) Kind() LConstraintKind {
	kind := LConstraintLE
	if b.IsLowBound {
		kind = LConstraintGE
	}
	if b.Strict {
		kind /= 2
	}
	return kind
}

func (b *ImpliedBound) Equal(other *ImpliedBound) (bool, error) {
	if b.manager != other.manager {
		return false, ErrExactVectorOwner
	}
	if b.stopped || other.stopped || b.calls.pending() || other.calls.pending() {
		return false, ErrExactVectorPending
	}
	// Keep the source short-circuit order, including the rational comparison
	// before the remaining three evidence fields.
	if b.Column != other.Column || b.IsLowBound != other.IsLowBound {
		return false, nil
	}
	equal, err := b.manager.Equal(&b.Bound, &other.Bound)
	if err != nil || !equal {
		return false, err
	}
	return b.CoefficientIsPositive == other.CoefficientIsPositive && b.RowOrTermIndex == other.RowOrTermIndex && b.Strict == other.Strict, nil
}

// Assign copies the source-generated assignment used when the propagator
// replaces a weaker bound. Evidence is published only after the owning copy.
func (b *ImpliedBound) Assign(other *ImpliedBound) error {
	if b.manager != other.manager {
		return ErrExactVectorOwner
	}
	if b.stopped || other.stopped || other.calls.pending() {
		return ErrExactVectorPending
	}
	before, err := b.calls.begin(b.manager)
	if err != nil {
		return err
	}
	err = b.calls.after(b.manager, before, b.manager.Copy(&b.Bound, &other.Bound))
	if err != nil {
		b.stopped = true
		return err
	}
	b.Column, b.IsLowBound = other.Column, other.IsLowBound
	b.CoefficientIsPositive, b.RowOrTermIndex, b.Strict = other.CoefficientIsPositive, other.RowOrTermIndex, other.Strict
	return nil
}

func (b *ImpliedBound) Close() error {
	if err := b.calls.cleanupRaw(b.manager); err != nil {
		return err
	}
	if err := b.manager.Del(&b.Bound); err != nil {
		return err
	}
	b.Bound = numeral.NewRational()
	if err := b.calls.cleanupRoots(b.manager); err != nil {
		return err
	}
	b.stopped = false
	return nil
}
