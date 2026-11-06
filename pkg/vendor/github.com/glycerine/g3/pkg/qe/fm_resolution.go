package qe

// Source: src/qe/qe_lite.cpp fm::fm::get_coeff / resolve.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fmCoefficient(c *fmConstraint, x uint32) (*numeral.OwnedRational, error) {
	for i, index := range c.variables {
		if index == x {
			return &c.coefficients[i], nil
		}
	}
	return nil, ast.ErrASTDomain
}

func (s *fmStore) resolve(lower, upper *fmConstraint, x uint32) (result *fmConstraint, err error) {
	scope, err := s.begin()
	if err != nil {
		return nil, err
	}
	defer s.finish(scope, &err)
	return s.resolveCore(scope, lower, upper, x)
}
func (s *fmStore) resolveCore(scope *fmScope, lower, upper *fmConstraint, x uint32) (*fmConstraint, error) {
	if lower == nil || upper == nil || lower.owner != s || upper.owner != s || lower.closed || upper.closed || uint64(x) >= uint64(len(s.variables)) {
		return nil, ast.ErrASTDomain
	}
	s.counter += uint32(len(lower.variables) + len(upper.variables) + len(lower.literals) + len(upper.literals))
	n := s.recognizer.manager.NumeralService()
	la, err := fmCoefficient(lower, x)
	if err != nil {
		return nil, err
	}
	b, err := fmCoefficient(upper, x)
	if err != nil {
		return nil, err
	}
	negative, err := n.IsNeg(la)
	if err != nil {
		return nil, err
	}
	positive, err := n.IsPos(b)
	if err != nil {
		return nil, err
	}
	if !negative || !positive {
		return nil, ast.ErrASTDomain
	}
	a, err := scope.keep(n.Neg(la))
	if err != nil {
		return nil, err
	}
	if s.variables[x].integer {
		one, err := n.IsOne(a)
		if err != nil {
			return nil, err
		}
		otherOne, err := n.IsOne(b)
		if err != nil {
			return nil, err
		}
		if !one && !otherOne {
			return nil, ast.ErrASTDomain
		}
	}
	constant, err := scope.keep(n.Mul(&lower.constant, b))
	if err != nil {
		return nil, err
	}
	other, err := scope.keep(n.Mul(&upper.constant, a))
	if err != nil {
		return nil, err
	}
	if err = n.AddAssign(constant, other); err != nil {
		return nil, err
	}
	strict := lower.strict || upper.strict
	var xs []uint32
	var as []*numeral.OwnedRational
	positions := make(map[uint32]int)
	for i, index := range lower.variables {
		if index == x {
			continue
		}
		positions[index] = len(xs)
		xs = append(xs, index)
		v, err := scope.keep(n.Mul(&lower.coefficients[i], b))
		if err != nil {
			return nil, err
		}
		as = append(as, v)
	}
	for i, index := range upper.variables {
		if index == x {
			continue
		}
		v, err := scope.keep(n.Mul(&upper.coefficients[i], a))
		if err != nil {
			return nil, err
		}
		if pos, ok := positions[index]; ok {
			if err = n.AddAssign(as[pos], v); err != nil {
				return nil, err
			}
		} else {
			// Source records positions for lower variables only. Upper-only
			// duplicates are excluded by the input normal-form recognizer.
			xs = append(xs, index)
			as = append(as, v)
		}
	}
	allInt, j := true, 0
	for i, index := range xs {
		zero, err := n.IsZero(as[i])
		if err != nil {
			return nil, err
		}
		if zero {
			continue
		}
		if !s.variables[index].integer {
			allInt = false
		}
		xs[j], as[j] = index, as[i]
		j++
	}
	xs, as = xs[:j], as[:j]
	if allInt && strict {
		strict = false
		if err = n.AddAssignInt(constant, -1); err != nil {
			return nil, err
		}
	}
	positive, err = n.IsPos(constant)
	if err != nil {
		return nil, err
	}
	zero, err := n.IsZero(constant)
	if err != nil {
		return nil, err
	}
	if len(xs) == 0 && (positive || (!strict && zero)) {
		return nil, s.traceResolution(2064, x, " consequent literal is always true: \n", lower, upper, nil)
	}
	literals := append([]int32(nil), lower.literals...)
	signs := make(map[int32]bool)
	for _, lit := range lower.literals {
		key := lit
		if key < 0 {
			key = -key
		}
		signs[key] = lit < 0
	}
	tautology := false
	for _, lit := range upper.literals {
		key := lit
		if key < 0 {
			key = -key
		}
		if sign, ok := signs[key]; ok {
			if sign != (lit < 0) {
				tautology = true
				break
			}
		} else {
			literals = append(literals, lit)
		}
	}
	if tautology {
		return nil, s.traceResolution(2108, x, " tautology: \n", lower, upper, nil)
	}
	negative, err = n.IsNeg(constant)
	if err != nil {
		return nil, err
	}
	if len(literals) == 0 && len(xs) == 0 && (negative || (strict && zero)) {
		if err = s.traceResolution(2118, x, " inconsistent: \n", lower, upper, nil); err != nil {
			return nil, err
		}
		s.inconsistent = true
		return nil, nil
	}
	c, err := s.newConstraint(literals, xs, as, constant, strict)
	if err != nil {
		return nil, err
	}
	if err = s.traceResolution(2136, x, "\n", lower, upper, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *fmStore) traceResolution(line int, x uint32, message string, lower, upper, result *fmConstraint) error {
	return s.recognizer.emit("resolve", line, func(out io.Writer) error {
		_, _ = io.WriteString(out, "resolution "+strconv.FormatUint(uint64(x), 10)+message)
		if err := s.displayConstraint(out, lower); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		if err := s.displayConstraint(out, upper); err != nil {
			return err
		}
		if result != nil {
			_, _ = io.WriteString(out, "\n---->\n")
			if err := s.displayConstraint(out, result); err != nil {
				return err
			}
			_, _ = io.WriteString(out, "\nnew_dep: 0\n")
		} else {
			_, _ = io.WriteString(out, "\n")
		}
		return nil
	})
}
