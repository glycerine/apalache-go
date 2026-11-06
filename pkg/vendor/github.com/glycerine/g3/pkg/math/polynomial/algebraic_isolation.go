package polynomial

// algebraic_numbers.cpp:541–684, real univariate factor/isolation pipeline.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *AlgebraicRootCore) checkpoint() error {
	if !m.config.Checkpoint() {
		return newAlgebraicCancellation()
	}
	return m.config.Cooperation()
}
func (m *AlgebraicRootCore) ensureIsolation() error {
	var e error
	if m.factors == nil {
		m.factors, e = m.config.Polynomials.NewFactors()
		if e != nil {
			return e
		}
	}
	if m.roots == nil {
		m.roots, e = numericBinaryRationalManager_NewVector(m.config.Binary)
		if e != nil {
			return e
		}
	}
	if m.lowers == nil {
		m.lowers, e = numericBinaryRationalManager_NewVector(m.config.Binary)
		if e != nil {
			return e
		}
	}
	if m.uppers == nil {
		m.uppers, e = numericBinaryRationalManager_NewVector(m.config.Binary)
		if e != nil {
			return e
		}
	}
	return nil
}
func (m *AlgebraicRootCore) factorPolynomial(v UPolynomialView) (bool, error) {
	return m.factorPolynomialIntoWithSquareFreeReceipt(v, m.factors, &m.isolationSquareFree)
}
func (m *AlgebraicRootCore) factorPolynomialInto(v UPolynomialView, factors *UPolynomialFactors) (bool, error) {
	return m.factorPolynomialIntoWithSquareFreeReceipt(v, factors, nil)
}
func (m *AlgebraicRootCore) factorPolynomialIntoWithSquareFreeReceipt(v UPolynomialView, factors *UPolynomialFactors, r *upolySquareFreeCallReceipt) (bool, error) {
	if m.factorExecuting {
		return false, ErrAlgebraicPending
	}
	m.factorExecuting = true
	defer func() { m.factorExecuting = false }()

	if m.factor {
		return m.config.Factors.Factor(v, factors, m.factorParams)
	}
	if m.tmp3.slot == 0 {
		var e error
		m.tmp3, e = m.config.Polynomials.NewPolynomial()
		if e != nil {
			return false, e
		}
	}
	if e := m.config.Polynomials.Reset(m.tmp3); e != nil {
		return false, e
	}
	squareFreeError := m.config.Polynomials.squareFreeViewWithReceipt(v, m.tmp3, r)
	if e := squareFreeError; e != nil {
		return false, e
	}
	w, e := m.config.Polynomials.wholeView(m.tmp3)
	if e != nil {
		return false, e
	}
	// The trace owns its callback borrow while this private helper enters it.
	m.factorExecuting = false
	traceError := m.traceIsolationPolynomial(w, true)
	m.factorExecuting = true
	if e = traceError; e != nil {
		return false, e
	}
	return false, factors.PushCopy(w, 1)
}
func (m *AlgebraicRootCore) pushNew(v *AlgebraicVector, a *Algebraic) error {
	if e := v.pushTake(a); e != nil {
		if a.slot != 0 {
			m.pending = *a
			l := &m.locations[a.slot-1]
			if l.published {
				l.published = false
				m.callers--
			}
			*a = Algebraic{}
		}
		return e
	}
	return nil
}
func (m *AlgebraicRootCore) IsolateRoots(v UPolynomialView, out *AlgebraicVector) (err error) {
	return m.isolateRootsForPolynomial(v, out, nil)
}
func (m *AlgebraicRootCore) isolateRootsForPolynomial(v UPolynomialView, out *AlgebraicVector, invocation *algebraicIsolationConversionScope) (err error) {
	if err = m.check(); err != nil {
		return err
	}
	if out == nil || out.core != m {
		return ErrAlgebraicOwner
	}
	if err = out.check(); err != nil {
		return err
	}
	if m.univariateIsolationRunning {
		return ErrAlgebraicPending
	}
	if err = m.beginIsolationNumeric(); err != nil {
		return err
	}
	defer func() { err = m.finishIsolationNumeric(err) }()
	m.univariateIsolationRunning = true
	m.univariateIsolationOutput = out
	defer func() { m.univariateIsolationRunning = false; m.univariateIsolationOutput = nil }()
	if err = m.traceIsolationPolynomial(v, false); err != nil {
		return err
	}
	if v.count == 0 {
		return nil
	}
	if m.pending.slot != 0 {
		return ErrAlgebraicPending
	}
	if err = m.ensureIsolation(); err != nil {
		return err
	}
	if err = m.factors.Reset(); err != nil {
		return err
	}
	zero, err := m.config.Polynomials.HasZeroRoots(v)
	if err != nil {
		return err
	}
	if zero {
		a := Algebraic{}
		if err = out.pushTake(&a); err != nil {
			return err
		}
		if m.tmp2.slot == 0 {
			m.tmp2, err = m.config.Polynomials.NewPolynomial()
			if err != nil {
				return err
			}
		}
		if err = m.config.Polynomials.RemoveZeroRoots(v, m.tmp2); err != nil {
			return err
		}
		v, err = m.config.Polynomials.wholeView(m.tmp2)
		if err != nil {
			return err
		}
	}
	before := m.config.Factors.frontend.receipt
	full, err := m.factorPolynomial(v)
	frontend := &m.config.Factors.frontend
	if invocation != nil && frontend.receipt != before && frontend.receipt.invocation != 0 && frontend.result == m.factors {
		invocation.factor = frontend.receipt
	}
	if err != nil {
		return err
	}
	n, err := m.factors.DistinctFactors()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		f, e := m.factors.FactorView(i)
		if e != nil {
			return e
		}
		degree := upolyViewDegree(f)
		if err = m.traceIsolationFactor(i, degree); err != nil {
			return err
		}
		if degree == 0 {
			continue
		}
		if err = m.beginQ(); err != nil {
			return err
		}
		if f.count == 2 {
			if err = m.traceIsolationLinear(); err != nil {
				return m.finishQ(err)
			}
			x, e := m.config.Polynomials.read(f, 0)
			if e != nil {
				return m.finishQ(e)
			}
			y, e := m.config.Polynomials.read(f, 1)
			if e != nil {
				return m.finishQ(e)
			}
			if err = m.config.Rationals.SetInteger(&m.localQ, x); err != nil {
				return m.finishQ(err)
			}
			if err = m.config.Rationals.DivInt(&m.localQ, y, &m.localQ); err != nil {
				return m.finishQ(err)
			}
			if err = m.config.Rationals.Neg(&m.localQ); err != nil {
				return m.finishQ(err)
			}
			var a Algebraic
			if err = m.SetTake(&a, &m.localQ); err != nil {
				return m.finishQ(err)
			}
			err = m.pushNew(out, &a)
			if err = m.finishQ(err); err != nil {
				return err
			}
			continue
		}
		if err = m.assertIsolationScratchEmpty(); err != nil {
			return m.finishQ(err)
		}
		if invocation != nil {
			invocation.scratchStarted = true
		}
		if err = m.config.Polynomials.SquareFreeIsolateRoots(f, m.config.Binary, m.roots, m.lowers, m.uppers); err != nil {
			return m.finishQ(err)
		}
		size := m.roots.state.size
		if err = m.traceIsolatedBasicRoots(size); err != nil {
			return m.finishQ(err)
		}
		for j := uint32(0); j < size; j++ {
			b := &m.roots.state.values[j]
			if err = m.config.Binary.ToRational(m.config.Rationals, b, &m.localQ); err != nil {
				return m.finishQ(err)
			}
			var a Algebraic
			if err = m.SetTake(&a, &m.localQ); err != nil {
				return m.finishQ(err)
			}
			if err = m.pushNew(out, &a); err != nil {
				return m.finishQ(err)
			}
		}
		if err = m.assertIsolationBoundsSizes(); err != nil {
			return m.finishQ(err)
		}
		size = m.lowers.state.size
		for j := uint32(0); j < size; j++ {
			if j >= m.uppers.state.size {
				return m.finishQ(arena.ErrBounds)
			}
			lo, hi := &m.lowers.state.values[j], &m.uppers.state.values[j]
			refinement := &m.isolateRefinable
			if invocation != nil {
				refinement = &invocation.refinable
			}
			irr, e := m.config.Polynomials.isolatingToRefinableWithReceipt(f, m.config.Binary, lo, hi, refinement)
			if e != nil {
				if stop := m.config.Polynomials.refinableReceiptSourceCause(refinement); stop != nil {
					return errors.Join(e, stop)
				}
				return m.finishQ(e)
			}
			var a Algebraic
			if irr {
				a, err = m.mkAlgebraicCell(f, lo, hi, full)
			} else {
				err = m.config.Binary.ToRational(m.config.Rationals, lo, &m.localQ)
				if err == nil {
					err = m.SetTake(&a, &m.localQ)
				}
			}
			if err != nil {
				return m.finishQ(err)
			}
			if err = m.pushNew(out, &a); err != nil {
				return m.finishQ(err)
			}
		}
		if err = m.roots.Reset(); err != nil {
			return m.finishQ(err)
		}
		if err = m.lowers.Reset(); err != nil {
			return m.finishQ(err)
		}
		if err = m.uppers.Reset(); err != nil {
			return m.finishQ(err)
		}
		if err = m.finishQ(nil); err != nil {
			return err
		}
	}
	// Concrete heap sort is the documented external std::sort backend adapter;
	// it sorts the entire append-only output without extra heap scratch.
	for start := len(out.values)/2 - 1; start >= 0; start-- {
		if err = m.rootSift(out, uint32(start), uint32(len(out.values))); err != nil {
			return err
		}
	}
	for end := uint32(len(out.values)); end > 1; end-- {
		out.values[0], out.values[end-1] = out.values[end-1], out.values[0]
		if err = m.rootSift(out, 0, end-1); err != nil {
			return err
		}
	}
	return nil
}
func (m *AlgebraicRootCore) rootSift(v *AlgebraicVector, root, end uint32) error {
	for uint64(root)*2+1 < uint64(end) {
		child := root*2 + 1
		if child+1 < end {
			d, e := m.Compare(&v.values[child], &v.values[child+1])
			if e != nil {
				return e
			}
			if d < 0 {
				child++
			}
		}
		d, e := m.Compare(&v.values[root], &v.values[child])
		if e != nil {
			return e
		}
		if d >= 0 {
			return nil
		}
		v.values[root], v.values[child] = v.values[child], v.values[root]
		root = child
	}
	return nil
}
func (m *AlgebraicRootCore) MkRoot(v UPolynomialView, index uint32, out *Algebraic) (err error) {
	return m.mkRootWithConstruction(v, index, out, nil)
}
func (m *AlgebraicRootCore) mkRootWithConstruction(v UPolynomialView, index uint32, out *Algebraic, caller *algebraicRootConstructionScope) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if m.rootConstruction.active && caller != &m.rootConstruction {
		return ErrAlgebraicPending
	}
	if index == 0 {
		return newAlgebraicException("invalid root object, root index must be greater than 0")
	}
	if v.count == 0 {
		return newAlgebraicException("invalid root object, polynomial must not be the zero polynomial")
	}
	if m.rootSelection != nil {
		return ErrAlgebraicPending
	}
	r, err := (m).NewVector()
	if err != nil {
		return err
	}
	m.rootSelection = r
	if caller != nil {
		caller.selection = r
	}
	defer func() {
		if caller != nil {
			if stop := caller.childSourceCause(); stop != nil {
				caller.sourceStop = stop
				err = errors.Join(err, stop)
				return
			}
		}
		e := r.Close()
		if e == nil {
			m.rootSelection = nil
		} else {
			err = errors.Join(err, e)
		}
	}()
	var conversion *algebraicIsolationConversionScope
	if caller != nil {
		conversion = &caller.conversion
	}
	if err = m.isolateRootsForPolynomial(v, r, conversion); err != nil {
		return err
	}
	count := uint32(len(r.values))
	if err = m.traceRootSelection(r, count); err != nil {
		return err
	}
	if index > count {
		return newAlgebraicException("invalid root object, polynomial does have sufficient roots")
	}
	return m.Copy(out, &r.values[index-1])
}

// CleanupIsolation retries only explicitly retained source lexical/vector roots.
func (m *AlgebraicRootCore) CleanupIsolation() error {
	if m != nil && m.self == m && (m.comparisonTraceRunning || m.rootTrace.running || m.rootTracePending() || m.rootDisplay.active || m.coefficientNormalizationPending() || m.univariateIsolationRunning) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.manager != nil && (m.manager.constructing || m.manager.finishing) {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && m.rootConstruction.active {
		return ErrAlgebraicPending
	}
	if m != nil && m.self == m && (m.rootConstruction.running || m.rootConstruction.finishing) {
		return ErrAlgebraicPending
	}
	if m.rootSelection != nil {
		if e := m.rootSelection.Close(); e != nil {
			return e
		}
		m.rootSelection = nil
	}
	if m.roots != nil {
		if e := m.roots.Reset(); e != nil {
			return e
		}
	}
	if m.lowers != nil {
		if e := m.lowers.Reset(); e != nil {
			return e
		}
	}
	if m.uppers != nil {
		if e := m.uppers.Reset(); e != nil {
			return e
		}
	}
	return nil
}

var _ = arena.ErrBounds
