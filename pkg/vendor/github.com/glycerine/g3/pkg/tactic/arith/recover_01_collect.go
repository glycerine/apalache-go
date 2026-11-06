package arith

// Source: src/tactic/arith/recover_01_tactic.cpp, imp::save_clause.

import "github.com/glycerine/g3/pkg/ast"

// recover01Clauses groups eligible clauses by the numerical variable declaration.
// A group's first accepted clause fixes its arity. Clause order and borrowed
// handles are preserved; no substitutions or model conversion happen here.
func recover01Clauses(m *ast.ASTManager, roots []ast.ASTHandle, maxBits uint32) (map[ast.ASTHandle][]ast.ASTHandle, error) {
	groups := make(map[ast.ASTHandle][]ast.ASTHandle)
	for _, root := range roots {
		or, e := arithmeticBasicApp(m, root, ast.ASTOpOr)
		if e != nil {
			return nil, e
		}
		if !or {
			continue
		}
		n, e := m.GetNumArgs(root)
		if e != nil {
			return nil, e
		}
		if n <= 1 || n >= maxBits {
			continue
		}
		var variable ast.ASTHandle
		found, valid := false, true
		for i := uint32(0); i < n; i++ {
			lit, e := m.GetArg(root, i)
			if e != nil {
				return nil, e
			}
			constant, e := arithmeticUninterpretedConstant(m, lit)
			if e != nil {
				return nil, e
			}
			if constant {
				continue
			}
			not, arg, e := m.IsNot(lit)
			if e != nil {
				return nil, e
			}
			if not {
				constant, e = arithmeticUninterpretedConstant(m, arg)
				if e != nil {
					return nil, e
				}
				if constant {
					continue
				}
			}
			eq, e := arithmeticBasicApp(m, lit, ast.ASTOpEq)
			if e != nil {
				return nil, e
			}
			if found || !eq {
				valid = false
				break
			}
			left, e := m.GetArg(lit, 0)
			if e != nil {
				return nil, e
			}
			right, e := m.GetArg(lit, 1)
			if e != nil {
				return nil, e
			}
			for side := 0; side < 2; side++ {
				constant, e = arithmeticUninterpretedConstant(m, left)
				if e != nil {
					return nil, e
				}
				if constant {
					_, _, numeral, e := m.ArithmeticNumeralText(right)
					if e != nil {
						return nil, e
					}
					if numeral {
						variable, e = m.GetDecl(left)
						if e != nil {
							return nil, e
						}
						found = true
						break
					}
				}
				left, right = right, left
			}
			if !found {
				valid = false
				break
			}
		}
		if !valid || !found {
			continue
		}
		clauses := groups[variable]
		if len(clauses) > 0 {
			arity, e := m.GetNumArgs(clauses[len(clauses)-1])
			if e != nil {
				return nil, e
			}
			if arity != n {
				continue
			}
		}
		groups[variable] = append(clauses, root)
	}
	return groups, nil
}
