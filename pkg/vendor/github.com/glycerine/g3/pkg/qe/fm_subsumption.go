package qe

// Source: src/qe/qe_lite.cpp fm::fm registration and backward subsumption.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *fmStore) register(c *fmConstraint) (indexed bool, err error) {
	scope, err := s.begin()
	if err != nil {
		return false, err
	}
	defer s.finish(scope, &err)
	if c == nil || c.owner != s || c.closed {
		return false, ast.ErrASTDomain
	}
	return s.registerCore(scope, c)
}
func (s *fmStore) registerCore(scope *fmScope, c *fmConstraint) (bool, error) {
	if err := s.normalizeCore(scope, c); err != nil {
		return false, err
	}
	no, err := s.falseConstraint(c)
	if err != nil {
		return false, err
	}
	if no {
		// Source formats c after del_constraint(c). Format the live native
		// value first, preserving the diagnostic without a dangling pointer.
		if err = s.traceConstraint("register_constraint", 1801, "is false ", c); err != nil {
			return false, err
		}
		if err = s.deleteConstraint(c); err != nil {
			return false, err
		}
		s.inconsistent = true
		return false, nil
	}
	indexed := false
	n := s.recognizer.manager.NumeralService()
	for i, index := range c.variables {
		if uint64(index) >= uint64(len(s.variables)) {
			return false, ast.ErrASTDomain
		}
		v := &s.variables[index]
		if v.forbidden {
			continue
		}
		indexed = true
		negative, err := n.IsNeg(&c.coefficients[i])
		if err != nil {
			return false, err
		}
		if negative {
			v.lowers = append(v.lowers, c)
		} else {
			v.uppers = append(v.uppers, c)
		}
	}
	if indexed {
		s.insertTodo(c)
		return true, nil
	}
	if err = s.traceConstraint("register_constraint", 1824, "all variables are forbidden ", c); err != nil {
		return false, err
	}
	h, err := s.expressionCore(scope, c)
	if err != nil {
		return false, err
	}
	ref, err := s.recognizer.manager.NewRef(h)
	if err != nil {
		return false, err
	}
	s.outputs = append(s.outputs, ref)
	return false, s.deleteConstraint(c)
}

func (s *fmStore) subsumes(c1, c2 *fmConstraint) (bool, error) {
	if err := s.check(); err != nil {
		return false, err
	}
	if c1 == nil || c2 == nil || c1.owner != s || c2.owner != s || c1.closed || c2.closed {
		return false, ast.ErrASTDomain
	}
	if c1 == c2 || len(c1.literals) > len(c2.literals) || len(c1.variables) != len(c2.variables) {
		return false, nil
	}
	n := s.recognizer.manager.NumeralService()
	greater, err := n.Greater(&c1.constant, &c2.constant)
	if err != nil || greater {
		return false, err
	}
	if !c1.strict && c2.strict {
		equal, err := n.Equal(&c1.constant, &c2.constant)
		if err != nil || equal {
			return false, err
		}
	}
	s.counter += uint32(len(c1.literals) + len(c2.literals))
	positions := make(map[uint32]int)
	for i, index := range c1.variables {
		positions[index] = i
	}
	for i, index := range c2.variables {
		pos, ok := positions[index]
		if !ok {
			return false, nil
		}
		equal, err := n.Equal(&c1.coefficients[pos], &c2.coefficients[i])
		if err != nil || !equal {
			return false, err
		}
	}
	// The source scratch sign table is keyed by Boolean variable; retain its
	// sign assignment rather than treating opposite literals as two keys.
	signs := make(map[int32]bool)
	for _, lit := range c2.literals {
		key := lit
		if key < 0 {
			key = -key
		}
		signs[key] = lit < 0
	}
	for _, lit := range c1.literals {
		key := lit
		if key < 0 {
			key = -key
		}
		sign, ok := signs[key]
		if !ok || sign != (lit < 0) {
			return false, nil
		}
	}
	return true, nil
}

func (s *fmStore) backwardSubsumption(c *fmConstraint) error {
	if len(c.variables) == 0 {
		return nil
	}
	best, bestSize := -1, int(^uint(0)>>1)
	lower := false
	n := s.recognizer.manager.NumeralService()
	for i, index := range c.variables {
		v := &s.variables[index]
		if v.forbidden {
			continue
		}
		negative, err := n.IsNeg(&c.coefficients[i])
		if err != nil {
			return err
		}
		cs := v.uppers
		if negative {
			cs = v.lowers
		}
		if len(cs) < bestSize {
			best, bestSize, lower = int(index), len(cs), negative
		}
	}
	if best < 0 || bestSize == 0 {
		return nil
	}
	v := &s.variables[best]
	cs := v.uppers
	if lower {
		cs = v.lowers
	}
	s.counter += uint32(len(cs))
	j := 0
	for _, c2 := range cs {
		if c2.dead {
			continue
		}
		yes, err := s.subsumes(c, c2)
		if err != nil {
			return err
		}
		if yes {
			if err = s.recognizer.emit("backward_subsumption", 1447, func(out io.Writer) error {
				if err := s.displayConstraint(out, c); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\nsubsumed:\n")
				if err := s.displayConstraint(out, c2); err != nil {
					return err
				}
				_, _ = io.WriteString(out, "\n")
				return nil
			}); err != nil {
				return err
			}
			c2.dead = true
			continue
		}
		cs[j] = c2
		j++
	}
	if lower {
		v.lowers = cs[:j]
	} else {
		v.uppers = cs[:j]
	}
	return nil
}

func (s *fmStore) subsume() (err error) {
	scope, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(scope, &err)
	for len(s.todo) != 0 {
		c := s.popTodo()
		if c.dead {
			continue
		}
		if err := s.backwardSubsumption(c); err != nil {
			return err
		}
	}
	return nil
}
