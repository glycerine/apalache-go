package qe

// Source: src/qe/qe_lite.cpp fm::fm::to_expr.

import "github.com/glycerine/g3/pkg/ast"

// expression returns a caller-owned reference, including a nonzero output if
// constructing it succeeded but lexical cleanup subsequently stopped.
func (s *fmStore) expression(c *fmConstraint) (out ast.ASTRef, err error) {
	scope, err := s.begin()
	if err != nil {
		return out, err
	}
	defer s.finish(scope, &err)
	if c == nil || c.owner != s || c.closed {
		return out, ast.ErrASTDomain
	}
	h, err := s.expressionCore(scope, c)
	if err != nil {
		return out, err
	}
	return s.recognizer.manager.NewRef(h)
}

func (s *fmStore) expressionCore(scope *fmScope, c *fmConstraint) (ast.ASTHandle, error) {
	r := s.recognizer
	m := r.manager
	n := m.NumeralService()
	var inequality ast.ASTHandle
	if len(c.variables) == 0 {
		positive, err := n.IsPos(&c.constant)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		zero, err := n.IsZero(&c.constant)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if positive || (!c.strict && zero) {
			return scope.hold(m.MkTrue())
		}
	} else {
		integer := s.allInteger(c)
		var monomials []ast.ASTHandle
		for i, index := range c.variables {
			if uint64(index) >= uint64(len(s.variables)) {
				return ast.ASTHandle{}, ast.ErrASTDomain
			}
			x, err := m.RefGet(s.variables[index].expression)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !integer && s.variables[index].integer {
				x, err = scope.hold(m.MkFamilyApp(r.arithmetic, 14, nil, []ast.ASTHandle{x}))
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			one, err := n.IsOne(&c.coefficients[i])
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !one {
				a, err := scope.hold(m.ArithmeticNumeral(&c.coefficients[i], integer))
				if err != nil {
					return ast.ASTHandle{}, err
				}
				x, err = scope.hold(m.MkArithmeticMul(a, x))
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			monomials = append(monomials, x)
		}
		lhs := monomials[0]
		if len(monomials) > 1 {
			var err error
			lhs, err = scope.hold(m.MkFamilyApp(r.arithmetic, 6, nil, monomials))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		rhs, err := scope.hold(m.ArithmeticNumeral(&c.constant, integer))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if c.strict {
			inequality, err = scope.hold(m.MkArithmeticGE(lhs, rhs))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			inequality, err = scope.hold(m.MkNot(inequality))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		} else {
			inequality, err = scope.hold(m.MkArithmeticLE(lhs, rhs))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
	}
	if len(c.literals) == 0 {
		if inequality.IsNull() {
			return scope.hold(m.MkFalse())
		}
		return inequality, nil
	}
	var literals []ast.ASTHandle
	for _, lit := range c.literals {
		index := int64(lit)
		if index < 0 {
			index = -index
		}
		if index <= 0 || index >= int64(len(s.booleans)) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		h, err := m.RefGet(s.booleans[index])
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if lit < 0 {
			h, err = scope.hold(m.MkNot(h))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		literals = append(literals, h)
	}
	if !inequality.IsNull() {
		literals = append(literals, inequality)
	}
	if len(literals) == 1 {
		return literals[0], nil
	}
	return scope.hold(m.MkOrArray(literals))
}
