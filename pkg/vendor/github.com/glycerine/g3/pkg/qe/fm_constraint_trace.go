package qe

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *fmStore) displayConstraint(out io.Writer, c *fmConstraint) error {
	r := s.recognizer
	m := r.manager
	n := m.NumeralService()
	for _, lit := range c.literals {
		index := int64(lit)
		if index < 0 {
			index = -index
			_, _ = io.WriteString(out, "~")
		}
		if index <= 0 || index >= int64(len(s.booleans)) {
			return ast.ErrASTDomain
		}
		h, err := m.RefGet(s.booleans[index])
		if err != nil {
			return err
		}
		if err = writeExpression(m, r.config, out, h); err != nil {
			return err
		}
		_, _ = io.WriteString(out, " ")
	}
	_, _ = io.WriteString(out, "(")
	if len(c.variables) == 0 {
		_, _ = io.WriteString(out, "0")
	}
	for i, index := range c.variables {
		if i > 0 {
			_, _ = io.WriteString(out, " + ")
		}
		one, err := n.IsOne(&c.coefficients[i])
		if err != nil {
			return err
		}
		if !one {
			text, err := n.String(&c.coefficients[i])
			if err != nil {
				return err
			}
			_, _ = io.WriteString(out, text+"*")
		}
		h, err := m.RefGet(s.variables[index].expression)
		if err != nil {
			return err
		}
		if err = writeExpression(m, r.config, out, h); err != nil {
			return err
		}
	}
	comparison := " <= "
	if c.strict {
		comparison = " < "
	}
	_, _ = io.WriteString(out, comparison)
	text, err := n.String(&c.constant)
	if err != nil {
		return err
	}
	_, _ = io.WriteString(out, text+")")
	return nil
}

func (s *fmStore) traceConstraint(function string, line int, prefix string, c *fmConstraint) error {
	return s.recognizer.emit(function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, prefix)
		if err := s.displayConstraint(out, c); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}

func (s *fmStore) traceVariables(xs []uint32) error {
	return s.recognizer.emit("add_constraint", 1777, func(out io.Writer) error {
		_, _ = io.WriteString(out, "before mk_constraint: ")
		for _, index := range xs {
			_, _ = io.WriteString(out, " "+strconv.FormatUint(uint64(index), 10))
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}
