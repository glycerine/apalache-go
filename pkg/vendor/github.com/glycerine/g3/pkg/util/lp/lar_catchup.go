package lp

// Source: src/util/lp/lar_core_solver.h:407-482.

import "github.com/glycerine/g3/pkg/util/numeral"

// Only column selection is shared by the concrete pair and floating consumers.
// The two FIFO queues delay useful endpoints until their partner is reached.
type larCatchUpQueues struct {
	enter, leave []uint32
	ei, li       int
}

func (q *larCatchUpQueues) selectColumns(enter, leave uint32, target, heading []int32) (uint32, uint32, bool, error) {
	if uint64(enter) >= uint64(len(target)) {
		return 0, 0, false, ErrBounds
	}
	goodE := false
	if target[enter] >= 0 {
		if uint64(enter) >= uint64(len(heading)) {
			return 0, 0, false, ErrBounds
		}
		goodE = heading[enter] < 0
	}
	if uint64(leave) >= uint64(len(target)) {
		return 0, 0, false, ErrBounds
	}
	goodL := false
	if target[leave] < 0 {
		if uint64(leave) >= uint64(len(heading)) {
			return 0, 0, false, ErrBounds
		}
		goodL = heading[leave] >= 0
	}
	if !goodE && !goodL {
		return enter, leave, false, nil
	}
	if goodE && !goodL {
		for q.li < len(q.leave) {
			j := q.leave[q.li]
			if uint64(j) >= uint64(len(heading)) {
				return 0, 0, false, ErrBounds
			}
			if heading[j] >= 0 {
				break
			}
			q.li++
		}
		if q.li == len(q.leave) {
			q.enter = append(q.enter, enter)
			return enter, leave, false, nil
		}
		leave = q.leave[q.li]
		q.li++
	} else if !goodE && goodL {
		for q.ei < len(q.enter) {
			j := q.enter[q.ei]
			if uint64(j) >= uint64(len(heading)) {
				return 0, 0, false, ErrBounds
			}
			if heading[j] < 0 {
				break
			}
			q.ei++
		}
		if q.ei == len(q.enter) {
			q.leave = append(q.leave, leave)
			return enter, leave, false, nil
		}
		enter = q.enter[q.ei]
		q.ei++
	}
	return enter, leave, true, nil
}

func (c *LARCoreSolver) CatchUpInPairLU(trace []uint32, target []int32) (err error) {
	if c.stopped || c.catchUpWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	s := c.RPrimal
	if s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	l := s.Factorization
	if l == nil || l.RefactorCounter+uint32(len(trace)/2) >= 200 {
		for i := 0; i < len(trace); i += 2 {
			if i+1 >= len(trace) {
				return ErrBounds
			}
			if err = s.ChangeBasisUnconditionally(trace[i], trace[i+1]); err != nil {
				return err
			}
		}
		if l != nil {
			if err = l.Close(); err != nil {
				return err
			}
		}
		s.Factorization = nil
	} else {
		c.catchUpWork = NewIndexedRationalVector(c.manager, s.A.RowCount())
		if sourceDebugEnabled && l.GetStatus() != LUOK {
			return ErrAssertion
		}
		var queues larCatchUpQueues
		zero := numeral.NewRational()
		for i := 0; i < len(trace); i += 2 {
			if i+1 >= len(trace) {
				return ErrBounds
			}
			enter, leave, use, e := queues.selectColumns(trace[i], trace[i+1], target, *s.Heading)
			if e != nil {
				return e
			}
			if !use {
				continue
			}
			if sourceDebugEnabled && ((*s.Heading)[enter] >= 0 || (*s.Heading)[leave] < 0) {
				return ErrAssertion
			}
			if l.GetStatus() == LUOK {
				if err = l.PrepareEntering(enter, c.catchUpWork); err != nil {
					return err
				}
				if err = l.ReplaceColumn(&zero, c.catchUpWork, uint32((*s.Heading)[leave])); err != nil {
					return err
				}
			}
			if err = s.ChangeBasisUnconditionally(enter, leave); err != nil {
				return err
			}
		}
		if l.GetStatus() != LUOK {
			if err = l.Close(); err != nil {
				return err
			}
			s.Factorization = nil
		}
		if err = c.catchUpWork.CloseForward(); err != nil {
			return err
		}
		c.catchUpWork = nil
	}
	if s.Factorization == nil {
		return InitPairFactorization(&s.Factorization, s.A, *s.Basis, c.settings)
	}
	return nil
}

func (c *LARCoreSolver) CatchUpInFloatLU(trace []uint32, target []int32) (err error) {
	if c.stopped || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	s := c.DPrimal
	if s == nil {
		return ErrBounds
	}
	l := s.Factorization
	if l == nil || l.RefactorCounter+uint32(len(trace)/2) >= 200 {
		for i := 0; i < len(trace); i += 2 {
			if i+1 >= len(trace) {
				return ErrBounds
			}
			if err = s.ChangeBasisUnconditionally(trace[i], trace[i+1]); err != nil {
				return err
			}
		}
		if l != nil {
			l.Close()
		}
		s.Factorization = nil
	} else {
		w := NewIndexedFloatVector(s.A.RowCount())
		if sourceDebugEnabled && l.GetStatus() != LUOK {
			return ErrAssertion
		}
		var queues larCatchUpQueues
		for i := 0; i < len(trace); i += 2 {
			if i+1 >= len(trace) {
				return ErrBounds
			}
			enter, leave, use, e := queues.selectColumns(trace[i], trace[i+1], target, *s.Heading)
			if e != nil {
				return e
			}
			if !use {
				continue
			}
			if sourceDebugEnabled && ((*s.Heading)[enter] >= 0 || (*s.Heading)[leave] < 0) {
				return ErrAssertion
			}
			if l.GetStatus() == LUOK {
				if err = l.PrepareEntering(enter, w); err != nil {
					return err
				}
				if err = l.ReplaceColumn(0, w, uint32((*s.Heading)[leave])); err != nil {
					return err
				}
			}
			if err = s.ChangeBasisUnconditionally(enter, leave); err != nil {
				return err
			}
		}
		if l.GetStatus() != LUOK {
			l.Close()
			s.Factorization = nil
		}
	}
	// Unlike the precise consumer, source leaves a discarded floating LU nil.
	return nil
}

// The retained reverse consumer is exact only (solve_on_signature).
func (c *LARCoreSolver) CatchUpInPairLUInReverse(trace []uint32) (err error) {
	if c.stopped || c.catchUpWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	s := c.RPrimal
	if s == nil {
		return ErrBounds
	}
	if err = s.ready(); err != nil {
		return err
	}
	for i := len(trace); i > 0; i -= 2 {
		if i < 2 {
			return ErrBounds
		}
		if err = s.ChangeBasisUnconditionally(trace[i-1], trace[i-2]); err != nil {
			return err
		}
	}
	return s.InitLU()
}
