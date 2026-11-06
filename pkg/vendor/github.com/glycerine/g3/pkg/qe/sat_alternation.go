package qe

// Source: src/qe/qe_sat_tactic.cpp, skolemize_existential_prefix and
// the quantifier-extraction portion of extract_alt_form.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type satAlternation struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables [][]ast.ASTRef
	closed    bool
}

func extractSATAlternation(m *ast.ASTManager, config Config, formula ast.ASTHandle) (_ *satAlternation, resultErr error) {
	if m == nil || config.Context == nil || config.Globals == nil {
		return nil, ast.ErrASTHandle
	}
	hoist, err := rewriter.NewQuantifierHoister(m, config.Context, config.Globals, nil)
	if err != nil {
		if hoist != nil {
			err = errors.Join(err, hoist.Close())
		}
		return nil, err
	}
	result := &satAlternation{manager: m}
	defer func() {
		resultErr = errors.Join(resultErr, hoist.Close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	prefix, err := hoist.PullExists(formula, true, true)
	if err != nil {
		return nil, err
	}
	result.formula, err = m.CopyRef(prefix.Formula)
	err = errors.Join(err, prefix.Close())
	if err != nil {
		return nil, err
	}
	for {
		current, e := m.RefGet(result.formula)
		if e != nil {
			return nil, e
		}
		block, e := hoist.Hoist(current, true, true)
		if e != nil {
			return nil, e
		}
		if len(block.Variables) == 0 {
			if e = block.Close(); e != nil {
				return nil, e
			}
			break
		}
		if block.IsForall != (len(result.variables)%2 == 0) {
			return nil, errors.Join(ast.ErrASTDomain, block.Close())
		}
		var refs []ast.ASTRef
		for _, v := range block.Variables {
			ref, cause := m.CopyRef(v)
			if cause != nil {
				for i := range refs {
					cause = errors.Join(cause, m.DelRef(&refs[i]))
				}
				return nil, errors.Join(cause, block.Close())
			}
			refs = append(refs, ref)
		}
		result.variables = append(result.variables, refs)
		e = m.AssignRef(result.formula, block.Formula)
		e = errors.Join(e, block.Close())
		if e != nil {
			return nil, e
		}
	}
	if len(result.variables)%2 == 1 {
		current, e := m.RefGet(result.formula)
		if e != nil {
			return nil, e
		}
		negated, e := m.MkNot(current)
		if e != nil {
			return nil, e
		}
		if e = m.SetRef(result.formula, negated); e != nil {
			return nil, e
		}
	}
	return result, nil
}

func (s *satAlternation) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.formula != (ast.ASTRef{}) {
		if err := s.manager.DelRef(&s.formula); err != nil {
			return err
		}
	}
	for len(s.variables) > 0 {
		i := len(s.variables) - 1
		for len(s.variables[i]) > 0 {
			j := len(s.variables[i]) - 1
			ref := &s.variables[i][j]
			err := s.manager.DelRef(ref)
			if *ref == (ast.ASTRef{}) {
				s.variables[i] = s.variables[i][:j]
			}
			if err != nil {
				return err
			}
		}
		s.variables = s.variables[:i]
	}
	s.closed = true
	return nil
}
