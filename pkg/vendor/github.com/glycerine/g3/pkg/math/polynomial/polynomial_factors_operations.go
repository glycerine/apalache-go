package polynomial

// Source: src/math/polynomial/polynomial.cpp:1640-1673,6577-6590;
// src/util/obj_ref.h; src/util/scoped_numeral.h.

import (
	"errors"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (f *PolynomialFactors) invoke(body func() error) error {
	m := f.manager.core
	previous := m.factorsOwner
	m.factorsOwner = f
	f.executing = true
	defer func() { f.executing = false; m.factorsOwner = previous }()
	return body()
}
func (f *PolynomialFactors) finishCurrent(body error) error {
	m := f.manager.core
	if f.bodyCause == nil {
		f.bodyCause = body
	}
	if f.powerGeneration != 0 && m.powerLocal.active && m.powerLocal.generation == f.powerGeneration && m.powerLocal.cleanupCause != nil {
		f.stopped = true
		if f.sourceCause == nil {
			f.sourceCause = m.powerLocal.cleanupCause
		}
	}
	if f.constantGeneration != 0 && m.constantGeneration == f.constantGeneration && m.constantScope.active && m.constantScope.stopped {
		f.stopped = true
		if f.sourceCause == nil {
			f.sourceCause = m.constantScope.sourceCause
		}
	}
	if f.stopped {
		return errors.Join(body, f.sourceCause)
	}
	if f.currentActive {
		if e := f.current.closeValue(m, false); e != nil {
			f.stopped = true
			f.sourceCause = e
			return errors.Join(body, e)
		}
		if f.current.incoming == (Polynomial{}) && !f.current.assigning {
			f.currentActive = false
		}
	}
	return body
}
func (f *PolynomialFactors) Multiply(out *PolynomialRef) (cause error) {
	if e := f.open(); e != nil {
		return e
	}
	m := f.manager.core
	if out == nil || out.owner != m {
		return ErrPolynomialOwner
	}
	if e := out.open(); e != nil {
		return e
	}
	n, e := f.sourceSize()
	if e != nil {
		return e
	}
	f.generation++
	f.multiplyActive = true
	// executing is temporarily suspended at external cooperation boundaries
	// so their independent primitive work is not attributed to this caller.
	// Its source/owning lifetime remains protected through the final current DT.
	f.multiplyRunning = true
	defer func() { f.multiplyRunning = false }()
	defer func() {
		cause = f.finishCurrent(cause)
		if cause == nil {
			f.multiplyActive = false
			f.freshResult = Polynomial{}
			f.powerGeneration = 0
			f.constantGeneration = 0
		}
	}()
	return f.invoke(func() error {
		if n == 0 {
			p, e := m.MkConstantTakeCopy(&f.constant)
			if e != nil {
				return e
			}
			return out.root.assign(m, p)
		}
		for i := uint32(0); ; i++ {
			size, e := f.sourceSize()
			if e != nil {
				return e
			}
			if i >= size {
				break
			}
			p, e := f.factor(i)
			if e != nil {
				return e
			}
			f.current = polynomialReference{}
			f.currentActive = true
			if e = f.current.assign(m, p); e != nil {
				return e
			}
			d, e := f.degree(i)
			if e != nil {
				return e
			}
			if d > 1 {
				// The source evaluates m_degrees[i] again for the pw argument.
				d, e = f.degree(i)
				if e != nil {
					return e
				}
				if e = m.powerInto(f.current.value, d, &f.current); e != nil {
					return e
				}
			}
			if i == 0 {
				e = out.root.assign(m, f.current.value)
			} else {
				var q Polynomial
				q, e = m.Mul(out.root.value, f.current.value)
				if e == nil {
					e = out.root.assign(m, q)
				}
			}
			if e != nil {
				return e
			}
			if e = f.finishCurrent(nil); e != nil {
				return e
			}
		}
		u, e := m.monomials.Unit()
		if e != nil {
			return e
		}
		p, e := m.Scale(&f.constant, u, out.root.value)
		if e != nil {
			return e
		}
		return out.root.assign(m, p)
	})
}
func (f *PolynomialFactors) cleanupMultiply() error {
	m := f.manager.core
	previous := m.factorsOwner
	m.factorsOwner = f
	defer func() { m.factorsOwner = previous }()
	if frame := f.displayReceipt; frame != nil && frame.generation == f.displayGeneration && frame.active && frame.sink == nil && frame.monomialSink == nil && !frame.inserting {
		if e := m.cleanupDisplayFrame(frame); e != nil {
			return e
		}
	}
	if f.constantGeneration != 0 && m.constantGeneration == f.constantGeneration && m.constantScope.active {
		if e := m.cleanupConstantScope(); e != nil {
			return e
		}
	}
	if f.powerGeneration != 0 && m.powerLocal.active && m.powerLocal.generation == f.powerGeneration {
		if e := m.CleanupPolynomialPower(); e != nil {
			return e
		}
	}
	if f.currentActive {
		if e := f.current.close(m); e != nil {
			return e
		}
		f.currentActive = false
	}
	if f.freshResult != (Polynomial{}) {
		if e := m.CleanupValue(&f.freshResult); e != nil {
			return e
		}
	}
	if f.factoryGeneration != 0 && m.pending.generation == f.factoryGeneration && m.pending.factorsParent == f {
		if e := m.cleanupPendingCreation(); e != nil {
			return e
		}
	}

	if f.singleton != (Monomial{}) {
		if f.singletonLast {
			if e := m.monomials.CleanupValue(&f.singleton); e != nil {
				return e
			}
			f.singletonLast = false
		} else if m.singletonReference && m.singletonMonomial == f.singleton && m.singletonGeneration == f.singletonGeneration {
			if e := m.monomials.check(); e != nil {
				return e
			}
			outcome := monomialDecrementOutcome{}
			e := m.monomials.decRefCoreOutcome(f.singleton, &outcome)
			if outcome.written {
				m.singletonReference = false
				m.singletonMonomial = Monomial{}
				if outcome.last && e != nil {
					f.singletonLast = true
				} else {
					f.singleton = Monomial{}
				}
			}
			if e != nil {
				return e
			}
		} else {
			f.singleton = Monomial{}
		}
	}

	if f.generated != (Monomial{}) && m.generatedActive && m.generatedMonomial == f.generated {
		if e := m.cleanupGeneratedMonomial(); e != nil {
			return e
		}
		f.generated = Monomial{}
	}
	if f.calleeGeneration != 0 && m.generatedCalleePending && m.generatedCalleeGeneration == f.calleeGeneration && m.monomials.creation.generation == f.calleeGeneration && m.monomials.creation.parent == m {
		if e := m.monomials.CleanupCreation(); e != nil {
			return e
		}
	}
	if f.somOwned && m.som.factorsParent == f && m.som.factorsGeneration == f.generation {
		if e := m.som.close(); e != nil {
			return e
		}
		// Explicit owning abort destroyed the concrete adapter state. Initialize
		// a fresh default member only after all actual previous roots were drained.
		m.som = polynomialSOM{}
		m.som.init(m, false)
		f.somOwned = false
	}
	if f.cheapOwned && m.cheap.factorsParent == f && m.cheap.factorsGeneration == f.generation {
		if e := m.cheap.close(); e != nil {
			return e
		}
		m.cheap = polynomialSOM{}
		m.cheap.init(m, true)
		f.cheapOwned = false
	}
	if f.localActive {
		if e := m.integer.Del(&f.local); e != nil {
			return e
		}
		f.localActive = false
	}
	if e := f.numeric.cleanup(m.integer); e != nil {
		return e
	}
	if f.monomialRaw != ([3]umemory.Block{}) {
		now, e := m.monomials.allocator.PendingRoots()
		if e != nil {
			return e
		}
		if now == f.monomialRaw {
			e = m.monomials.allocator.CleanupPending()
			after, x := m.monomials.allocator.PendingRoots()
			f.monomialRaw = after
			if e != nil || x != nil {
				return errors.Join(e, x)
			}
		} else {
			f.monomialRaw = [3]umemory.Block{}
		}
	}
	f.multiplyActive = false
	return nil
}
func (f *PolynomialFactors) DisplayInto(sink PolynomialTextSink) (cause error) {
	if e := f.open(); e != nil {
		return e
	}
	if sink == nil {
		return ErrPolynomialOwner
	}
	m := f.manager.core
	previous := m.factorsOwner
	m.factorsOwner = f
	f.executing = true
	defer func() { f.executing = false; m.factorsOwner = previous }()
	frame, e := m.displayFrame(sink)
	if e != nil {
		return e
	}
	defer func() { cause = frame.finish(cause) }()
	text, e := frame.stringInteger(&f.constant)
	if e != nil {
		return e
	}
	if e = frame.WriteLiteral(text); e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		n, e := f.sourceSize()
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		if e = frame.WriteLiteral(" * ("); e != nil {
			return e
		}
		p, e := f.factor(i)
		if e != nil {
			return e
		}
		if e = f.manager.DisplayInto(p, sink, false); e != nil {
			return e
		}
		if e = frame.WriteLiteral(")^"); e != nil {
			return e
		}
		d, e := f.degree(i)
		if e != nil {
			return e
		}
		if e = frame.WriteUnsigned(d); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialManager) AccumulateFactorConstant(f *PolynomialFactors, c *unumeral.Int) error {
	return m.accumulateFactorConstantWithReceipt(f, c, nil)
}
func (m *MultivariatePolynomialManager) FlipFactorSign(f *PolynomialFactors) error {
	return m.flipFactorSignWithReceipt(f, nil)
}
func (f *PolynomialFactors) finishLocal(body error) error {
	if f.bodyCause == nil {
		f.bodyCause = body
	}
	if f.stopped {
		return errors.Join(body, f.sourceCause)
	}
	if f.localActive {
		if _, e := upolynomialSourceNumeralDel(f.manager.core.integer, &f.local); e != nil {
			f.stopped = true
			f.sourceCause = e
			return errors.Join(body, e)
		}
		f.localActive = false
	}
	return body
}
