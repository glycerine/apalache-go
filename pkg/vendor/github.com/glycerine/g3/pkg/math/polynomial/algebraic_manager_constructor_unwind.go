package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:64-125,2772-2780.

// unwindConstructorSource invokes only completed declarations. A failed
// polynomial child has already unwound its own completed declarations; its
// retained cold owner is not a completed polynomial manager member.
func (m *AlgebraicManager) unwindConstructorSource() error {
	if m == nil || m.self != m || m.completed || m.originalCause == nil {
		return ErrAlgebraicOwner
	}
	if m.sourceRunning || m.constructing || m.finishing {
		return ErrAlgebraicPending
	}
	if m.unwindStarted {
		return m.unwindCause
	}
	m.unwindStarted, m.sourceRunning = true, true
	defer func() { m.sourceRunning = false }()
	fail := func(e error) error { m.unwindCause = e; return e }
	// A constructor-body lexical helper must finish its own source destruction
	// before older members. Owning CloseHelpers is not a source retry.
	if m.params.Helpers != nil {
		if e := m.params.DestructorCause; e != nil {
			return fail(e)
		}
		return fail(ErrAlgebraicPending)
	}
	if e := m.bindSourceDestructor(); e != nil {
		return fail(e)
	}
	defer m.unbindSourceDestructor()
	for m.unwindStage < 14 {
		if e := m.sourceMember(m.unwindStage, &m.unwindIndex, true); e != nil {
			return fail(e)
		}
		m.unwindStage++
		m.unwindIndex = 0
	}
	// manager's own constructor did not finish. Its raw allocator pointer and
	// imp/Go owner allocations have no outer ~manager invocation here.
	m.unwindFinalized = true
	return nil
}

// The same concrete source members occur in normal destruction and in the
// constructed prefix of a failed constructor. Presence and the actual child
// constructor flag select that prefix; neither path manufactures completion.
func (m *AlgebraicManager) sourceMember(stage uint8, index *uint32, constructorUnwind bool) error {
	c := m.AlgebraicRootCore
	switch stage {
	case 0:
		if c != nil {
			return m.sourceScopedPolynomial(&c.addTmp, index)
		}
	case 1, 2, 3:
		if c != nil {
			vs := []**BinaryRationalVector{&c.uppers, &c.lowers, &c.roots}
			v := vs[stage-1]
			if *v != nil {
				if e := (*v).Close(); e != nil {
					return e
				}
				*v = nil
			}
		}
	case 4:
		if c != nil && c.factors != nil {
			return c.factors.SourceFinalize()
		}
	case 5, 6, 7, 8:
		if c != nil {
			ps := []*UPolynomial{&c.evalSignTmp, &c.tmp3, &c.tmp2, &c.isolateTmp1}
			return m.sourceScopedPolynomial(ps[stage-5], index)
		}
	case 9:
		if c != nil && m.rationalTemporaryConstructed {
			return m.sourceRationalTemporary()
		}
	case 10:
		if m.univariate != nil {
			return m.univariate.SourceFinalize()
		}
	case 11:
		if m.polynomial != nil {
			if constructorUnwind && !m.polynomialConstructed {
				return m.polynomial.unwindCause
			}
			return m.polynomial.SourceFinalize()
		}
	case 12:
		if m.intervals != nil {
			return m.intervals.SourceFinalize()
		}
	case 13:
		if m.binary != nil {
			return m.binary.SourceFinalize()
		}
	default:
		return ErrAlgebraicOwner
	}
	return nil
}
