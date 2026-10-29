package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:440-476,509-539.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// This is Go lifetime metadata for the actual borrowed source core. Numeric
// caller bindings exist only on the synchronous execution/cleanup chain.
type algebraicForeignCopyScope struct {
	owner, source             *AlgebraicRootCore
	active, running, cleaning bool
	bindings                  [4]unumeral.NumericCallerBinding
	hooks                     unumeral.NumericInvocationHooks
	roots                     unumeral.NumericAllocationRoots
	boundCopy                 unumeral.IntegerCopyAllocationReceipt
}

func (s *algebraicForeignCopyScope) NumericExecuting() bool { return s.running || s.cleaning }
func (s *algebraicForeignCopyScope) NumericBound(executing bool) bool {
	m := s.owner
	return m != nil && m.self == m && !m.closed && s == &m.foreignCopy && s.active && (!executing || s.NumericExecuting())
}
func (s *algebraicForeignCopyScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || z != s.owner.config.Rationals.Integers() {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicForeignCopyScope) bind() error {
	a, b := s.source.config.Rationals.Integers(), s.owner.config.Rationals.Integers()
	for i := range s.bindings {
		var e error
		switch i {
		case 0:
			e = a.BindCaller(&s.bindings[i], s, 0)
		case 1:
			e = a.Natural.BindCaller(&s.bindings[i], s, 0)
		case 2:
			e = b.BindCaller(&s.bindings[i], s, 0)
		case 3:
			e = b.Natural.BindCaller(&s.bindings[i], s, 0)
		}
		if e != nil {
			for j := i - 1; j >= 0; j-- {
				if e := s.bindings[j].Unbind(); e != nil {
					panic(e)
				}
			}
			return e
		}
	}
	return nil
}
func (s *algebraicForeignCopyScope) unbind() {
	for i := len(s.bindings) - 1; i >= 0; i-- {
		if e := s.bindings[i].Unbind(); e != nil {
			panic(e)
		}
	}
}
func (s *algebraicForeignCopyScope) release() {
	s.source.foreignCopyBorrowers--
	s.source.config.Polynomials.algebraicCopyBorrowers--
	s.owner.config.Polynomials.algebraicCopyBorrowers--
	*s = algebraicForeignCopyScope{}
}

// CopyFromCore implements set(a,b) when b belongs to another concrete manager,
// as required by arith_decl_plugin::translate. Partial destination construction
// follows source order and remains caller-owned after failure.
func (m *AlgebraicRootCore) CopyFromCore(dst *Algebraic, source *AlgebraicRootCore, src *Algebraic) (err error) {
	if source == m {
		return m.Copy(dst, src)
	}
	dl, e := m.location(dst)
	if e != nil {
		return e
	}
	sl, e := source.location(src)
	if e != nil {
		return e
	}
	z := m.config.Rationals.Integers()
	if z.MatchesCopyAllocationInvocation(&m.coefficientCopy) || z.MatchesCopyAllocationInvocation(&m.rationalCopy) || m.constQ.active || m.localActive || m.pending.slot != 0 {
		return ErrAlgebraicPending
	}
	// Canonical zero can be the same Go numeral in both admitted cores.
	// cpp:510 returns before the selected branch assertion or source payload.
	if dst == src {
		return nil
	}
	// The selected cpp:514/518/528/533 branch asserts before payload reads.
	if _, _, e = algebraicAssertionSwitch(); e != nil {
		return e
	}
	s := &m.foreignCopy
	*s = algebraicForeignCopyScope{owner: m, source: source, active: true, running: true}
	s.hooks.Policy = s
	source.foreignCopyBorrowers++
	source.config.Polynomials.algebraicCopyBorrowers++
	m.config.Polynomials.algebraicCopyBorrowers++
	if e := s.bind(); e != nil {
		s.release()
		return e
	}
	defer func() {
		s.unbind()
		s.running = false
		if err == nil {
			s.release()
		}
	}()
	if sl == nil || sl.kind == 0 {
		// The root-to-basic branch deletes its cell before the const-mpq
		// overload constructs tmp. Other basic branches keep their cell.
		if dl != nil && dl.kind == 1 {
			if e = algebraicQueryTagAssertionSwitch(); e != nil {
				return e
			}
			if e := m.destroyCell(dst, dl, false); e != nil {
				return e
			}
			dl = nil
		}
		q, e := m.copySourceBasicValue(source, sl)
		if e != nil {
			return e
		}
		return m.copyForeignBasic(dst, dl, source, q)
	}
	if dl == nil || dl.kind == 0 {
		if dl != nil {
			if e = algebraicQueryTagAssertionSwitch(); e != nil {
				return e
			}
			if e := m.destroyCell(dst, dl, false); e != nil {
				return e
			}
		}
		next, e := m.allocateCell(1)
		if e != nil {
			if next.slot != 0 && m.locations[next.slot-1].live {
				m.pending = next
			}
			return e
		}
		m.publish(dst, next)
		dl = &m.locations[dst.slot-1]
	} else {
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		old, e := m.load(dl)
		if e != nil {
			return e
		}
		if e = m.deletePolynomial(dl, &old); e != nil {
			return e
		}
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		e = unumeral.SourceBinaryIntervalDel(m.config.Intervals, &old.interval)
		if e = m.save(dl, &old, e); e != nil {
			return e
		}
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
	}
	if e = algebraicQueryTagAssertionSwitch(); e != nil {
		return e
	}
	r, e := source.copySourcePolynomialFields(sl)
	if e != nil {
		return e
	}
	out, e := m.load(dl)
	if e != nil {
		return e
	}
	v, e := source.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = m.copyPolynomialFromOwner(dl, &out, source.config.Polynomials, v); e != nil {
		return e
	}
	r.interval, e = source.copySourceIntervalFields(sl)
	if e != nil {
		return e
	}
	context := unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
	e = m.config.Intervals.WithAllocationContext(context).CopyFromOwnerTracked(&out.interval, source.config.Intervals, &r.interval, &s.boundCopy)
	if e = m.save(dl, &out, e); e != nil {
		return e
	}
	out.flags, e = source.copySourceFlags(sl)
	if e != nil {
		return e
	}
	return m.store(dl, &out)
}

func (m *AlgebraicRootCore) copyForeignBasic(dst *Algebraic, dl *algebraicLocation, source *AlgebraicRootCore, q algebraicBasicValueView) (err error) {
	if err = m.beginConstQ(); err != nil {
		return err
	}
	defer func() { m.constQ.running = false; err = m.finishConstQ(err) }()
	s := &m.constQ
	s.executing = true
	err = q.copyTo(m, &m.localQ, &s.copy)
	s.executing = false
	m.rationalCopy = s.copy
	if err != nil {
		return err
	}
	zero, e := m.config.Rationals.IsZero(&m.localQ)
	if e != nil {
		return e
	}
	if zero {
		if dl != nil {
			if e = algebraicQueryTagAssertionSwitch(); e != nil {
				return e
			}
			if e = m.destroyCell(dst, dl, false); e != nil {
				return e
			}
		}
		return m.assertCellResetZero(dst)
	}
	if dl != nil {
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
		r, e := m.load(dl)
		if e != nil {
			return e
		}
		e = m.config.Rationals.CopyWithAllocationReceipt(&r.value, &m.localQ, &m.rationalCopy)
		return m.save(dl, &r, e)
	}
	next, e := m.allocateCell(0)
	if next.slot != 0 && m.locations[next.slot-1].live {
		m.pending = next
	}
	if e != nil {
		return e
	}
	l := &m.locations[next.slot-1]
	r, e := m.load(l)
	if e != nil {
		return e
	}
	e = m.config.Rationals.Swap(&r.value, &m.localQ)
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	m.publish(dst, next)
	return nil
}

func (m *AlgebraicRootCore) cleanupForeignCopy() error {
	s := &m.foreignCopy
	if !s.active {
		return nil
	}
	if s.running || s.cleaning || !s.NumericBound(false) {
		return ErrAlgebraicPending
	}
	s.cleaning = true
	if e := s.bind(); e != nil {
		s.cleaning = false
		return e
	}
	bound := true
	defer func() {
		if bound {
			s.unbind()
			s.cleaning = false
		}
	}()
	z := m.config.Rationals.Integers()
	if e := z.CleanupCopyAllocationInvocation(&m.coefficientCopy); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.boundCopy); e != nil {
		return e
	}
	if e := s.roots.Cleanup(z); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&m.rationalCopy); e != nil {
		return e
	}
	if e := m.cleanupConstQ(); e != nil {
		return e
	}
	if e := m.cleanupCellAllocation(); e != nil {
		return e
	}
	// Unbind before clearing the concrete scope that owns those bindings.
	s.unbind()
	bound = false
	s.cleaning = false
	s.release()
	return nil
}
