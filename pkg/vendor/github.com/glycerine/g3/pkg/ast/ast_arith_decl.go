package ast

// Source: src/ast/arith_decl_plugin.cpp:342-382,392-398,451-509.

import (
	"github.com/glycerine/g3/pkg/util/z3exception"
)

func arithmeticUsesCoercion(k int32) bool {
	return k == 6 || k == 7 || k == 9 || k == 18 || k == 2 || k == 3 || k == 4 || k == 5 || k == 8
}
func (m *ASTManager) arithmeticOperatorDecl(k int32, arity int, real bool) astNodeRef {
	if k == 7 && arity == 1 {
		k = 8
	}
	p := m.arithmeticPlugin
	switch k {
	case 2, 3, 4, 5, 6, 7, 8, 9: // comparisons, add/subtract/negate/multiply
		if real {
			return p.fields[k]
		}
		return p.fields[k+9]
	case 10:
		return p.fields[10] // real division
	case 11:
		return p.fields[19] // integer division
	case 12:
		return p.fields[21] // remainder
	case 13:
		return p.fields[20] // modulus
	case 14, 15, 16:
		return p.fields[k+8] // to_real, to_int, is_int
	case 17: // absolute value
		if real {
			return p.fields[28]
		}
		return p.fields[27]
	case 18: // power
		if real {
			return p.fields[26]
		}
		return p.fields[25]
	case 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30: // trigonometric functions
		return p.fields[k+10]
	case 31, 32: // pi, euler
		return m.appAt(p.fields[k+10].slot).decl
	case 33, 34, 35: // neg-root, asin-u, acos-u
		return p.fields[k+10]
	default:
		return astNodeRef{}
	}
}
func arithmeticCheckArguments(k int32, arity int) error {
	if arity == 0 && k != 31 && k != 32 {
		return ASTException{z3exception.NewDefaultException("no arguments supplied to arithmetical operator")}
	}
	return nil
}
func (m *ASTManager) arithmeticDecl(k int32, ps []ASTParameter, domain []ASTHandle) (astNodeRef, error) {
	if k == 0 {
		return m.arithmeticNumeralDecl(ps, len(domain))
	}
	if e := arithmeticCheckArguments(k, len(domain)); e != nil {
		return astNodeRef{}, e
	}
	real := false
	r := m.external(m.arithmeticPlugin.fields[0])
	if m.coercions && arithmeticUsesCoercion(k) {
		for _, sort := range domain {
			if sort == r {
				real = true
				break
			}
		}
	} else if len(domain) != 0 {
		real = domain[0] == r
	}
	return m.arithmeticOperatorDecl(k, len(domain), real), nil
}
func (m *ASTManager) arithmeticExprDecl(k int32, ps []ASTParameter, args []ASTHandle) (astNodeRef, error) {
	if k == 0 {
		return m.arithmeticNumeralDecl(ps, len(args))
	}
	if e := arithmeticCheckArguments(k, len(args)); e != nil {
		return astNodeRef{}, e
	}
	limit := len(args)
	if !(m.coercions && arithmeticUsesCoercion(k)) && limit > 1 {
		limit = 1
	}
	real := false
	for _, h := range args[:limit] {
		r, e := m.requireExpr(h)
		if e != nil {
			return astNodeRef{}, e
		}
		s, e := m.sortOf(r)
		if e != nil {
			return astNodeRef{}, e
		}
		if s == m.arithmeticPlugin.fields[0] {
			real = true
			break
		}
	}
	return m.arithmeticOperatorDecl(k, len(args), real), nil
}
