package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

// MaxIndex follows the source body-only free-variable test. Quantifier
// annotations are not visited, and each binder subtracts its arity even from
// the no-variable sentinel -1.
// Source: duality_rpfp.cpp:3519-3548.
func MaxIndex(term *Expr) (int, error) {
	h, err := term.Raw()
	if err != nil {
		return -1, err
	}
	m := term.value.object.context.manager
	memo := make(map[ast.ASTHandle]int)
	var visit func(ast.ASTHandle) (int, error)
	visit = func(h ast.ASTHandle) (int, error) {
		if value, found := memo[h]; found {
			return value, nil
		}
		kind, err := m.GetKind(h)
		if err != nil {
			return -1, err
		}
		result := -1
		switch kind {
		case ast.ASTApp:
			n, err := m.GetNumArgs(h)
			if err != nil {
				return -1, err
			}
			for i := uint32(0); i < n; i++ {
				child, err := m.GetArg(h, i)
				if err != nil {
					return -1, err
				}
				value, err := visit(child)
				if err != nil {
					return -1, err
				}
				if value > result {
					result = value
				}
			}
		case ast.ASTQuantifier:
			body, err := m.GetQuantifierBody(h)
			if err != nil {
				return -1, err
			}
			value, err := visit(body)
			if err != nil {
				return -1, err
			}
			bound, err := m.GetNumDeclarations(h)
			if err != nil {
				return -1, err
			}
			result = value - int(bound)
		case ast.ASTVar:
			index, err := m.GetVarIndex(h)
			if err != nil {
				return -1, err
			}
			result = int(index)
		}
		memo[h] = result
		return result, nil
	}
	return visit(h)
}

func IsClosedFormula(term *Expr) (bool, error) { index, err := MaxIndex(term); return index < 0, err }

// IsLiteral returns an owning atom and its Boolean polarity. A nonliteral
// returns nil,false,nil. Closed quantified formulas are source literals.
// Source: duality_rpfp.cpp:433-451.
func IsLiteral(term *Expr) (*Expr, bool, error) {
	if err := term.open(); err != nil {
		return nil, false, err
	}
	c := term.value.object.context
	m := c.manager
	h, err := term.Raw()
	if err != nil {
		return nil, false, err
	}
	polarity := true
	for {
		kind, err := m.GetKind(h)
		if err != nil {
			return nil, false, err
		}
		if kind == ast.ASTQuantifier {
			quantified, err := NewExpr(c, h)
			if err != nil {
				return nil, false, err
			}
			closed, err := IsClosedFormula(quantified)
			if err != nil {
				return nil, false, errors.Join(err, quantified.Close())
			}
			if closed {
				return quantified, polarity, nil
			}
			if err = quantified.Close(); err != nil {
				return nil, false, err
			}
			return nil, false, nil
		}
		if kind != ast.ASTApp {
			return nil, false, nil
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return nil, false, err
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return nil, false, err
		}
		if family == m.BasicFamilyID() {
			switch op {
			case ast.ASTOpNot:
				h, err = m.GetArg(h, 0)
				if err != nil {
					return nil, false, err
				}
				polarity = !polarity
				continue
			case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpIff, ast.ASTOpImplies:
				return nil, false, nil
			}
		}
		atom, err := NewExpr(c, h)
		return atom, polarity, err
	}
}
