package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, collect_fd/is_fd/is_bound.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// eq2bvCandidates returns observed equality maxima, before cleanup_fd adjusts
// domain sizes from bounds and creates bit-vector constants. Handles are borrowed.
func eq2bvCandidates(m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]uint32, error) {
	return eq2bvCollect(m, roots, make(map[ast.ASTHandle]bool))
}

func eq2bvCollect(m *ast.ASTManager, roots []ast.ASTHandle, nonfd map[ast.ASTHandle]bool) (map[ast.ASTHandle]uint32, error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	maxima := make(map[ast.ASTHandle]uint32)
	pair := func(v, c ast.ASTHandle, requireInt bool) (uint32, bool, error) {
		constant, e := arithmeticUninterpretedConstant(m, v)
		if e != nil || !constant || nonfd[v] {
			return 0, false, e
		}
		if requireInt {
			integer, found, e := arithmeticConstant(m, v, family)
			if e != nil || !found || !integer {
				return 0, false, e
			}
		}
		text, _, found, e := m.ArithmeticNumeralText(c)
		if e != nil || !found {
			return 0, false, e
		}
		r, ok := new(big.Rat).SetString(text)
		if !ok || !r.IsInt() || r.Sign() < 0 || r.Num().BitLen() > 32 {
			return 0, false, nil
		}
		return uint32(r.Num().Uint64()), true, nil
	}
	for _, root := range roots {
		kind, e := m.GetKind(root)
		if e != nil {
			return nil, e
		}
		if kind == ast.ASTApp {
			fid, e := m.GetFamilyID(root)
			if e != nil {
				return nil, e
			}
			op, e := m.GetDeclKind(root)
			if e != nil {
				return nil, e
			}
			if fid == family && (op == 2 || op == 3) {
				left, e := m.GetArg(root, 0)
				if e != nil {
					return nil, e
				}
				right, e := m.GetArg(root, 1)
				if e != nil {
					return nil, e
				}
				_, lower, e := pair(left, right, false)
				if e != nil {
					return nil, e
				}
				_, upper, e := pair(right, left, false)
				if e != nil {
					return nil, e
				}
				if lower || upper {
					continue
				}
			}
		}
		todo := []ast.ASTHandle{root}
		for len(todo) > 0 {
			h := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if nonfd[h] {
				continue
			}
			nonfd[h] = true
			kind, e := m.GetKind(h)
			if e != nil {
				return nil, e
			}
			if kind == ast.ASTQuantifier {
				body, e := m.GetQuantifierBody(h)
				if e != nil {
					return nil, e
				}
				todo = append(todo, body)
			} else if kind == ast.ASTApp {
				eq, e := arithmeticBasicApp(m, h, ast.ASTOpEq)
				if e != nil {
					return nil, e
				}
				if eq {
					left, e := m.GetArg(h, 0)
					if e != nil {
						return nil, e
					}
					right, e := m.GetArg(h, 1)
					if e != nil {
						return nil, e
					}
					value, found, e := pair(left, right, true)
					if e != nil {
						return nil, e
					}
					if !found {
						left, right = right, left
						value, found, e = pair(left, right, true)
						if e != nil {
							return nil, e
						}
					}
					if found {
						old, exists := maxima[left]
						if !exists || old < value {
							maxima[left] = value
						}
						continue
					}
				}
				n, e := m.GetNumArgs(h)
				if e != nil {
					return nil, e
				}
				for i := uint32(0); i < n; i++ {
					child, e := m.GetArg(h, i)
					if e != nil {
						return nil, e
					}
					todo = append(todo, child)
				}
			} else if kind != ast.ASTVar {
				return nil, ast.ErrASTDomain
			}
		}
	}
	for h := range maxima {
		if nonfd[h] {
			delete(maxima, h)
		}
	}
	return maxima, nil
}

// These concrete expression predicates are shared by the arithmetic collectors.
func arithmeticUninterpretedConstant(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	kind, e := m.GetKind(h)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != ast.NullFamilyID {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	return n == 0, e
}

func arithmeticBasicApp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	kind, e := m.GetKind(h)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != m.BasicFamilyID() {
		return false, e
	}
	k, e := m.GetDeclKind(h)
	return k == op, e
}
