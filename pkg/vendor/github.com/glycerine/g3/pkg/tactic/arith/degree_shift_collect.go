package arith

// Source: src/tactic/arith/degree_shift_tactic.cpp, candidate collection.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// degreeShiftCandidates borrows roots and returns the common positive exponent
// of each eligible real constant. Rewriting and model reconstruction are separate
// phases; this collector does not modify or own any expressions.
func degreeShiftCandidates(m *ast.ASTManager, roots []ast.ASTHandle) (map[ast.ASTHandle]*big.Int, error) {
	return degreeShiftCandidatesWithCheckpoint(m, roots, nil)
}

func degreeShiftCandidatesWithCheckpoint(m *ast.ASTManager, roots []ast.ASTHandle, checkpoint func() error) (map[ast.ASTHandle]*big.Int, error) {
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	degrees := make(map[ast.ASTHandle]*big.Int)
	visited := make(map[ast.ASTHandle]bool)
	var todo []ast.ASTHandle
	visit := func(h ast.ASTHandle) {
		if !visited[h] {
			visited[h] = true
			todo = append(todo, h)
		}
	}
	save := func(h ast.ASTHandle, degree *big.Int) error {
		kind, e := m.GetKind(h)
		if e != nil || kind != ast.ASTApp {
			return e
		}
		integer, found, e := arithmeticConstant(m, h, family)
		if e != nil || !found || integer {
			return e
		}
		if old := degrees[h]; old != nil {
			old.GCD(nil, nil, old, degree)
		} else {
			degrees[h] = new(big.Int).Set(degree)
		}
		return nil
	}
	one := big.NewInt(1)
	args := func(h ast.ASTHandle) error {
		kind, e := m.GetKind(h)
		if e != nil || kind != ast.ASTApp {
			return e
		}
		n, e := m.GetNumArgs(h)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			child, e := m.GetArg(h, i)
			if e != nil {
				return e
			}
			if e = save(child, one); e != nil {
				return e
			}
			visit(child)
		}
		return nil
	}
	for _, root := range roots {
		visit(root)
		for len(todo) != 0 {
			if checkpoint != nil {
				if err := checkpoint(); err != nil {
					return nil, err
				}
			}
			h := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			kind, e := m.GetKind(h)
			if e != nil {
				return nil, e
			}
			switch kind {
			case ast.ASTVar:
				continue
			case ast.ASTQuantifier:
				body, e := m.GetQuantifierBody(h)
				if e != nil {
					return nil, e
				}
				visit(body)
				n, e := m.GetNumPatterns(h)
				if e != nil {
					return nil, e
				}
				for i := uint32(0); i < n; i++ {
					p, e := m.GetPattern(h, i)
					if e != nil {
						return nil, e
					}
					visit(p)
				}
				n, e = m.GetNumNoPatterns(h)
				if e != nil {
					return nil, e
				}
				for i := uint32(0); i < n; i++ {
					p, e := m.GetNoPattern(h, i)
					if e != nil {
						return nil, e
					}
					visit(p)
				}
			case ast.ASTApp:
				fid, e := m.GetFamilyID(h)
				if e != nil {
					return nil, e
				}
				op, e := m.GetDeclKind(h)
				if e != nil {
					return nil, e
				}
				if fid == family && op == 18 {
					exponent, e := m.GetArg(h, 1)
					if e != nil {
						return nil, e
					}
					text, _, found, e := m.ArithmeticNumeralText(exponent)
					if e != nil {
						return nil, e
					}
					k, ok := new(big.Rat).SetString(text)
					if found && ok && k.IsInt() && k.Sign() > 0 {
						base, e := m.GetArg(h, 0)
						if e != nil {
							return nil, e
						}
						if e = save(base, k.Num()); e != nil {
							return nil, e
						}
						if e = args(base); e != nil {
							return nil, e
						}
						continue
					}
				}
				if e = args(h); e != nil {
					return nil, e
				}
			default:
				return nil, ast.ErrASTDomain
			}
		}
	}
	for h, degree := range degrees {
		if degree.Cmp(one) == 0 {
			delete(degrees, h)
		}
	}
	return degrees, nil
}
