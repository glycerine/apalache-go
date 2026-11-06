package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

type quantSimplification struct {
	context   *Context
	variables *ast.ASTUsedVars
	pins      []*Expr
}

func (s *quantSimplification) keep(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	term, err := NewExpr(s.context, h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.pins = append(s.pins, term)
	return h, nil
}
func (s *quantSimplification) op(h ast.ASTHandle) (int32, error) {
	m := s.context.manager
	kind, err := m.GetKind(h)
	if err != nil {
		return -1, err
	}
	if kind != ast.ASTApp {
		return -1, nil
	}
	return m.GetDeclKind(h)
}
func (s *quantSimplification) simple(args []ast.ASTHandle, and bool) (ast.ASTHandle, error) {
	m := s.context.manager
	var reduced []ast.ASTHandle
	for _, h := range args {
		if (and && m.IsFalse(h)) || (!and && m.IsTrue(h)) {
			return h, nil
		}
		if (and && m.IsTrue(h)) || (!and && m.IsFalse(h)) {
			continue
		}
		reduced = append(reduced, h)
	}
	switch len(reduced) {
	case 0:
		if and {
			return s.keep(m.MkTrue())
		}
		return s.keep(m.MkFalse())
	case 1:
		return reduced[0], nil
	default:
		if and {
			return s.keep(m.MkAndArgs(reduced))
		}
		return s.keep(m.MkOrArgs(reduced))
	}
}
func (s *quantSimplification) negate(h ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.context.manager
	neg, child, err := m.IsNot(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if neg {
		return child, nil
	}
	if m.IsTrue(h) {
		return s.keep(m.MkFalse())
	}
	if m.IsFalse(h) {
		return s.keep(m.MkTrue())
	}
	return s.keep(m.MkNot(h))
}
func (s *quantSimplification) collect(h ast.ASTHandle, op int32, negative bool, out *[]ast.ASTHandle) error {
	m := s.context.manager
	kind, err := s.op(h)
	if err != nil {
		return err
	}
	if kind == ast.ASTOpNot {
		child, err := m.GetArg(h, 0)
		if err != nil {
			return err
		}
		other := ast.ASTOpAnd
		if op == ast.ASTOpAnd {
			other = ast.ASTOpOr
		}
		return s.collect(child, other, !negative, out)
	}
	if kind == op {
		count, err := m.GetNumArgs(h)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			child, err := m.GetArg(h, i)
			if err != nil {
				return err
			}
			if err = s.collect(child, op, negative, out); err != nil {
				return err
			}
		}
		return nil
	}
	if negative {
		h, err = s.negate(h)
		if err != nil {
			return err
		}
	}
	*out = append(*out, h)
	return nil
}
func (s *quantSimplification) lower(h ast.ASTHandle) (ast.ASTHandle, error) {
	term, err := NewExpr(s.context, h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.pins = append(s.pins, term)
	lowered, err := DeleteBound(0, 1, term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.pins = append(s.pins, lowered)
	return lowered.Raw()
}
func (s *quantSimplification) push(q, body ast.ASTHandle, forall bool) (ast.ASTHandle, error) {
	m := s.context.manager
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if count != 1 {
		return s.keep(m.UpdateQuantifierKind(q, forall, body))
	}
	op := ast.ASTOpAnd
	if forall {
		op = ast.ASTOpOr
	}
	var juncts, dependent, independent []ast.ASTHandle
	if err = s.collect(body, op, false, &juncts); err != nil {
		return ast.ASTHandle{}, err
	}
	for _, h := range juncts {
		if err = s.variables.Collect(h); err != nil {
			return ast.ASTHandle{}, err
		}
		if !s.variables.Contains(0).IsNull() {
			dependent = append(dependent, h)
		} else {
			independent = append(independent, h)
		}
	}
	if len(dependent) == 0 {
		combined, err := s.simple(independent, op == ast.ASTOpAnd)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return s.lower(combined)
	}
	combined, err := s.simple(dependent, op == ast.ASTOpAnd)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := s.keep(m.UpdateQuantifierKind(q, forall, combined))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(independent) > 0 {
		free, err := s.simple(independent, op == ast.ASTOpAnd)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		free, err = s.lower(free)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		// Source constructs this pair directly without a final simplification.
		if forall {
			return s.keep(m.MkOr(result, free))
		}
		return s.keep(m.MkAnd(result, free))
	}
	return result, nil
}
func (s *quantSimplification) clone(q, body ast.ASTHandle, forall bool) (ast.ASTHandle, error) {
	m := s.context.manager
	op, err := s.op(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	distribute, partial := ast.ASTOpOr, ast.ASTOpAnd
	if forall {
		distribute, partial = ast.ASTOpAnd, ast.ASTOpOr
	}
	if op == distribute {
		count, err := m.GetNumArgs(body)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args := make([]ast.ASTHandle, count)
		for i := uint32(0); i < count; i++ {
			child, err := m.GetArg(body, i)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i], err = s.clone(q, child, forall)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return s.simple(args, op == ast.ASTOpAnd)
	}
	if op == partial {
		return s.push(q, body, forall)
	}
	if op == ast.ASTOpNot {
		child, err := m.GetArg(body, 0)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		changed, err := s.clone(q, child, !forall)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return s.keep(m.MkNot(changed))
	}
	count, err := m.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if count == 1 {
		if err = s.variables.Collect(body); err != nil {
			return ast.ASTHandle{}, err
		}
		if s.variables.Contains(0).IsNull() {
			return s.lower(body)
		}
	}
	return s.keep(m.UpdateQuantifierKind(q, forall, body))
}

// CloneQuantAndSimp distributes quantifiers through juncts, pushes independent
// terms outside single binders and removes unused single binders with index
// adjustment. Negation dualizes the quantifier as in the source.
// Source: duality_rpfp.cpp:547-591,2407-2420.
func (c *Context) CloneQuantAndSimp(quantifier, body *Expr) (out *Expr, result error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	for _, term := range []*Expr{quantifier, body} {
		if err := term.open(); err != nil {
			return nil, err
		}
		if term.value.object.context != c {
			return nil, ErrContextMismatch
		}
	}
	q, err := quantifier.Raw()
	if err != nil {
		return nil, err
	}
	h, err := body.Raw()
	if err != nil {
		return nil, err
	}
	forall, err := c.manager.IsForall(q)
	if err != nil {
		return nil, err
	}
	variables, err := ast.NewASTUsedVars(c.manager)
	if err != nil {
		return nil, err
	}
	state := &quantSimplification{context: c, variables: variables}
	defer func() {
		for _, pin := range state.pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	changed, err := state.clone(q, h, forall)
	if err != nil {
		return nil, err
	}
	return NewExpr(c, changed)
}
