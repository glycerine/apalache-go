package lp

// Source: src/util/lp/breakpoint.h; lp_primal_core_solver.hpp:1069-1072,
// 1245-1248,1306-1333; rational.h. Concrete retained mpq/mpq instantiation.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalBreakpoint borrows Delta when passed to a solver operation. Records in
// RationalPrimalCoreSolver.Breakpoints own independent Delta values until cleared.
type RationalBreakpoint struct {
	Column uint32
	Type   BreakpointType
	Delta  *numeral.Rational
}

func (m *RationalPrimalCoreSolver) destroyBreakpointRecords() error {
	for i, b := range m.Breakpoints {
		if b == nil { // A completed destruction prefix is safe to resume.
			continue
		}
		if err := m.manager.Del(b.Delta); err != nil {
			return err
		}
		m.Breakpoints[i] = nil
	}
	m.Breakpoints = m.Breakpoints[:0]
	return nil
}

func (m *RationalPrimalCoreSolver) ClearBreakpoints() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.destroyBreakpointRecords(); err != nil {
		return err
	}
	return m.BreakpointQueue.Clear()
}

func (m *RationalPrimalCoreSolver) constructBreakpoint(j uint32, delta *numeral.Rational, typ BreakpointType) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.breakpointWork = NewIndexedRationalVector(m.manager, 4)
	arg, ctorArg, temporary, priority := &m.breakpointWork.Data[0], &m.breakpointWork.Data[1], &m.breakpointWork.Data[2], &m.breakpointWork.Data[3]
	if err = m.manager.Copy(arg, delta); err != nil {
		return err
	}
	if err = m.manager.Copy(ctorArg, arg); err != nil {
		return err
	}
	if err = m.manager.Copy(temporary, ctorArg); err != nil {
		return err
	}
	// The constructor's by-value parameter dies on return, before push_back.
	if err = m.manager.Del(ctorArg); err != nil {
		return err
	}
	owned := numeral.NewRational()
	m.pendingBreakpoint = &RationalBreakpoint{Column: j, Type: typ, Delta: &owned}
	if err = m.manager.Copy(&owned, temporary); err != nil {
		return err
	}
	m.Breakpoints = append(m.Breakpoints, m.pendingBreakpoint)
	m.pendingBreakpoint = nil
	if err = m.manager.Del(temporary); err != nil {
		return err
	}
	// Scalar abs returns a copied operand, then mutates that owned value.
	if err = m.manager.Copy(priority, arg); err != nil {
		return err
	}
	if err = m.manager.Abs(priority); err != nil {
		return err
	}
	return nil
}

// AddBreakpoint retains the appended record if priority enqueue fails. Queue
// numerical receipts remain with the queue, outside the primal capture.
func (m *RationalPrimalCoreSolver) AddBreakpoint(j uint32, delta *numeral.Rational, typ BreakpointType) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.addBreakpointValue(j, delta, typ)
}

// Called by this primal's collection expression while its scalar locals live.
func (m *RationalPrimalCoreSolver) addBreakpointValue(j uint32, delta *numeral.Rational, typ BreakpointType) (err error) {
	if delta == nil {
		return ErrBounds
	}
	if err = m.constructBreakpoint(j, delta, typ); err != nil {
		return err
	}
	if err = m.BreakpointQueue.Enqueue(m.BreakpointQueue.Size(), &m.breakpointWork.Data[3]); err != nil {
		return err
	}
	if err = m.breakpointWork.Close(); err != nil {
		return err
	}
	m.breakpointWork = nil
	return nil
}

// ChangeSlopeOnBreakpoint owns its scalar expressions and borrows both the
// breakpoint and caller slope. Source heading and entering assertions remain.
func (m *RationalPrimalCoreSolver) ChangeSlopeOnBreakpoint(entering uint32, b *RationalBreakpoint, slope *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.changeSlopeOnBreakpointValue(entering, b, slope)
}

func (m *RationalPrimalCoreSolver) changeSlopeOnBreakpointValue(entering uint32, b *RationalBreakpoint, slope *numeral.Rational) (err error) {
	if b == nil || b.Delta == nil || slope == nil {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 7)
	d, absolute, direction, delta, two, doubled, temporary := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3], &m.work.Data[4], &m.work.Data[5], &m.work.Data[6]
	if b.Column == entering {
		if sourceDebugEnabled {
			if b.Type == BreakpointFixed {
				return ErrAssertion
			}
			zero, e := m.manager.IsZero(b.Delta)
			if e != nil {
				return e
			}
			if zero {
				return ErrAssertion
			}
		}
		if err = m.manager.SetInt32(direction, m.SignOfEnteringDelta); err != nil {
			return err
		}
		if err = m.manager.Add(slope, direction, slope); err != nil {
			return err
		}
	} else {
		if uint64(b.Column) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		i := (*m.Heading)[b.Column]
		if i < 0 {
			if sourceDebugEnabled {
				return ErrAssertion
			}
			return ErrBounds
		}
		if uint64(i) >= uint64(len(m.Ed.Data)) {
			return ErrBounds
		}
		if err = m.manager.Copy(d, &m.Ed.Data[i]); err != nil {
			return err
		}
		if err = m.manager.Neg(d); err != nil {
			return err
		}
		zero, e := m.manager.IsZero(d)
		if e != nil {
			return e
		}
		if zero {
			return m.releaseSlopeWork()
		}
		if err = m.manager.Copy(absolute, d); err != nil {
			return err
		}
		if err = m.manager.Abs(absolute); err != nil {
			return err
		}
		if err = m.manager.SetInt32(direction, m.SignOfEnteringDelta); err != nil {
			return err
		}
		if err = etaRationalProduct(m.manager, temporary, delta, direction, absolute); err != nil {
			return err
		}
		if err = m.manager.Del(absolute); err != nil {
			return err
		}
		if err = m.manager.Del(direction); err != nil {
			return err
		}
		switch b.Type {
		case BreakpointFixed:
			zero, e = m.manager.IsZero(b.Delta)
			if e != nil {
				return e
			}
			if !zero {
				if err = m.manager.SetInt32(two, 2); err != nil {
					return err
				}
				if err = etaRationalProduct(m.manager, temporary, doubled, two, delta); err != nil {
					return err
				}
				delta = doubled
			}
			if err = m.manager.Add(slope, delta, slope); err != nil {
				return err
			}
		case BreakpointLow, BreakpointUpper:
			if err = m.manager.Add(slope, delta, slope); err != nil {
				return err
			}
		default:
			if sourceDebugEnabled {
				return ErrAssertion
			}
		}
	}
	return m.releaseSlopeWork()
}

func (m *RationalPrimalCoreSolver) releaseSlopeWork() error {
	if err := m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
